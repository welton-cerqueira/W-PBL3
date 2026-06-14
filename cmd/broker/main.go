package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"W-PBL3/internal/api"
	"W-PBL3/internal/consenso"
	"W-PBL3/internal/drone"
)

func main() {
	var (
		id       = flag.String("id", "broker1", "ID do broker")
		apiAddr  = flag.String("api-addr", "", "Endereço público da API (ex: 192.168.1.10:8080)")
		raftAddr = flag.String("raft-addr", "", "Endereço Raft (ex: 192.168.1.10:7000)")
		ehLider  = flag.Bool("lider", false, "Inicia como líder (bootstrap)")
		outros   = flag.String("outros", "", "Outros brokers no formato id=ip:porta,id=ip:porta")
	)
	flag.Parse()

	if *apiAddr == "" || *raftAddr == "" {
		log.Fatal("--api-addr e --raft-addr são obrigatórios")
	}

	peers := make(map[string]string)
	peers[*id] = *raftAddr

	if *outros != "" {
		for _, par := range strings.Split(*outros, ",") {
			partes := strings.SplitN(par, "=", 2)
			if len(partes) != 2 {
				log.Fatalf("Formato inválido em -outros: %s (esperado id=endereco)", par)
			}
			peerID := strings.TrimSpace(partes[0])
			peerRaftAddr := strings.TrimSpace(partes[1])
			peers[peerID] = peerRaftAddr
		}
	}

	cfg := consenso.RaftConfigTCP{
		NodeID:    *id,
		RaftAddr:  *raftAddr,
		ApiAddr:   *apiAddr,
		Peers:     peers,
		Bootstrap: *ehLider,
	}

	raftNode, err := consenso.NewTCPRaft(cfg)
	if err != nil {
		log.Fatalf("[ERRO] Falha ao criar nó Raft: %v", err)
	}
	defer raftNode.Close()

	droneManager := drone.NovoGerenciadorDrones()
	servidor := api.NovoServidorAPI(*apiAddr, raftNode, droneManager)
	servidor.RegistrarRotas()

	if *ehLider {
		time.Sleep(5 * time.Second)
		aplicarCreditosIniciais(raftNode)
	}

	go func() {
		if err := servidor.Iniciar(); err != nil {
			log.Fatalf("[ERRO] Falha ao iniciar servidor: %v", err)
		}
	}()

	log.Printf("[SUCESSO] Broker %s rodando. API: http://%s, Raft: %s", *id, *apiAddr, *raftAddr)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[FIM] Encerrando broker...")
}

func aplicarCreditosIniciais(raftNode *consenso.TCPRaft) {
	// Garante que o nó é líder antes de tentar aplicar transações
	for !raftNode.EhLider() {
		log.Println("[INICIAL] Aguardando tornar-se líder...")
		time.Sleep(1 * time.Second)
	}

	// Aguarda a presença de pelo menos um peer (além de si mesmo) no cluster
	log.Println("[INICIAL] Aguardando conexão com seguidores (até 10s)...")
	time.Sleep(5 * time.Second)

	maxTentativas := 5
	for tentativa := 1; tentativa <= maxTentativas; tentativa++ {
		transacaoA, err := consenso.NovaTransacao(consenso.TipoRecarga, consenso.DadosRecarga{
			CompanhiaID:   "COMP-A",
			Valor:         100,
			AutorizadoPor: "sistema",
		})
		if err != nil {
			log.Printf("[INICIAL] Erro ao criar transação A: %v", err)
			continue
		}
		// Ignora o novo saldo retornado
		if _, err := raftNode.AplicarTransacao(transacaoA); err != nil {
			log.Printf("[INICIAL] Tentativa %d/%d: falha ao recarregar COMP-A: %v", tentativa, maxTentativas, err)
			time.Sleep(2 * time.Second)
			continue
		}
		log.Println("[INICIAL] ✅ COMP-A recebeu 100 créditos")
		break
	}

	for tentativa := 1; tentativa <= maxTentativas; tentativa++ {
		transacaoB, err := consenso.NovaTransacao(consenso.TipoRecarga, consenso.DadosRecarga{
			CompanhiaID:   "COMP-B",
			Valor:         50,
			AutorizadoPor: "sistema",
		})
		if err != nil {
			log.Printf("[INICIAL] Erro ao criar transação B: %v", err)
			continue
		}
		if _, err := raftNode.AplicarTransacao(transacaoB); err != nil {
			log.Printf("[INICIAL] Tentativa %d/%d: falha ao recarregar COMP-B: %v", tentativa, maxTentativas, err)
			time.Sleep(2 * time.Second)
			continue
		}
		log.Println("[INICIAL] ✅ COMP-B recebeu 50 créditos")
		break
	}
}
