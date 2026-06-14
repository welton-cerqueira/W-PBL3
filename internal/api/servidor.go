package api

import (
	"W-PBL3/internal/consenso"
	"W-PBL3/internal/drone"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

type ServidorAPI struct {
	app          *fiber.App
	porta        string
	estado       *consenso.EstadoLedger
	raftNode     *consenso.TCPRaft
	droneManager *drone.GerenciadorDrones
	drones       map[string]string
}

func NovoServidorAPI(porta string, raftNode *consenso.TCPRaft, droneManager *drone.GerenciadorDrones) *ServidorAPI {
	estado := raftNode.ObterEstado()
	app := fiber.New(fiber.Config{
		ServerHeader: "W-PBL3",
		AppName:      "W-PBL3 Broker",
	})
	app.Use(logger.New())
	app.Use(cors.New())

	return &ServidorAPI{
		app:          app,
		porta:        porta,
		estado:       estado,
		raftNode:     raftNode,
		droneManager: droneManager,
		drones:       make(map[string]string),
	}
}

// Iniciar inicia o servidor HTTP
func (s *ServidorAPI) Iniciar() error {
	return s.app.Listen(":" + s.porta)
}

// RegistrarRotas registra todas as rotas da API
func (s *ServidorAPI) RegistrarRotas() {
	// Rotas públicas (consulta)
	s.app.Get("/health", s.healthCheck)
	s.app.Get("/saldo/:companhia_id", s.consultarSaldo)
	s.app.Get("/historico", s.consultarHistorico)
	s.app.Get("/missoes", s.listarMissoes)
	s.app.Get("/leader", s.getLeader) // NOVA ROTA: retorna o líder atual

	// Rotas para companhias
	s.app.Post("/requisitar-drone", s.requisitarDrone)
	s.app.Post("/recarregar", s.recarregarCreditos)

	// Rotas para drones
	s.app.Post("/drone/registrar", s.registrarDrone)
	s.app.Post("/drone/relatar-missao", s.relatarMissao)
	s.app.Post("/drone/liberar", s.liberarDrone)

	// Rotas internas entre brokers
	s.app.Post("/raft/comando", s.receberComandoRaft)

	// Rotas de auditoria
	s.app.Get("/verificar-cadeia", s.verificarCadeiaLaudos)
	s.app.Get("/estatisticas", s.obterEstatisticasLaudos)
	s.app.Get("/drone/status", s.statusDrone)
}
