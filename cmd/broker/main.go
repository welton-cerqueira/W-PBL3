package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"W-PBL3/internal/api"
	"W-PBL3/internal/consenso"
	"W-PBL3/internal/drone"
)

func main() {
	// Parâmetros de linha de comando
	var (
		id        = flag.String("id", "broker1", "ID do broker (ex: broker1, broker2, broker3)")
		porta     = flag.String("porta", "8080", "Porta HTTP do broker")
		ehLider   = flag.Bool("lider", false, "Se true, este broker inicia como líder")
		outrosNos = flag.String("outros", "", "IDs dos outros brokers separados por vírgula (ex: broker2,broker3)")
	)
	flag.Parse()

	log.Printf("[INICIO] Iniciando broker %s na porta %s", *id, *porta)

	// 1. Inicializa o estado do ledger
	estado := consenso.NovoEstadoLedger()

	// 2. Configura nós Raft (simplificado)
	// Em uma implementação real, isso se comunicaria com outros brokers
	var listaOutros []string
	if *outrosNos != "" {
		// Aqui você implementaria a lógica para se conectar aos outros nós
		listaOutros = []string{} // Placeholder
	}

	raftNode := consenso.NovoNoRaft(*id, estado, listaOutros)

	// Define se este nó é líder ou seguidor
	if *ehLider {
		raftNode.TornarLider()
	} else {
		// Em produção, descobriria o líder automaticamente
		raftNode.TornarSeguidor("broker1")
	}

	// 3. Inicializa o gerenciador de drones
	droneManager := drone.NovoGerenciadorDrones()

	// 4. Inicializa o servidor API
	servidor := api.NovoServidorAPI(*porta, estado, raftNode, droneManager)
	servidor.RegistrarRotas()

	// 5. Adiciona créditos iniciais para as companhias (somente se for o líder)
	if *ehLider {
		adicionarCreditosIniciais(estado, raftNode)
	}

	// 6. Inicia o servidor em uma goroutine
	go func() {
		if err := servidor.Iniciar(); err != nil {
			log.Fatalf("[ERRO] Falha ao iniciar servidor: %v", err)
		}
	}()

	log.Printf("[SUCESSO] Broker %s rodando em http://localhost:%s", *id, *porta)

	// 7. Aguarda sinal de término
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[FIM] Encerrando broker...")
}

// adicionarCreditosIniciais adiciona créditos para as companhias de teste
func adicionarCreditosIniciais(estado *consenso.EstadoLedger, raftNode *consenso.NoRaft) {
	// Companhia A: 100 créditos
	dadosRecargaA := consenso.DadosRecarga{
		CompanhiaID:   "COMP-A",
		Valor:         100,
		AutorizadoPor: "sistema",
	}
	transacaoA, _ := consenso.NovaTransacao(consenso.TipoRecarga, dadosRecargaA)
	if err := estado.AplicarTransacao(transacaoA); err != nil {
		log.Printf("[AVISO] Erro ao recarregar COMP-A: %v", err)
	}

	// Companhia B: 50 créditos
	dadosRecargaB := consenso.DadosRecarga{
		CompanhiaID:   "COMP-B",
		Valor:         50,
		AutorizadoPor: "sistema",
	}
	transacaoB, _ := consenso.NovaTransacao(consenso.TipoRecarga, dadosRecargaB)
	if err := estado.AplicarTransacao(transacaoB); err != nil {
		log.Printf("[AVISO] Erro ao recarregar COMP-B: %v", err)
	}

	log.Println("[INICIAL] Créditos iniciais adicionados: COMP-A=100, COMP-B=50")
}
