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
	droneID              string
	addr                 string // endereço completo (ex: "192.168.1.20:9001")
	brokerList           string // lista de brokers: "broker1=192.168.1.10:8080,broker2=..."
	brokers              map[string]string
	registrado           bool
	emMissao             bool
	missaoAtual          string
	rotaAtual            string
	ultimoLiderAddr      string // armazena o último líder conhecido
	missaoAtualCompanhia string
)

func main() {
	flag.StringVar(&droneID, "id", "", "ID do drone (ex: drone1)")
	flag.StringVar(&addr, "addr", "", "Endereço completo do drone (ex: 192.168.1.20:9001)")
	flag.StringVar(&brokerList, "brokers", "", "Lista de brokers (ex: broker1=192.168.1.10:8080,broker2=...)")
	flag.Parse()

	if droneID == "" || addr == "" || brokerList == "" {
		log.Fatal("Flags -id, -addr e -brokers são obrigatórias")
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

	log.Printf("[DRONE %s] Iniciando simulador autônomo (endereço %s)", droneID, addr)

	// Registrar no broker (com tentativas até encontrar um líder)
	registrarNoBroker()

	// Iniciar servidor HTTP para receber missões
	go iniciarServidorHTTP()

	// Iniciar monitoramento de líder (re-registro automático)
	go monitorarLider()

	// Aguardar sinal de término
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("[DRONE %s] Encerrando...", droneID)
}

// monitorarLider verifica periodicamente se o líder mudou e, se sim, reregistra o drone
func monitorarLider() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !registrado {
			continue
		}
		leader, err := descobreLider()
		if err != nil {
			log.Printf("[DRONE %s] Erro ao descobrir líder para monitoramento: %v", droneID, err)
			continue
		}
		if leader != ultimoLiderAddr {
			log.Printf("[DRONE %s] Líder mudou de %s para %s. Re-registrando...", droneID, ultimoLiderAddr, leader)
			registrarNoBroker()
		}
	}
}

// descobreLider consulta todos os brokers até encontrar o líder atual
func descobreLider() (string, error) {
	for id, urlBase := range brokers {
		url := fmt.Sprintf("http://%s/leader", urlBase)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("[DRONE %s] Broker %s (%s) indisponível: %v", droneID, id, urlBase, err)
			continue
		}
		// Leitura do corpo
		var leaderInfo struct {
			LeaderID   string `json:"leader_id"`
			LeaderAddr string `json:"leader_addr"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&leaderInfo); err == nil && leaderInfo.LeaderAddr != "" {
			resp.Body.Close()
			log.Printf("[DRONE %s] Líder atual: %s (%s)", droneID, leaderInfo.LeaderID, leaderInfo.LeaderAddr)
			return leaderInfo.LeaderAddr, nil
		}
		resp.Body.Close()
	}
	return "", fmt.Errorf("não foi possível determinar o líder")
}

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
			log.Printf("[DRONE %s] Erro de conexão com %s, tentando descobrir novo líder...", droneID, url)
			leader, err2 := descobreLider()
			if err2 != nil {
				return nil, fmt.Errorf("falha ao descobrir líder: %v", err2)
			}
			// Extrai o caminho da URL original
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

func registrarNoBroker() {
	// Descobre o líder antes de registrar
	leaderAddr, err := descobreLider()
	if err != nil {
		log.Printf("[DRONE %s] ❌ Não foi possível encontrar líder: %v", droneID, err)
		return
	}
	registrarURL := "http://" + leaderAddr + "/drone/registrar"
	reqData := map[string]string{
		"drone_id": droneID,
		"addr":     addr, // envia endereço completo
	}
	jsonData, _ := json.Marshal(reqData)

	resp, err := http.Post(registrarURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[DRONE %s] ❌ Falha ao registrar: %v", droneID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		log.Printf("[DRONE %s] ✅ Registrado com sucesso no líder %s", droneID, leaderAddr)
		registrado = true
		ultimoLiderAddr = leaderAddr
	} else {
		log.Printf("[DRONE %s] ❌ Registro recusado, status %d", droneID, resp.StatusCode)
	}
}

func iniciarServidorHTTP() {
	http.HandleFunc("/iniciar-missao", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDRequisicao string `json:"id_requisicao"`
			Rota         string `json:"rota"`
			DroneID      string `json:"drone_id"`
			CompanhiaID  string `json:"companhia_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[DRONE %s] Erro ao decodificar requisição: %v", droneID, err)
			http.Error(w, "Erro ao decodificar", 400)
			return
		}
		log.Printf("[DRONE %s] 🚀 Missão REAL recebida do broker: %s (Rota: %s, Companhia: %s)", droneID, req.IDRequisicao, req.Rota, req.CompanhiaID)
		missaoAtualCompanhia = req.CompanhiaID
		go executarMissaoReal(req.IDRequisicao, req.Rota)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "missão iniciada"})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "drone": droneID})
	})

	log.Printf("[DRONE %s] 🌐 Servidor HTTP iniciado em %s", droneID, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
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
	tempoVoo := time.Duration(5+rand.Intn(10)) * time.Second
	time.Sleep(tempoVoo)

	enviarLaudo()
}

func enviarLaudo() {
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
		"companhia_id":     missaoAtualCompanhia,
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

	// Descobre líder atual antes de enviar
	leaderAddr, err := descobreLider()
	if err != nil {
		log.Printf("[DRONE %s] ❌ Não foi possível encontrar líder: %v", droneID, err)
		emMissao = false
		missaoAtual = ""
		rotaAtual = ""
		return
	}
	url := "http://" + leaderAddr + "/drone/relatar-missao"
	resp, err := doRequestWithRedirect("POST", url, jsonData)
	if err != nil {
		log.Printf("[DRONE %s] ❌ Erro ao enviar laudo: %v", droneID, err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
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
