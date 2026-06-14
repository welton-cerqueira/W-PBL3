package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"W-PBL3/internal/consenso"
	"W-PBL3/pkg/modelos"

	"github.com/gofiber/fiber/v2"
)

// Estrutura para armazenar informações do drone
type DroneInfo struct {
	ID    string
	Porta string
}

// healthCheck verifica se o broker está vivo
func (s *ServidorAPI) healthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "online",
		"lider":  s.raftNode.EhLider(),
		"porta":  s.porta,
	})
}

// consultarSaldo retorna o saldo de uma companhia
func (s *ServidorAPI) consultarSaldo(c *fiber.Ctx) error {
	companhiaID := c.Params("companhia_id")
	saldo := s.estado.ObterSaldo(companhiaID)

	return c.JSON(fiber.Map{
		"companhia_id": companhiaID,
		"saldo":        saldo,
	})
}

// consultarHistorico retorna todo o histórico do ledger
func (s *ServidorAPI) consultarHistorico(c *fiber.Ctx) error {
	historico := s.estado.ObterHistorico()
	return c.JSON(fiber.Map{
		"total_transacoes": len(historico),
		"historico":        historico,
	})
}

// listarMissoes lista todas as missões registradas
func (s *ServidorAPI) listarMissoes(c *fiber.Ctx) error {
	missoes := s.droneManager.ListarMissoesAtivas()
	return c.JSON(fiber.Map{
		"total_missoes": len(missoes),
		"missoes":       missoes,
	})
}

// requisitarDrone é o endpoint principal para companhias solicitarem drones
func (s *ServidorAPI) requisitarDrone(c *fiber.Ctx) error {
	var req struct {
		CompanhiaID string `json:"companhia_id"`
		Rota        string `json:"rota"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Requisição inválida"})
	}

	// Verifica se o nó atual é o líder
	if !s.raftNode.EhLider() {
		return c.Status(503).JSON(fiber.Map{
			"erro":  "Este não é o nó líder",
			"lider": s.raftNode.ObterLiderID(),
		})
	}

	// Cria requisição
	requisicao := modelos.NovaRequisicaoEscolta(req.CompanhiaID, req.Rota)

	// Cria transação de pagamento (custo fixo de 10 créditos)
	dadosPagamento := consenso.DadosPagamento{
		DeCompanhiaID:   req.CompanhiaID,
		ParaCompanhiaID: "sistema",
		Valor:           10,
		Motivo:          "requisicao_drone",
		IDRequisicao:    requisicao.IDRequisicao,
	}

	transacao, err := consenso.NovaTransacao(consenso.TipoPagamento, dadosPagamento)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": "Erro ao criar transação"})
	}

	// Aplica a transação via Raft
	if err := s.raftNode.AplicarTransacao(transacao); err != nil {
		if err.Error() == "saldo insuficiente" {
			return c.Status(402).JSON(fiber.Map{
				"erro":   "Saldo insuficiente",
				"status": "negada",
			})
		}
		return c.Status(500).JSON(fiber.Map{"erro": err.Error()})
	}

	// Pagamento confirmado! Alocar um drone
	droneID, err := s.droneManager.AlocarDrone(requisicao.IDRequisicao, req.Rota)
	if err != nil {
		log.Printf("Erro ao alocar drone: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"erro":   "Nenhum drone disponível",
			"status": "pendente",
		})
	}

	requisicao.DroneID = droneID
	requisicao.Status = modelos.StatusAprovada

	go s.notificarDrone(droneID, requisicao.IDRequisicao, req.Rota)

	return c.JSON(fiber.Map{
		"status":             "aprovada",
		"id_requisicao":      requisicao.IDRequisicao,
		"drone_id":           droneID,
		"creditos_debitados": 10,
		"saldo_restante":     s.estado.ObterSaldo(req.CompanhiaID),
	})

}

// recarregarCreditos adiciona créditos a uma companhia (apenas para testes)
func (s *ServidorAPI) recarregarCreditos(c *fiber.Ctx) error {
	var req struct {
		CompanhiaID string `json:"companhia_id"`
		Valor       int    `json:"valor"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Requisição inválida"})
	}

	if !s.raftNode.EhLider() {
		return c.Status(503).JSON(fiber.Map{"erro": "Este não é o nó líder"})
	}

	dadosRecarga := consenso.DadosRecarga{
		CompanhiaID:   req.CompanhiaID,
		Valor:         req.Valor,
		AutorizadoPor: "admin",
	}

	transacao, err := consenso.NovaTransacao(consenso.TipoRecarga, dadosRecarga)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": "Erro ao criar transação"})
	}

	if err := s.raftNode.AplicarTransacao(transacao); err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status":     "recarga realizada",
		"novo_saldo": s.estado.ObterSaldo(req.CompanhiaID),
	})
}

// relatarMissao recebe o laudo de um drone
func (s *ServidorAPI) relatarMissao(c *fiber.Ctx) error {
	var laudo modelos.LaudoMissao
	if err := c.BodyParser(&laudo); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Laudo inválido: " + err.Error()})
	}

	log.Printf("[LAUDO] Recebido laudo para missão: %s", laudo.IDRequisicao)
	log.Printf("[LAUDO] Drone: %s, Resultado: %s", laudo.DroneID, laudo.Resultado)

	// Verificação de hash desabilitada (conforme combinado)
	/*
	   if !laudo.VerificarIntegridade() {
	       log.Printf("[ALERTA] Laudo com hash inválido detectado!")
	       return c.Status(409).JSON(fiber.Map{
	           "erro":   "Laudo adulterado detectado!",
	           "status": "rejeitado",
	       })
	   }
	*/

	if !s.raftNode.EhLider() {
		return c.Status(503).JSON(fiber.Map{
			"erro":  "Este não é o nó líder",
			"lider": s.raftNode.ObterLiderID(),
		})
	}

	// --- Buscar a companhia associada a esta requisição ---
	var companhiaID string
	historico := s.estado.ObterHistorico()
	for _, tx := range historico {
		if tx.Tipo == consenso.TipoPagamento {
			var dadosPagamento consenso.DadosPagamento
			if err := json.Unmarshal(tx.Dados, &dadosPagamento); err == nil {
				if dadosPagamento.IDRequisicao == laudo.IDRequisicao {
					companhiaID = dadosPagamento.DeCompanhiaID
					break
				}
			}
		}
	}

	// Se não encontrou, usa um fallback (apenas para não quebrar)
	if companhiaID == "" {
		companhiaID = "desconhecida"
		log.Printf("[LAUDO] ⚠️ Não foi possível identificar a companhia para a missão %s", laudo.IDRequisicao)
	}

	// Registra o laudo como transação no ledger
	dadosLaudo := consenso.DadosLaudo{
		IDRequisicao:   laudo.IDRequisicao,
		DroneID:        laudo.DroneID,
		Rota:           laudo.Rota,
		Resultado:      laudo.Resultado,
		Obstaculos:     laudo.Obstaculos,
		Incidentes:     laudo.Incidentes,
		DataHoraInicio: laudo.DataHoraInicio,
		DataHoraFim:    laudo.DataHoraFim,
		Hash:           laudo.HashVerificacao,
		HashAnterior:   laudo.HashAnterior,
	}

	transacao, err := consenso.NovaTransacao(consenso.TipoLaudo, dadosLaudo)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": "Erro ao criar transação: " + err.Error()})
	}

	if err := s.raftNode.AplicarTransacao(transacao); err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": "Erro ao aplicar transação: " + err.Error()})
	}

	// Libera o drone
	s.droneManager.LiberarDrone(laudo.DroneID)
	log.Printf("[DRONE] Drone %s liberado após missão %s", laudo.DroneID, laudo.IDRequisicao)

	// --- Exibir conteúdo detalhado do laudo e total de tokens (créditos) da companhia ---
	saldo := s.estado.ObterSaldo(companhiaID)

	log.Printf("========== LAUDO COMPLETO ==========")
	log.Printf("ID Requisição: %s", laudo.IDRequisicao)
	log.Printf("Drone: %s", laudo.DroneID)
	log.Printf("Rota: %s", laudo.Rota)
	log.Printf("Resultado: %s", laudo.Resultado)
	if len(laudo.Obstaculos) > 0 {
		log.Printf("Obstáculos: %v", laudo.Obstaculos)
	} else {
		log.Printf("Obstáculos: nenhum")
	}
	if len(laudo.Incidentes) > 0 {
		log.Printf("Incidentes: %v", laudo.Incidentes)
	} else {
		log.Printf("Incidentes: nenhum")
	}
	log.Printf("Início: %d | Fim: %d", laudo.DataHoraInicio, laudo.DataHoraFim)
	log.Printf("Hash: %s", laudo.HashVerificacao)
	log.Printf("Hash Anterior: %s", laudo.HashAnterior)
	log.Printf("Total de tokens (créditos) da companhia %s: %d", companhiaID, saldo)
	log.Printf("====================================")

	return c.JSON(fiber.Map{
		"status":          "laudo registrado com sucesso",
		"id_requisicao":   laudo.IDRequisicao,
		"drone_liberado":  true,
		"saldo_companhia": saldo,
	})
}

func (s *ServidorAPI) registrarDrone(c *fiber.Ctx) error {
	var req struct {
		DroneID string `json:"drone_id"`
		Porta   string `json:"porta"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Requisição inválida"})
	}

	// Armazenar informações do drone
	s.droneManager.RegistrarDrone(req.DroneID, req.Porta)

	// Armazenar porta para notificação
	if s.drones == nil {
		s.drones = make(map[string]string)
	}
	s.drones[req.DroneID] = req.Porta

	log.Printf("[DRONE] Drone %s registrado na porta %s", req.DroneID, req.Porta)
	return c.JSON(fiber.Map{"status": "drone registrado", "drone_id": req.DroneID})
}

// Função para notificar drone via HTTP
func (s *ServidorAPI) notificarDrone(droneID, idRequisicao, rota string) {
	// Obter porta do drone
	porta, existe := s.drones[droneID]
	if !existe {
		log.Printf("[BROKER] ❌ Drone %s não encontrado no registro", droneID)
		return
	}

	// Preparar payload
	payload := map[string]interface{}{
		"id_requisicao": idRequisicao,
		"drone_id":      droneID,
		"rota":          rota,
	}
	jsonData, _ := json.Marshal(payload)

	// Fazer requisição HTTP para o drone
	url := fmt.Sprintf("http://localhost:%s/iniciar-missao", porta)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[BROKER] ❌ Erro ao notificar drone %s: %v", droneID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[BROKER] ✅ Drone %s notificado para missão %s", droneID, idRequisicao)
	} else {
		log.Printf("[BROKER] ⚠️ Drone %s respondeu com status %d", droneID, resp.StatusCode)
	}
}

// verificarCadeiaLaudos verifica a integridade da cadeia de laudos
func (s *ServidorAPI) verificarCadeiaLaudos(c *fiber.Ctx) error {
	integro, mensagem := s.estado.VerificarCadeiaLaudos()

	// Contagem segura de laudos
	totalLaudos := 0
	historico := s.estado.ObterHistorico()
	for _, t := range historico {
		if t.Tipo == consenso.TipoLaudo {
			totalLaudos++
		}
	}

	return c.JSON(fiber.Map{
		"cadeia_integra": integro,
		"mensagem":       mensagem,
		"total_laudos":   totalLaudos,
	})
}

// statusDrone retorna o status atual dos drones (para debug)
func (s *ServidorAPI) statusDrone(c *fiber.Ctx) error {
	drones := s.droneManager.ListarDrones()
	return c.JSON(fiber.Map{
		"drones": drones,
		"total":  len(drones),
		"portas": s.drones,
	})
}

// liberarDrone endpoint manual para liberar um drone (apenas para debug)
func (s *ServidorAPI) liberarDrone(c *fiber.Ctx) error {
	var req struct {
		DroneID string `json:"drone_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Requisição inválida"})
	}

	s.droneManager.LiberarDrone(req.DroneID)
	return c.JSON(fiber.Map{
		"status":   "drone liberado",
		"drone_id": req.DroneID,
	})
}

// receberComandoRaft recebe comandos de outros nós (replicação)
func (s *ServidorAPI) receberComandoRaft(c *fiber.Ctx) error {
	var transacao consenso.Transacao
	if err := c.BodyParser(&transacao); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Transação inválida"})
	}

	if err := s.estado.AplicarTransacao(&transacao); err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// obterEstatisticasLaudos retorna estatísticas dos laudos
func (s *ServidorAPI) obterEstatisticasLaudos(c *fiber.Ctx) error {
	estatisticas := s.estado.ObterEstatisticasLaudos()
	return c.JSON(estatisticas)
}
