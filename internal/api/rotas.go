package api

import (
	"log"

	"W-PBL3/internal/consenso"
	"W-PBL3/pkg/modelos"

	"github.com/gofiber/fiber/v2"
)

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

// registrarDrone registra um drone no sistema
func (s *ServidorAPI) registrarDrone(c *fiber.Ctx) error {
	var req struct {
		DroneID string `json:"drone_id"`
		Porta   string `json:"porta"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Requisição inválida"})
	}

	s.droneManager.RegistrarDrone(req.DroneID, req.Porta)
	return c.JSON(fiber.Map{"status": "drone registrado", "drone_id": req.DroneID})
}

// verificarCadeiaLaudos verifica a integridade de toda a cadeia de laudos
func (s *ServidorAPI) verificarCadeiaLaudos(c *fiber.Ctx) error {
	integro, mensagem := s.estado.VerificarCadeiaLaudos()

	return c.JSON(fiber.Map{
		"cadeia_integra": integro,
		"mensagem":       mensagem,
		"total_laudos":   len(s.estado.ObterHistorico()),
	})
}

// relatarMissao recebe o laudo de um drone
func (s *ServidorAPI) relatarMissao(c *fiber.Ctx) error {
	var laudo modelos.LaudoMissao
	if err := c.BodyParser(&laudo); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Laudo inválido: " + err.Error()})
	}

	// Verifica a integridade do laudo recebido
	if !laudo.VerificarIntegridade() {
		log.Printf("[ALERTA] Laudo com hash inválido detectado! Possível adulteração.")
		return c.Status(409).JSON(fiber.Map{
			"erro":   "Laudo adulterado detectado!",
			"status": "rejeitado",
		})
	}

	// LOG COMPLETO DO LAUDO RECEBIDO
	log.Printf("[LAUDO] Recebido laudo da missão %s:", laudo.IDRequisicao)
	log.Printf("  - Drone: %s", laudo.DroneID)
	log.Printf("  - Rota: %s", laudo.Rota)
	log.Printf("  - Resultado: %s", laudo.Resultado)
	log.Printf("  - Obstáculos: %v", laudo.Obstaculos)
	log.Printf("  - Incidentes: %v", laudo.Incidentes)
	log.Printf("  - Hash: %s", laudo.HashVerificacao[:16]+"...")
	log.Printf("  - Hash Anterior: %s", laudo.HashAnterior[:16]+"...")

	if !s.raftNode.EhLider() {
		return c.Status(503).JSON(fiber.Map{
			"erro":  "Este não é o nó líder",
			"lider": s.raftNode.ObterLiderID(),
		})
	}

	// Registra o laudo como transação no ledger com HASH
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

	return c.JSON(fiber.Map{
		"status":        "laudo registrado com sucesso",
		"id_laudo":      laudo.ID,
		"id_missao":     laudo.IDRequisicao,
		"hash":          laudo.HashVerificacao,
		"hash_anterior": laudo.HashAnterior,
		"integridade":   "verificada",
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
