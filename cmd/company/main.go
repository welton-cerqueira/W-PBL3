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
	"strings"
	"syscall"
	"time"
)

var (
	companyID  string
	brokerList string            // lista de brokers (ex: "broker1=192.168.1.10:8080,broker2=...")
	brokers    map[string]string // id -> endereço
	rotas      = []string{"Rota Norte", "Rota Sul", "Rota Leste", "Rota Oeste", "Rota Central"}
)

func main() {
	flag.StringVar(&companyID, "id", "", "ID da companhia (ex: COMP-A)")
	flag.StringVar(&brokerList, "brokers", "", "Lista de brokers (ex: broker1=192.168.1.10:8080,broker2=...)")
	flag.Parse()

	if companyID == "" || brokerList == "" {
		log.Fatal("Flags -id e -brokers são obrigatórias")
	}

	// Parse da lista de brokers
	brokers = make(map[string]string)
	for _, part := range strings.Split(brokerList, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			brokers[kv[0]] = kv[1]
		}
	}
	if len(brokers) == 0 {
		log.Fatal("Nenhum broker válido fornecido")
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

// descobreLider consulta todos os brokers até encontrar o líder atual
func descobreLider() (string, error) {
	for id, urlBase := range brokers {
		url := fmt.Sprintf("http://%s/leader", urlBase)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("[COMPANY %s] Broker %s (%s) indisponível: %v", companyID, id, urlBase, err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var leaderInfo struct {
			LeaderID   string `json:"leader_id"`
			LeaderAddr string `json:"leader_addr"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&leaderInfo); err == nil && leaderInfo.LeaderAddr != "" {
			log.Printf("[COMPANY %s] Líder atual: %s (%s)", companyID, leaderInfo.LeaderID, leaderInfo.LeaderAddr)
			return leaderInfo.LeaderAddr, nil
		}
	}
	return "", fmt.Errorf("não foi possível determinar o líder")
}

// doRequestWithRedirect envia uma requisição HTTP, seguindo redirecionamentos e redescobrindo o líder se necessário
func doRequestWithRedirect(method, url string, body []byte) (*http.Response, error) {
	maxTentativas := 5
	for i := 0; i < maxTentativas; i++ {
		req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[COMPANY %s] Erro de conexão com %s, tentando descobrir novo líder...", companyID, url)
			leader, err2 := descobreLider()
			if err2 != nil {
				return nil, fmt.Errorf("falha ao descobrir líder: %v", err2)
			}
			// Extrai o caminho da URL original (ex: "/requisitar-drone")
			path := "/"
			if strings.Contains(url, "/") {
				parts := strings.SplitN(url, "/", 4)
				if len(parts) >= 4 {
					path = "/" + parts[3]
				} else if len(parts) == 3 {
					path = "/" + parts[2]
				}
			}
			url = "http://" + leader + path
			continue
		}
		if resp.StatusCode == http.StatusTemporaryRedirect {
			loc := resp.Header.Get("Location")
			resp.Body.Close()
			if loc == "" {
				return nil, fmt.Errorf("redirect sem location")
			}
			url = loc
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("excedido número de tentativas")
}

func simularRequisicoes() {
	for {
		intervalo := time.Duration(10+rand.Intn(20)) * time.Second
		time.Sleep(intervalo)

		rota := rotas[rand.Intn(len(rotas))]
		log.Printf("[COMPANY %s] Solicitando drone para %s", companyID, rota)

		resposta := requisitarDrone(rota)
		if resposta != "" {
			log.Printf("[COMPANY %s] Resposta: %s", companyID, resposta)
		}

		if rand.Intn(5) == 0 {
			verificarSaldo()
		}
	}
}

func requisitarDrone(rota string) string {
	leaderAddr, err := descobreLider()
	if err != nil {
		log.Printf("[COMPANY %s] ❌ Não foi possível encontrar líder: %v", companyID, err)
		return ""
	}
	url := "http://" + leaderAddr + "/requisitar-drone"

	reqData := map[string]string{
		"companhia_id": companyID,
		"rota":         rota,
	}
	jsonData, _ := json.Marshal(reqData)

	resp, err := doRequestWithRedirect("POST", url, jsonData)
	if err != nil {
		log.Printf("[COMPANY %s] Erro na requisição: %v", companyID, err)
		return ""
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Sprintf("Erro ao decodificar resposta: %v", err)
	}

	if resp.StatusCode == 402 {
		log.Printf("[COMPANY %s] Saldo insuficiente! Recarregando...", companyID)
		recarregarCreditos(100)
	}

	status := result["status"]
	if status == nil {
		status = result["erro"]
	}
	return fmt.Sprintf("HTTP %d: %v", resp.StatusCode, status)
}

func verificarSaldo() {
	leaderAddr, err := descobreLider()
	if err != nil {
		log.Printf("[COMPANY %s] ❌ Não foi possível encontrar líder: %v", companyID, err)
		return
	}
	url := "http://" + leaderAddr + "/saldo/" + companyID

	resp, err := doRequestWithRedirect("GET", url, nil)
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
	leaderAddr, err := descobreLider()
	if err != nil {
		log.Printf("[COMPANY %s] ❌ Não foi possível encontrar líder: %v", companyID, err)
		return
	}
	url := "http://" + leaderAddr + "/recarregar"

	reqData := map[string]interface{}{
		"companhia_id": companyID,
		"valor":        valor,
	}
	jsonData, _ := json.Marshal(reqData)

	resp, err := doRequestWithRedirect("POST", url, jsonData)
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
