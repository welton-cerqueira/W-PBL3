package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Company struct {
	ID    string
	Nome  string
	Saldo int
}

var (
	companyID string
	brokerURL string
	rotas     = []string{"Rota Norte", "Rota Sul", "Rota Leste", "Rota Oeste", "Rota Central"}
)

func main() {
	flag.StringVar(&companyID, "id", "", "ID da companhia (ex: COMP-A)")
	flag.StringVar(&brokerURL, "broker", "http://broker1:8080", "URL do broker")
	flag.Parse()

	if companyID == "" {
		log.Fatal("ID da companhia é obrigatório")
	}

	log.Printf("[COMPANY %s] Iniciando simulador autônomo...", companyID)

	// Aguardar brokers iniciarem
	time.Sleep(5 * time.Second)

	// Simular comportamento da companhia
	go simularRequisicoes()

	// Aguardar sinal de término
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("[COMPANY %s] Encerrando...", companyID)
}

func simularRequisicoes() {
	for {
		// Aguardar entre 10-30 segundos entre requisições
		intervalo := time.Duration(10+rand.Intn(20)) * time.Second
		time.Sleep(intervalo)

		// Escolher rota aleatória
		rota := rotas[rand.Intn(len(rotas))]

		log.Printf("[COMPANY %s] Solicitando drone para %s", companyID, rota)

		resposta := requisitarDrone(rota)
		if resposta != "" {
			log.Printf("[COMPANY %s] Resposta: %s", companyID, resposta)
		}

		// Verificar saldo periodicamente
		if rand.Intn(5) == 0 { // 20% das vezes
			verificarSaldo()
		}
	}
}

func requisitarDrone(rota string) string {
	reqData := map[string]string{
		"companhia_id": companyID,
		"rota":         rota,
	}
	jsonData, _ := json.Marshal(reqData)

	resp, err := http.Post(brokerURL+"/requisitar-drone", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[COMPANY %s] Erro na requisição: %v", companyID, err)
		return ""
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Sprintf("Erro ao decodificar resposta: %v", err)
	}

	// Verificar se houve erro de saldo insuficiente
	if resp.StatusCode == 402 {
		log.Printf("[COMPANY %s] Saldo insuficiente! Recarregando...", companyID)
		recarregarCreditos(100) // Recarrega 100 créditos
	}

	// Extrair informações relevantes
	status := result["status"]
	if status == nil {
		status = result["erro"]
	}

	return fmt.Sprintf("HTTP %d: %v", resp.StatusCode, status)
}

func verificarSaldo() {
	resp, err := http.Get(brokerURL + "/saldo/" + companyID)
	if err != nil {
		log.Printf("[COMPANY %s] Erro ao verificar saldo: %v", companyID, err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[COMPANY %s] Erro ao decodificar saldo: %v", companyID, err)
		return
	}

	if saldo, ok := result["saldo"].(float64); ok {
		log.Printf("[COMPANY %s] Saldo atual: %.0f créditos", companyID, saldo)

		if saldo < 50 {
			log.Printf("[COMPANY %s] ⚠️ Saldo baixo (%.0f < 50). Recarregando 200 créditos...", companyID, saldo)
			recarregarCreditos(200)
		} else {
			log.Printf("[COMPANY %s] Saldo suficiente (%.0f). Nenhuma recarga necessária.", companyID, saldo)
		}
	}
}

func recarregarCreditos(valor int) {
	reqData := map[string]interface{}{
		"companhia_id": companyID,
		"valor":        valor,
	}
	jsonData, _ := json.Marshal(reqData)

	resp, err := http.Post(brokerURL+"/recarregar", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[COMPANY %s] Erro na recarga: %v", companyID, err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[COMPANY %s] Erro ao decodificar resposta da recarga: %v", companyID, err)
		return
	}

	if novoSaldo, ok := result["novo_saldo"].(float64); ok {
		log.Printf("[COMPANY %s] ✅ Recarga de %d créditos realizada! Novo saldo: %.0f", companyID, valor, novoSaldo)
	} else {
		log.Printf("[COMPANY %s] Recarga solicitada: %d créditos", companyID, valor)
	}
}
