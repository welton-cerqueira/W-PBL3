package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	droneID     string
	brokerURL   string
	registrado  bool
	emMissao    bool
	missaoAtual string
)

func main() {
	flag.StringVar(&droneID, "id", "", "ID do drone (ex: drone1)")
	flag.StringVar(&brokerURL, "broker", "http://localhost:8080", "URL do broker")
	flag.Parse()

	if droneID == "" {
		log.Fatal("ID do drone é obrigatório")
	}

	log.Printf("[DRONE %s] Iniciando simulador autônomo...", droneID)

	// Registrar no broker
	registrarNoBroker()

	// Simular comportamento autônomo
	go simularComportamento()

	// Aguardar sinal de término
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("[DRONE %s] Encerrando...", droneID)
}

func registrarNoBroker() {
	for i := 0; i < 10; i++ {
		reqData := map[string]string{
			"drone_id": droneID,
			"porta":    "9001",
		}
		jsonData, _ := json.Marshal(reqData)

		resp, err := http.Post(brokerURL+"/drone/registrar", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[DRONE %s] Erro ao registrar (tentativa %d): %v", droneID, i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			log.Printf("[DRONE %s] ✅ Registrado com sucesso no broker %s", droneID, brokerURL)
			registrado = true
			return
		}
	}
	log.Printf("[DRONE %s] ❌ Falha ao registrar após 10 tentativas", droneID)
}

func simularComportamento() {
	// Aguardar registro
	time.Sleep(2 * time.Second)

	for registrado {
		if !emMissao {
			log.Printf("[DRONE %s] 🟢 Disponível - aguardando missões...", droneID)
			time.Sleep(3 * time.Second)

			// Simular que recebeu uma missão (para teste)
			// Em produção, isso viria do broker via webhook
			if rand.Intn(10) == 0 { // 10% de chance de simular missão
				missaoAtual = "missao_simulada_" + time.Now().Format("150405")
				emMissao = true
				log.Printf("[DRONE %s] 🚀 Missão simulada recebida: %s", droneID, missaoAtual)
				go executarMissao()
			}
		}
	}
}

func executarMissao() {
	log.Printf("[DRONE %s] ✈️ Executando missão %s...", droneID, missaoAtual)

	// Simular tempo de voo entre 5-15 segundos
	tempoVoo := time.Duration(5+rand.Intn(10)) * time.Second
	time.Sleep(tempoVoo)

	// Enviar laudo da missão
	enviarLaudo()
}

func enviarLaudo() {
	// Gerar dados do laudo
	obstaculos := []string{}
	if rand.Intn(3) == 0 {
		obstaculos = append(obstaculos, "Congestionamento detectado")
	}
	if rand.Intn(4) == 0 {
		obstaculos = append(obstaculos, "Objeto flutuante na rota")
	}
	if rand.Intn(5) == 0 {
		obstaculos = append(obstaculos, "Navio parado na rota")
	}

	incidentes := []string{}
	if rand.Intn(5) == 0 {
		incidentes = append(incidentes, "Perda de sinal temporária")
	}
	if rand.Intn(8) == 0 {
		incidentes = append(incidentes, "Bateria em nível crítico")
	}

	laudo := map[string]interface{}{
		"id_requisicao":    missaoAtual,
		"drone_id":         droneID,
		"rota":             "Rota " + string(rune(65+rand.Intn(5))),
		"resultado":        "sucesso",
		"obstaculos":       obstaculos,
		"incidentes":       incidentes,
		"data_hora_inicio": time.Now().Unix() - int64(tempoVoo.Seconds()),
		"data_hora_fim":    time.Now().Unix(),
		"hash_anterior":    "",
		"hash_verificacao": gerarHash(missaoAtual),
	}

	jsonData, _ := json.Marshal(laudo)

	resp, err := http.Post(brokerURL+"/drone/relatar-missao", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[DRONE %s] ❌ Erro ao enviar laudo: %v", droneID, err)
		emMissao = false
		missaoAtual = ""
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[DRONE %s] ✅ Laudo enviado com sucesso para missão %s", droneID, missaoAtual)
		log.Printf("[DRONE %s] 📋 Obstáculos: %v", droneID, obstaculos)
		if len(incidentes) > 0 {
			log.Printf("[DRONE %s] ⚠️ Incidentes: %v", droneID, incidentes)
		}
		emMissao = false
		missaoAtual = ""
	} else {
		log.Printf("[DRONE %s] ❌ Falha ao enviar laudo. Status: %d", droneID, resp.StatusCode)
		emMissao = false
		missaoAtual = ""
	}
}

func gerarHash(s string) string {
	return "hash_" + time.Now().Format("150405") + "_" + s[:min(8, len(s))]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var tempoVoo time.Duration
