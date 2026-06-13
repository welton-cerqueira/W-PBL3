package main

import (
	"flag"
	"log"
	"time"

	"W-PBL3/pkg/modelos"

	"github.com/gofiber/fiber/v2"
)

var (
	droneID   string
	porta     string
	brokerURL string
)

func main() {
	flag.StringVar(&droneID, "id", "drone1", "ID do drone")
	flag.StringVar(&porta, "porta", "9001", "Porta do drone")
	flag.StringVar(&brokerURL, "broker", "http://localhost:8080", "URL do broker")
	flag.Parse()

	app := fiber.New()

	// Endpoint para receber missões
	app.Post("/iniciar-missao", func(c *fiber.Ctx) error {
		var req struct {
			IDRequisicao string `json:"id_requisicao"`
			Rota         string `json:"rota"`
			DroneID      string `json:"drone_id"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"erro": "Requisição inválida"})
		}

		log.Printf("[DRONE %s] Missão recebida: Rota %s (ID: %s)",
			droneID, req.Rota, req.IDRequisicao)

		// Simula execução da missão
		go executarMissao(req.IDRequisicao, req.Rota)

		return c.JSON(fiber.Map{"status": "missão iniciada"})
	})

	// Registra o drone no broker
	registrarNoBroker()

	// Inicia o servidor
	log.Printf("[DRONE %s] Iniciando na porta %s", droneID, porta)
	log.Fatal(app.Listen(":" + porta))
}

func registrarNoBroker() {
	// Em produção, isso seria uma requisição HTTP para o broker
	log.Printf("[DRONE %s] Registrado no broker %s", droneID, brokerURL)
}

func executarMissao(idRequisicao, rota string) {
	log.Printf("[DRONE %s] Executando missão na rota %s...", droneID, rota)

	// Simula o tempo de voo
	time.Sleep(5 * time.Second)

	// Cria o laudo
	laudo := modelos.NovoLaudoMissao(idRequisicao, droneID, rota, modelos.ResultadoSucesso)
	laudo.AdicionarObstaculo("Detectado congestionamento leve")
	laudo.AdicionarIncidente("Comunicação instável por 2 segundos")
	laudo.FinalizarLaudo()

	// Envia laudo para o broker
	enviarLaudoParaBroker(laudo)
}

func enviarLaudoParaBroker(laudo *modelos.LaudoMissao) {
	// Simula envio via HTTP para o broker
	log.Printf("[DRONE %s] Enviando laudo da missão %s para broker",
		droneID, laudo.IDRequisicao)

	// Na implementação real, faria uma requisição HTTP POST
	// para http://localhost:8080/drone/relatar-missao

	log.Printf("[DRONE %s] Laudo enviado: %s", droneID, laudo.Resultado)
}
