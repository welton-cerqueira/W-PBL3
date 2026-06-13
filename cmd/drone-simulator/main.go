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
	porta       string
	registrado  bool
	emMissao    bool
	missaoAtual string
	rotaAtual   string
)

func main() {
	flag.StringVar(&droneID, "id", "", "ID do drone (ex: drone1)")
	flag.StringVar(&brokerURL, "broker", "http://localhost:8080", "URL do broker")
	flag.StringVar(&porta, "porta", "9001", "Porta para receber missões")
	flag.Parse()

	if droneID == "" {
		log.Fatal("ID do drone é obrigatório")
	}

	log.Printf("[DRONE %s] Iniciando simulador autônomo...", droneID)

	// Registrar no broker
	registrarNoBroker()

	// Iniciar servidor HTTP para receber missões
	go iniciarServidorHTTP()

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
			"porta":    porta,
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

func iniciarServidorHTTP() {
	http.HandleFunc("/iniciar-missao", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[DRONE %s] Erro ao decodificar requisição: %v", droneID, err)
			http.Error(w, "Erro ao decodificar", 400)
			return
		}

		idRequisicao, ok := req["id_requisicao"].(string)
		if !ok {
			log.Printf("[DRONE %s] Campo id_requisicao inválido", droneID)
			http.Error(w, "id_requisicao inválido", 400)
			return
		}

		rota, _ := req["rota"].(string)

		log.Printf("[DRONE %s] 🚀 Missão REAL recebida do broker: %s (Rota: %s)", droneID, idRequisicao, rota)

		// Iniciar missão em background
		go executarMissaoReal(idRequisicao, rota)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "missão iniciada"})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "drone": droneID})
	})

	log.Printf("[DRONE %s] 🌐 Servidor HTTP iniciado na porta %s", droneID, porta)
	log.Fatal(http.ListenAndServe(":"+porta, nil))
}

func executarMissaoReal(idRequisicao, rota string) {
	if emMissao {
		log.Printf("[DRONE %s] ⚠️ Já em missão, ignorando nova missão %s", droneID, idRequisicao)
		return
	}

	emMissao = true
	missaoAtual = idRequisicao
	rotaAtual = rota

	log.Printf("[DRONE %s] ✈️ Executando missão REAL %s na rota %s...", droneID, missaoAtual, rotaAtual)

	// Simular tempo de voo entre 5-15 segundos
	tempoVoo := time.Duration(5+rand.Intn(10)) * time.Second
	time.Sleep(tempoVoo)

	// Enviar laudo da missão
	enviarLaudo()
}

func enviarLaudo() {
	// Gerar dados do laudo baseados na missão real
	obstaculos := []string{}
	if rand.Intn(3) == 0 {
		obstaculos = append(obstaculos, "Congestionamento detectado na rota")
	}
	if rand.Intn(4) == 0 {
		obstaculos = append(obstaculos, "Objeto flutuante na posição 23°S")
	}
	if rand.Intn(5) == 0 {
		obstaculos = append(obstaculos, "Navio cargueiro parado na rota")
	}
	if rand.Intn(6) == 0 {
		obstaculos = append(obstaculos, "Zona de neblina densa")
	}

	incidentes := []string{}
	if rand.Intn(5) == 0 {
		incidentes = append(incidentes, "Perda de sinal por 3 segundos")
	}
	if rand.Intn(8) == 0 {
		incidentes = append(incidentes, "Bateria em nível crítico (15%)")
	}
	if rand.Intn(10) == 0 {
		incidentes = append(incidentes, "Comunicação com broker instável")
	}

	// Escolher resultado (90% sucesso, 10% falha)
	resultado := "sucesso"
	if rand.Intn(10) == 0 {
		resultado = "falha"
		incidentes = append(incidentes, "Missão não completada devido a falha técnica")
	}

	inicio := time.Now().Unix() - int64(5+rand.Intn(10))
	fim := time.Now().Unix()

	laudo := map[string]interface{}{
		"id_requisicao":    missaoAtual,
		"drone_id":         droneID,
		"rota":             rotaAtual,
		"resultado":        resultado,
		"obstaculos":       obstaculos,
		"incidentes":       incidentes,
		"data_hora_inicio": inicio,
		"data_hora_fim":    fim,
		"hash_anterior":    "",
		"hash_verificacao": gerarHash(missaoAtual),
	}

	jsonData, _ := json.Marshal(laudo)

	log.Printf("[DRONE %s] 📤 Enviando laudo para missão %s...", droneID, missaoAtual)

	resp, err := http.Post(brokerURL+"/drone/relatar-missao", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[DRONE %s] ❌ Erro ao enviar laudo: %v", droneID, err)
		emMissao = false
		missaoAtual = ""
		rotaAtual = ""
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[DRONE %s] ✅ Laudo enviado com sucesso para missão %s", droneID, missaoAtual)
		log.Printf("[DRONE %s] 📋 Resultado: %s", droneID, resultado)
		if len(obstaculos) > 0 {
			log.Printf("[DRONE %s] 📋 Obstáculos: %v", droneID, obstaculos)
		}
		if len(incidentes) > 0 {
			log.Printf("[DRONE %s] ⚠️ Incidentes: %v", droneID, incidentes)
		}
	} else {
		log.Printf("[DRONE %s] ❌ Falha ao enviar laudo. Status: %d", droneID, resp.StatusCode)
	}

	emMissao = false
	missaoAtual = ""
	rotaAtual = ""
}

func gerarHash(s string) string {
	if len(s) < 8 {
		return "hash_" + time.Now().Format("150405") + "_" + s
	}
	return "hash_" + time.Now().Format("150405") + "_" + s[:8]
}
