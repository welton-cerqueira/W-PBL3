package consenso

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type TCPRaft struct {
	mu             sync.RWMutex
	id             string
	addr           string            // endereço TCP Raft (ex: "192.168.1.10:7000")
	apiAddr        string            // endereço HTTP da API (ex: "192.168.1.10:8080")
	peers          map[string]string // id -> endereço Raft
	isLeader       bool
	leaderId       string
	leaderRaftAddr string // endereço Raft do líder
	leaderApiAddr  string // endereço API do líder
	state          *EstadoLedger
	heartbeatCh    chan bool
	stopCh         chan struct{}
	lastHeartbeat  time.Time // timestamp do último heartbeat recebido (seguidor)
	currentTerm    int       // número da época (termo) atual
	votedFor       string    // ID do nó que recebeu o voto neste termo
}

type RaftConfigTCP struct {
	NodeID    string
	RaftAddr  string
	ApiAddr   string
	Peers     map[string]string
	Bootstrap bool
}

func NewTCPRaft(cfg RaftConfigTCP) (*TCPRaft, error) {
	r := &TCPRaft{
		id:            cfg.NodeID,
		addr:          cfg.RaftAddr,
		apiAddr:       cfg.ApiAddr,
		peers:         cfg.Peers,
		isLeader:      cfg.Bootstrap,
		state:         NovoEstadoLedger(),
		heartbeatCh:   make(chan bool, 100),
		stopCh:        make(chan struct{}),
		lastHeartbeat: time.Now(),
	}

	if cfg.Bootstrap {
		r.leaderId = cfg.NodeID
		r.leaderRaftAddr = cfg.RaftAddr
		r.leaderApiAddr = cfg.ApiAddr
		r.currentTerm = 0
		r.votedFor = ""
		go r.sendHeartbeats()
	} else {
		go func() {
			time.Sleep(1 * time.Second)
			r.tryJoin()
			r.campaign()
		}()
	}

	go r.startTCPServer()
	return r, nil
}

// tryJoin tenta se juntar ao cluster enviando um comando "join" para qualquer peer conhecido
func (r *TCPRaft) tryJoin() {
	for id, addr := range r.peers {
		if id == r.id {
			continue
		}
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Printf("[RAFT %s] Falha ao conectar para join em %s: %v", r.id, addr, err)
			continue
		}
		msg := fmt.Sprintf(`{"type":"join","nodeId":"%s","nodeAddr":"%s","apiAddr":"%s"}`, r.id, r.addr, r.apiAddr)
		fmt.Fprintf(conn, "%s\n", msg)
		buf := make([]byte, 2)
		n, _ := conn.Read(buf)
		conn.Close()
		if n >= 2 && string(buf[:2]) == "ok" {
			log.Printf("[RAFT %s] ✅ Juntou-se ao cluster via %s (%s)", r.id, id, addr)
			return
		}
	}
	log.Printf("[RAFT %s] ⚠️ Não conseguiu se juntar a nenhum peer. Tentará novamente em 5s.", r.id)
	time.AfterFunc(5*time.Second, r.tryJoin)
}

func (r *TCPRaft) startTCPServer() {
	ln, err := net.Listen("tcp", r.addr)
	if err != nil {
		log.Fatalf("[RAFT %s] erro ao ouvir: %v", r.id, err)
	}
	defer ln.Close()
	for {
		select {
		case <-r.stopCh:
			return
		default:
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go r.handleConnection(conn)
		}
	}
}

func (r *TCPRaft) handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		var cmd map[string]interface{}
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			continue
		}
		typ := cmd["type"].(string)
		switch typ {
		case "heartbeat":
			r.handleHeartbeat(cmd)
		case "vote":
			r.handleVote(cmd, conn)
		case "append":
			r.handleAppend(cmd)
		case "join":
			r.handleJoin(cmd, conn)
		case "snapshot":
			r.handleSnapshot(cmd)
		}
	}
}

// ------------------ Eleição ------------------
func (r *TCPRaft) campaign() {
	time.Sleep(2 * time.Second)
	for {
		if r.isLeader {
			time.Sleep(1 * time.Second)
			continue
		}

		r.mu.Lock()
		lastHb := r.lastHeartbeat
		leaderExists := (r.leaderId != "")
		r.mu.Unlock()

		if leaderExists && time.Since(lastHb) > 2*time.Second {
			r.mu.Lock()
			log.Printf("[RAFT %s] Líder %s falhou (timeout), iniciando eleição", r.id, r.leaderId)
			r.leaderId = ""
			r.leaderRaftAddr = ""
			r.leaderApiAddr = ""
			r.mu.Unlock()
			leaderExists = false
		}

		if leaderExists {
			time.Sleep(1 * time.Second)
			continue
		}

		// Inicia uma nova eleição: incrementa termo e vota em si mesmo
		r.mu.Lock()
		r.currentTerm++
		r.votedFor = r.id
		meuTermo := r.currentTerm
		r.mu.Unlock()

		log.Printf("[RAFT %s] iniciando eleição para termo %d", r.id, meuTermo)
		votes := 1 // voto próprio

		for id, addr := range r.peers {
			if id == r.id {
				continue
			}
			if r.requestVote(addr, meuTermo) {
				votes++
			}
		}

		if votes > len(r.peers)/2 {
			r.mu.Lock()
			r.isLeader = true
			r.leaderId = r.id
			r.leaderRaftAddr = r.addr
			r.leaderApiAddr = r.apiAddr
			r.lastHeartbeat = time.Now()
			r.mu.Unlock()
			log.Printf("[RAFT %s] tornou-se LÍDER para termo %d", r.id, meuTermo)
			go r.sendHeartbeats()
			return
		} else {
			log.Printf("[RAFT %s] não obteve maioria para termo %d. Aguardando...", r.id, meuTermo)
			time.Sleep(3 * time.Second)
		}
	}
}

func (r *TCPRaft) requestVote(targetAddr string, termo int) bool {
	conn, err := net.DialTimeout("tcp", targetAddr, 1*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(1 * time.Second))

	req := map[string]interface{}{
		"type":      "vote",
		"candidate": r.id,
		"term":      termo,
	}
	data, _ := json.Marshal(req)
	fmt.Fprintf(conn, "%s\n", data)

	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		resp := scanner.Text()
		var res map[string]interface{}
		json.Unmarshal([]byte(resp), &res)
		if granted, ok := res["granted"].(bool); ok && granted {
			return true
		}
	}
	return false
}

func (r *TCPRaft) handleVote(cmd map[string]interface{}, conn net.Conn) {
	candidate, _ := cmd["candidate"].(string)
	termo, _ := cmd["term"].(float64)
	termoInt := int(termo)

	r.mu.Lock()
	defer r.mu.Unlock()

	granted := false
	if termoInt > r.currentTerm {
		r.currentTerm = termoInt
		r.votedFor = ""
		r.isLeader = false
		r.leaderId = ""
	}
	if (r.votedFor == "" || r.votedFor == candidate) && termoInt >= r.currentTerm {
		r.votedFor = candidate
		granted = true
		log.Printf("[RAFT %s] concedeu voto para %s no termo %d", r.id, candidate, termoInt)
	} else {
		log.Printf("[RAFT %s] recusou voto para %s (termo atual %d)", r.id, candidate, r.currentTerm)
	}

	resp := map[string]interface{}{"granted": granted}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(conn, "%s\n", data)
}

// ------------------ Heartbeat ------------------
func (r *TCPRaft) sendHeartbeats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			if !r.isLeader {
				return
			}
			r.mu.RLock()
			peers := make(map[string]string)
			for k, v := range r.peers {
				peers[k] = v
			}
			r.mu.RUnlock()
			for id, addr := range peers {
				if id == r.id {
					continue
				}
				go r.sendHeartbeat(addr)
			}
		}
	}
}

func (r *TCPRaft) sendHeartbeat(targetAddr string) {
	conn, err := net.DialTimeout("tcp", targetAddr, 500*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()

	r.mu.RLock()
	termo := r.currentTerm
	r.mu.RUnlock()

	req := map[string]interface{}{
		"type":           "heartbeat",
		"leader":         r.id,
		"leaderRaftAddr": r.addr,
		"leaderApiAddr":  r.apiAddr,
		"term":           termo,
	}
	data, _ := json.Marshal(req)
	fmt.Fprintf(conn, "%s\n", data)
}

func (r *TCPRaft) handleHeartbeat(cmd map[string]interface{}) {
	leaderId, _ := cmd["leader"].(string)
	leaderRaftAddr, _ := cmd["leaderRaftAddr"].(string)
	leaderApiAddr, _ := cmd["leaderApiAddr"].(string)
	termo, _ := cmd["term"].(float64) // NOVO

	r.mu.Lock()
	defer r.mu.Unlock()
	if termoInt := int(termo); termoInt > r.currentTerm {
		r.currentTerm = termoInt
		r.votedFor = ""
	}
	if r.isLeader && leaderId != r.id {
		r.isLeader = false
	}
	r.leaderId = leaderId
	r.leaderRaftAddr = leaderRaftAddr
	r.leaderApiAddr = leaderApiAddr
	r.lastHeartbeat = time.Now()
}

// ------------------ Replicação de comandos ------------------
func (r *TCPRaft) AplicarTransacao(transacao *Transacao) error {
	if !r.isLeader {
		return fmt.Errorf("não sou líder, líder é %s (%s)", r.leaderId, r.leaderApiAddr)
	}
	if err := r.state.AplicarTransacao(transacao); err != nil {
		return err
	}
	log.Printf("[RAFT LÍDER %s] 📝 Transação %s aplicada localmente", r.id, transacao.ID)

	data, _ := json.Marshal(transacao)
	cmd := map[string]interface{}{
		"type": "append",
		"data": string(data),
	}
	cmdJSON, _ := json.Marshal(cmd)
	cmdStr := string(cmdJSON)

	for id, addr := range r.peers {
		if id == r.id {
			continue
		}
		go func(peerId, peerAddr string) {
			if err := r.sendAppend(peerAddr, cmdStr); err != nil {
				log.Printf("[RAFT LÍDER %s] ❌ Falha ao replicar para %s (%s): %v", r.id, peerId, peerAddr, err)
			} else {
				log.Printf("[RAFT LÍDER %s] ✅ Transação %s replicada para %s (%s)", r.id, transacao.ID, peerId, peerAddr)
			}
		}(id, addr)
	}
	log.Printf("[RAFT LÍDER %s] 📡 Transação %s enviada para %d seguidores", r.id, transacao.ID, len(r.peers)-1)
	return nil
}

func (r *TCPRaft) sendAppend(targetAddr, cmd string) error {
	conn, err := net.DialTimeout("tcp", targetAddr, 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = fmt.Fprintf(conn, "%s\n", cmd)
	return err
}

func (r *TCPRaft) handleAppend(cmd map[string]interface{}) {
	dataStr, ok := cmd["data"].(string)
	if !ok {
		log.Printf("[RAFT SEGUIDOR %s] Campo 'data' inválido", r.id)
		return
	}
	var transacao Transacao
	if err := json.Unmarshal([]byte(dataStr), &transacao); err != nil {
		log.Printf("[RAFT SEGUIDOR %s] Erro ao decodificar: %v", r.id, err)
		return
	}
	if err := r.state.AplicarTransacao(&transacao); err != nil {
		log.Printf("[RAFT SEGUIDOR %s] Erro ao aplicar: %v", r.id, err)
		return
	}
	log.Printf("[RAFT SEGUIDOR %s] ✅ Transação replicada aplicada: %s", r.id, transacao.ID)
}

// ------------------ Join ------------------
func (r *TCPRaft) handleJoin(cmd map[string]interface{}, conn net.Conn) {
	nodeId, _ := cmd["nodeId"].(string)
	nodeRaftAddr, _ := cmd["nodeAddr"].(string)

	r.mu.Lock()
	r.peers[nodeId] = nodeRaftAddr
	go r.sendSnapshot(nodeRaftAddr)
	r.mu.Unlock()
	fmt.Fprintf(conn, "ok\n")
}

func (r *TCPRaft) handleSnapshot(cmd map[string]interface{}) {
	dataStr, _ := cmd["data"].(string)
	if dataStr == "" {
		return
	}
	var historico []Transacao
	if err := json.Unmarshal([]byte(dataStr), &historico); err != nil {
		log.Printf("[RAFT SEGUIDOR %s] Erro ao restaurar snapshot: %v", r.id, err)
		return
	}
	novoEstado := NovoEstadoLedger()
	for _, tx := range historico {
		if err := novoEstado.AplicarTransacao(&tx); err != nil {
			log.Printf("[RAFT SEGUIDOR %s] Erro ao aplicar tx do snapshot: %v", r.id, err)
			return
		}
	}
	r.mu.Lock()
	r.state = novoEstado
	r.mu.Unlock()
	log.Printf("[RAFT SEGUIDOR %s] ✅ Snapshot restaurado com %d transações", r.id, len(historico))
}

func (r *TCPRaft) sendSnapshot(peerAddr string) {
	historico := r.state.ObterHistorico()
	data, err := json.Marshal(historico)
	if err != nil {
		log.Printf("[RAFT LÍDER %s] Erro serializando snapshot: %v", r.id, err)
		return
	}
	conn, err := net.DialTimeout("tcp", peerAddr, 2*time.Second)
	if err != nil {
		log.Printf("[RAFT LÍDER %s] Falha enviar snapshot para %s: %v", r.id, peerAddr, err)
		return
	}
	defer conn.Close()
	msg := map[string]interface{}{
		"type": "snapshot",
		"data": string(data),
	}
	jsonMsg, _ := json.Marshal(msg)
	fmt.Fprintf(conn, "%s\n", jsonMsg)
}

func (r *TCPRaft) ObterEstado() *EstadoLedger {
	return r.state
}

func (r *TCPRaft) EhLider() bool {
	return r.isLeader
}

func (r *TCPRaft) ObterLiderID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.leaderId
}

func (r *TCPRaft) ObterLiderApiAddr() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.leaderApiAddr == "" && r.isLeader {
		return r.apiAddr
	}
	return r.leaderApiAddr
}

func (r *TCPRaft) Close() {
	close(r.stopCh)
}
