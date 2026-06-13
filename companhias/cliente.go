package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	var (
		brokerURL   = flag.String("broker", "http://localhost:8080", "URL do broker")
		comando     = flag.String("cmd", "saldo", "Comando: saldo, requisitar, recarregar")
		companhiaID = flag.String("cid", "COMP-A", "ID da companhia")
		valor       = flag.Int("valor", 10, "Valor para recarga")
		rota        = flag.String("rota", "Rota Norte", "Rota para escolta")
	)
	flag.Parse()

	client := &http.Client{Timeout: 10 * time.Second}

	switch *comando {
	case "saldo":
		consultarSaldo(client, *brokerURL, *companhiaID)

	case "requisitar":
		requisitarDrone(client, *brokerURL, *companhiaID, *rota)

	case "recarregar":
		recarregarCreditos(client, *brokerURL, *companhiaID, *valor)

	default:
		log.Fatalf("Comando desconhecido: %s", *comando)
	}
}

func consultarSaldo(client *http.Client, brokerURL, companhiaID string) {
	url := fmt.Sprintf("%s/saldo/%s", brokerURL, companhiaID)
	resp, err := client.Get(url)
	if err != nil {
		log.Fatalf("Erro: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Resposta: %s\n", body)
}

func requisitarDrone(client *http.Client, brokerURL, companhiaID, rota string) {
	reqData := map[string]string{
		"companhia_id": companhiaID,
		"rota":         rota,
	}
	jsonData, _ := json.Marshal(reqData)

	url := fmt.Sprintf("%s/requisitar-drone", brokerURL)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Erro: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Resposta: %s\n", body)
}

func recarregarCreditos(client *http.Client, brokerURL, companhiaID string, valor int) {
	reqData := map[string]interface{}{
		"companhia_id": companhiaID,
		"valor":        valor,
	}
	jsonData, _ := json.Marshal(reqData)

	url := fmt.Sprintf("%s/recarregar", brokerURL)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("Erro: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Resposta: %s\n", body)
}
