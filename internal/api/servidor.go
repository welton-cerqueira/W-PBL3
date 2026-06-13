package api

import (
	"W-PBL3/internal/consenso"
	"W-PBL3/internal/drone"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

// ServidorAPI representa o servidor HTTP do broker
type ServidorAPI struct {
	app          *fiber.App
	porta        string
	estado       *consenso.EstadoLedger
	raftNode     *consenso.NoRaft
	droneManager *drone.GerenciadorDrones
}

// NovoServidorAPI cria um novo servidor HTTP
func NovoServidorAPI(porta string, estado *consenso.EstadoLedger, raftNode *consenso.NoRaft, droneManager *drone.GerenciadorDrones) *ServidorAPI {
	app := fiber.New(fiber.Config{
		ServerHeader: "W-PBL3",
		AppName:      "W-PBL3 Broker",
	})

	// Middlewares
	app.Use(logger.New())
	app.Use(cors.New())

	return &ServidorAPI{
		app:          app,
		porta:        porta,
		estado:       estado,
		raftNode:     raftNode,
		droneManager: droneManager,
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

	// Rotas para companhias
	s.app.Post("/requisitar-drone", s.requisitarDrone)
	s.app.Post("/recarregar", s.recarregarCreditos)

	// Rotas para drones
	s.app.Post("/drone/registrar", s.registrarDrone)
	s.app.Post("/drone/relatar-missao", s.relatarMissao)

	// Rota para verificar integridade da cadeia
	s.app.Get("/verificar-cadeia", s.verificarCadeiaLaudos)

	// Nova rota para estatísticas
	s.app.Get("/estatisticas", s.obterEstatisticasLaudos)

	// Rotas internas entre brokers
	s.app.Post("/raft/comando", s.receberComandoRaft)

	// Nova rota para verificar integridade da cadeia
	s.app.Get("/verificar-cadeia", s.verificarCadeiaLaudos)
}
