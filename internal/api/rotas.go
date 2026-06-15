package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"W-PBL3/internal/consenso"
	"W-PBL3/internal/crypto"
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
	estado := s.raftNode.ObterEstado()
	companhiaID := c.Params("companhia_id")
	saldo := estado.ObterSaldo(companhiaID)

	return c.JSON(fiber.Map{
		"companhia_id": companhiaID,
		"saldo":        saldo,
	})
}

// consultarHistorico retorna todo o histórico do ledger
func (s *ServidorAPI) consultarHistorico(c *fiber.Ctx) error {
	estado := s.raftNode.ObterEstado()
	historico := estado.ObterHistorico()
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
		CompanhiaID  string `json:"companhia_id"`
		Rota         string `json:"rota"`
		Timestamp    int64  `json:"timestamp"`
		Nonce        string `json:"nonce"`
		ChavePublica string `json:"chave_publica"`
		Assinatura   string `json:"assinatura"`
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

	// --- Verificação da assinatura digital da companhia ---
	dadosStr := fmt.Sprintf("%s:%s:%d:%s", req.CompanhiaID, req.Rota, req.Timestamp, req.Nonce)
	chavePub, err := crypto.ImportarChavePublicaBase64(req.ChavePublica)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"erro": "Chave pública inválida"})
	}
	if !crypto.Verificar(chavePub, []byte(dadosStr), req.Assinatura) {
		return c.Status(401).JSON(fiber.Map{"erro": "Assinatura inválida"})
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
		Timestamp:       req.Timestamp,
		Nonce:           req.Nonce,
		ChavePublica:    req.ChavePublica,
		Assinatura:      req.Assinatura,
	}

	transacao, err := consenso.NovaTransacao(consenso.TipoPagamento, dadosPagamento)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": "Erro ao criar transação"})
	}

	// Aplica a transação via Raft
	if _, err := s.raftNode.AplicarTransacao(transacao); err != nil {
		if strings.Contains(err.Error(), "saldo insuficiente") {
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
	estado := s.raftNode.ObterEstado()

	go s.notificarDrone(droneID, requisicao.IDRequisicao, req.Rota, req.CompanhiaID)

	return c.JSON(fiber.Map{
		"status":             "aprovada",
		"id_requisicao":      requisicao.IDRequisicao,
		"drone_id":           droneID,
		"creditos_debitados": 10,
		"saldo_restante":     estado.ObterSaldo(req.CompanhiaID),
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

	novoSaldo, err := s.raftNode.AplicarTransacao(transacao)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": err.Error()})
	}

	// Se por algum motivo o novoSaldo for 0, podemos buscá-lo do estado (fallback)
	if novoSaldo == 0 {
		estado := s.raftNode.ObterEstado()
		novoSaldo = estado.ObterSaldo(req.CompanhiaID)
	}

	return c.JSON(fiber.Map{
		"status":     "recarga realizada",
		"novo_saldo": novoSaldo,
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

	// ===== VERIFICAÇÃO DE ASSINATURA =====
	dadosStr := fmt.Sprintf("%s:%s:%s:%s:%d:%d:%s",
		laudo.IDRequisicao,
		laudo.DroneID,
		laudo.Rota,
		laudo.Resultado,
		laudo.DataHoraInicio,
		laudo.DataHoraFim,
		laudo.HashAnterior,
	)
	chavePub, err := crypto.ImportarChavePublicaBase64(laudo.ChavePublica)
	if err != nil {
		log.Printf("[LAUDO] Chave pública inválida para missão %s: %v", laudo.IDRequisicao, err)
		return c.Status(401).JSON(fiber.Map{"erro": "Chave pública inválida"})
	}
	if !crypto.Verificar(chavePub, []byte(dadosStr), laudo.Assinatura) {
		log.Printf("[LAUDO] Assinatura inválida para missão %s", laudo.IDRequisicao)
		return c.Status(401).JSON(fiber.Map{"erro": "Assinatura inválida"})
	}
	// =====================================

	if !s.raftNode.EhLider() {
		return c.Status(503).JSON(fiber.Map{
			"erro":  "Este não é o nó líder",
			"lider": s.raftNode.ObterLiderID(),
		})
	}

	// --- Obter a companhia diretamente do laudo (campo enviado pelo drone) ---
	companhiaID := laudo.CompanhiaID
	if companhiaID == "" {
		companhiaID = "desconhecida"
		log.Printf("[LAUDO] ⚠️ Laudo sem companhia_id para missão %s", laudo.IDRequisicao)
	}

	// Registra o laudo como transação no ledger (inclui os campos de assinatura, se desejar)
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
		ChavePublica:   laudo.ChavePublica,
		Assinatura:     laudo.Assinatura,
	}

	transacao, err := consenso.NovaTransacao(consenso.TipoLaudo, dadosLaudo)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": "Erro ao criar transação: " + err.Error()})
	}

	if _, err := s.raftNode.AplicarTransacao(transacao); err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": "Erro ao aplicar transação: " + err.Error()})
	}

	// Libera o drone
	s.droneManager.LiberarDrone(laudo.DroneID)
	log.Printf("[DRONE] Drone %s liberado após missão %s", laudo.DroneID, laudo.IDRequisicao)

	estado := s.raftNode.ObterEstado()
	saldo := estado.ObterSaldo(companhiaID)

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

// registrarDrone registra um drone no sistema
// NOTA: Em ambiente distribuído, o drone deve enviar seu endereço completo (ex: "192.168.1.20:9001")
// no campo "addr". Atualmente o campo "porta" é usado, mas deve ser substituído por "addr".
func (s *ServidorAPI) registrarDrone(c *fiber.Ctx) error {
	var req struct {
		DroneID string `json:"drone_id"`
		Addr    string `json:"addr"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"erro": "Requisição inválida"})
	}

	s.droneManager.RegistrarDrone(req.DroneID, req.Addr)

	s.dronesMu.Lock()
	if s.drones == nil {
		s.drones = make(map[string]string)
	}
	s.drones[req.DroneID] = req.Addr
	s.dronesMu.Unlock()

	log.Printf("[DRONE] Drone %s registrado em %s", req.DroneID, req.Addr)
	return c.JSON(fiber.Map{"status": "drone registrado", "drone_id": req.DroneID})
}

// notificarDrone via HTTP
// ATENÇÃO: Esta função ainda assume que o drone está em "localhost". Para funcionar em rede,
// o drone deve fornecer seu endereço IP completo no registro, e este endereço deve ser usado aqui.
func (s *ServidorAPI) notificarDrone(droneID, idRequisicao, rota, companhiaID string) {
	s.dronesMu.RLock()
	endereco, existe := s.drones[droneID]
	s.dronesMu.RUnlock()
	if !existe {
		log.Printf("[BROKER] ❌ Drone %s não encontrado no registro", droneID)
		return
	}

	// Preparar payload
	payload := map[string]interface{}{
		"id_requisicao": idRequisicao,
		"drone_id":      droneID,
		"rota":          rota,
		"companhia_id":  companhiaID,
	}
	jsonData, _ := json.Marshal(payload)

	// Fazer requisição HTTP para o drone
	// TODO: usar o endereço completo (se for "192.168.1.20:9001", já funciona; se for só porta, assume localhost)
	url := fmt.Sprintf("http://%s/iniciar-missao", endereco)
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
	estado := s.raftNode.ObterEstado()
	integro, mensagem := estado.VerificarCadeiaLaudos()

	// Contagem segura de laudos
	totalLaudos := 0
	historico := estado.ObterHistorico()
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
	s.dronesMu.RLock()
	portas := make(map[string]string)
	for k, v := range s.drones {
		portas[k] = v
	}
	s.dronesMu.RUnlock()

	drones := s.droneManager.ListarDrones()
	return c.JSON(fiber.Map{
		"drones": drones,
		"total":  len(drones),
		"portas": portas,
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
	estado := s.raftNode.ObterEstado()
	if err := estado.AplicarTransacao(&transacao); err != nil {
		return c.Status(500).JSON(fiber.Map{"erro": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// obterEstatisticasLaudos retorna estatísticas dos laudos
func (s *ServidorAPI) obterEstatisticasLaudos(c *fiber.Ctx) error {
	estado := s.raftNode.ObterEstado()
	estatisticas := estado.ObterEstatisticasLaudos()
	return c.JSON(estatisticas)
}
