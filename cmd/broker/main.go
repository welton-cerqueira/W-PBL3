package main

import (
	"flag"
	"fmt"
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
		id        = flag.String("id", "broker1", "ID do broker")
		portaAPI  = flag.String("porta", "8080", "Porta HTTP")
		ehLider   = flag.Bool("lider", false, "Inicia como líder (bootstrap)")
		outrosNos = flag.String("outros", "", "IDs dos outros brokers (ex: broker2,broker3)")
	)
	flag.Parse()

	peers := make(map[string]string)
	peers[*id] = getRaftAddr(*id)

	if *outrosNos != "" {
		for _, outroID := range strings.Split(*outrosNos, ",") {
			outroID = strings.TrimSpace(outroID)
			if outroID == "" {
				continue
			}
			peers[outroID] = getRaftAddr(outroID)
		}
	}

	cfg := consenso.RaftConfigTCP{
		NodeID:    *id,
		RaftAddr:  peers[*id],
		Peers:     peers,
		Bootstrap: *ehLider,
	}

	raftNode, err := consenso.NewTCPRaft(cfg)
	if err != nil {
		log.Fatalf("[ERRO] Falha ao criar nó Raft: %v", err)
	}
	defer raftNode.Close()

	droneManager := drone.NovoGerenciadorDrones()
	servidor := api.NovoServidorAPI(*portaAPI, raftNode, droneManager) // assinatura ajustada
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

	log.Printf("[SUCESSO] Broker %s rodando. API: http://localhost:%s, Raft: %s", *id, *portaAPI, peers[*id])

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[FIM] Encerrando broker...")
}

func getRaftAddr(id string) string {
	var num int
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] >= '0' && id[i] <= '9' {
			num = num*10 + int(id[i]-'0')
		} else {
			break
		}
	}
	if num == 0 {
		num = 1
	}
	return fmt.Sprintf("127.0.0.1:%d", 7000+(num-1))
}

func aplicarCreditosIniciais(raftNode *consenso.TCPRaft) {
	transacaoA, _ := consenso.NovaTransacao(consenso.TipoRecarga, consenso.DadosRecarga{
		CompanhiaID:   "COMP-A",
		Valor:         100,
		AutorizadoPor: "sistema",
	})
	if err := raftNode.AplicarTransacao(transacaoA); err != nil {
		log.Printf("[AVISO] Erro ao recarregar COMP-A: %v", err)
	}

	transacaoB, _ := consenso.NovaTransacao(consenso.TipoRecarga, consenso.DadosRecarga{
		CompanhiaID:   "COMP-B",
		Valor:         50,
		AutorizadoPor: "sistema",
	})
	if err := raftNode.AplicarTransacao(transacaoB); err != nil {
		log.Printf("[AVISO] Erro ao recarregar COMP-B: %v", err)
	}
	log.Println("[INICIAL] Créditos iniciais aplicados via Raft")
}
