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
	mu          sync.RWMutex
	id          string
	addr        string            // endereço TCP (ex: "127.0.0.1:7000")
	peers       map[string]string // id -> endereço
	isLeader    bool
	leaderId    string
	leaderAddr  string
	state       *EstadoLedger
	heartbeatCh chan bool
	stopCh      chan struct{}
}

type RaftConfigTCP struct {
	NodeID    string
	RaftAddr  string
	Peers     map[string]string // id -> endereço dos outros nós
	Bootstrap bool              // se true, começa como líder
}

func NewTCPRaft(cfg RaftConfigTCP) (*TCPRaft, error) {
	r := &TCPRaft{
		id:          cfg.NodeID,
		addr:        cfg.RaftAddr,
		peers:       cfg.Peers,
		isLeader:    cfg.Bootstrap,
		state:       NovoEstadoLedger(),
		heartbeatCh: make(chan bool, 100),
		stopCh:      make(chan struct{}),
	}
	if !cfg.Bootstrap {
		go func() {
			time.Sleep(1 * time.Second) // Aguarda servidor TCP iniciar
			r.tryJoin()
			r.campaign()
		}()
	} else {
		go r.sendHeartbeats()
	}
	// Inicia servidor TCP
	go r.startTCPServer()
	// Se não for bootstrap, inicia eleição (após alguns segundos)
	if !cfg.Bootstrap {
		go r.campaign()
	} else {
		go r.sendHeartbeats()
	}
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
		msg := fmt.Sprintf(`{"type":"join","nodeId":"%s","nodeAddr":"%s"}`, r.id, r.addr)
		fmt.Fprintf(conn, "%s\n", msg)
		// Aguarda resposta "ok"
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
	time.Sleep(2 * time.Second) // aguarda estabilização
	for {
		if r.isLeader {
			time.Sleep(1 * time.Second)
			continue
		}
		// Se não recebe heartbeat do líder há mais de 2 segundos
		// Inicia eleição
		r.mu.Lock()
		lastLeader := r.leaderId
		r.mu.Unlock()
		if lastLeader != "" {
			time.Sleep(1 * time.Second)
			continue
		}
		log.Printf("[RAFT %s] iniciando eleição", r.id)
		votes := 1
		for id, addr := range r.peers {
			if id == r.id {
				continue
			}
			if r.requestVote(addr) {
				votes++
			}
		}
		if votes > len(r.peers)/2 {
			r.mu.Lock()
			r.isLeader = true
			r.leaderId = r.id
			r.leaderAddr = r.addr
			r.mu.Unlock()
			log.Printf("[RAFT %s] tornou-se LÍDER", r.id)
			go r.sendHeartbeats()
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func (r *TCPRaft) requestVote(targetAddr string) bool {
	conn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		return false
	}
	defer conn.Close()
	req := map[string]interface{}{
		"type":      "vote",
		"candidate": r.id,
		"term":      1,
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

	candidate := cmd["candidate"].(string)
	// Sempre vota sim (simplificado)
	log.Printf("[RAFT %s] recebido pedido de voto de %s", r.id, candidate)
	resp := map[string]interface{}{"granted": true}
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
	conn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		return
	}
	defer conn.Close()
	req := map[string]interface{}{
		"type":       "heartbeat",
		"leader":     r.id,
		"leaderAddr": r.addr,
	}
	data, _ := json.Marshal(req)
	fmt.Fprintf(conn, "%s\n", data)
}

func (r *TCPRaft) handleHeartbeat(cmd map[string]interface{}) {
	leader := cmd["leader"].(string)
	leaderAddr := cmd["leaderAddr"].(string)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isLeader && leader != r.id {
		r.isLeader = false
	}
	r.leaderId = leader
	r.leaderAddr = leaderAddr
}

// ------------------ Replicação de comandos ------------------
func (r *TCPRaft) AplicarTransacao(transacao *Transacao) error {
	if !r.isLeader {
		return fmt.Errorf("não sou líder, líder é %s", r.leaderId)
	}
	// Aplica localmente
	if err := r.state.AplicarTransacao(transacao); err != nil {
		return err
	}
	log.Printf("[RAFT LÍDER %s] 📝 Transação %s aplicada localmente", r.id, transacao.ID)

	// Prepara comando para replicação
	data, _ := json.Marshal(transacao)
	cmd := map[string]interface{}{
		"type": "append",
		"data": string(data),
	}
	cmdJSON, _ := json.Marshal(cmd)
	cmdStr := string(cmdJSON)

	replicados := 0
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
		replicados++
	}
	log.Printf("[RAFT LÍDER %s] 📡 Transação %s enviada para %d seguidores", r.id, transacao.ID, replicados)
	return nil
}

func (r *TCPRaft) sendAppend(targetAddr, cmd string) error {
	conn, err := net.Dial("tcp", targetAddr)
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
		log.Printf("[RAFT SEGUIDOR %s] Campo 'data' inválido ou ausente", r.id)
		return
	}
	var transacao Transacao
	if err := json.Unmarshal([]byte(dataStr), &transacao); err != nil {
		log.Printf("[RAFT SEGUIDOR %s] Erro ao decodificar transação replicada: %v", r.id, err)
		return
	}
	if err := r.state.AplicarTransacao(&transacao); err != nil {
		log.Printf("[RAFT SEGUIDOR %s] Erro ao aplicar transação replicada %s: %v", r.id, transacao.ID, err)
		return
	}
	log.Printf("[RAFT SEGUIDOR %s] ✅ Transação replicada aplicada: %s", r.id, transacao.ID)
}

// ------------------ Join ------------------
func (r *TCPRaft) handleJoin(cmd map[string]interface{}, conn net.Conn) {
	nodeId := cmd["nodeId"].(string)
	nodeAddr := cmd["nodeAddr"].(string)
	r.mu.Lock()
	r.peers[nodeId] = nodeAddr
	// Envia o estado completo (snapshot) para o novo nó
	go r.sendSnapshot(nodeAddr)
	r.mu.Unlock()
	fmt.Fprintf(conn, "ok\n")
}

func (r *TCPRaft) handleSnapshot(cmd map[string]interface{}) {
	dataStr, ok := cmd["data"].(string)
	if !ok {
		return
	}
	var historico []Transacao
	if err := json.Unmarshal([]byte(dataStr), &historico); err != nil {
		log.Printf("[RAFT SEGUIDOR %s] Erro ao restaurar snapshot: %v", r.id, err)
		return
	}
	// Cria um novo estado e aplica todas as transações na ordem
	novoEstado := NovoEstadoLedger()
	for _, tx := range historico {
		if err := novoEstado.AplicarTransacao(&tx); err != nil {
			log.Printf("[RAFT SEGUIDOR %s] Erro ao aplicar transação do snapshot: %v", r.id, err)
			return
		}
	}
	// Substitui o estado atual
	r.mu.Lock()
	r.state = novoEstado
	r.mu.Unlock()
	log.Printf("[RAFT SEGUIDOR %s] ✅ Snapshot restaurado com %d transações", r.id, len(historico))
}

// sendSnapshot envia todas as transações do histórico para o peer
func (r *TCPRaft) sendSnapshot(peerAddr string) {
	r.mu.RLock()
	historico := r.state.ObterHistorico()
	r.mu.RUnlock()

	// Serializa todas as transações
	data, err := json.Marshal(historico)
	if err != nil {
		log.Printf("[RAFT LÍDER %s] Erro ao serializar snapshot: %v", r.id, err)
		return
	}

	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		log.Printf("[RAFT LÍDER %s] Falha ao enviar snapshot para %s: %v", r.id, peerAddr, err)
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

func (r *TCPRaft) Close() {
	close(r.stopCh)
}
