package drone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"W-PBL3/pkg/modelos"
)

// ClienteDrone é responsável por se comunicar com os drones via HTTP
type ClienteDrone struct {
	client *http.Client
}

// NovoClienteDrone cria um novo cliente para comunicação com drones
func NovoClienteDrone() *ClienteDrone {
	return &ClienteDrone{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DespacharDrone envia um comando para o drone iniciar uma missão
func (c *ClienteDrone) DespacharDrone(droneID, porta, idRequisicao, rota string) error {
	// Prepara a requisição para o drone
	requisicao := map[string]interface{}{
		"id_requisicao": idRequisicao,
		"rota":          rota,
		"drone_id":      droneID,
	}

	jsonData, err := json.Marshal(requisicao)
	if err != nil {
		return fmt.Errorf("erro ao serializar requisição: %v", err)
	}

	// Envia para o drone
	url := fmt.Sprintf("http://localhost:%s/iniciar-missao", porta)
	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("erro ao comunicar com drone %s: %v", droneID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("drone %s retornou erro: %d", droneID, resp.StatusCode)
	}

	return nil
}

// EnviarLaudo envia o laudo da missão para o drone (simulado)
// Nota: Na implementação real, o drone que envia o laudo, não o broker
// Este método é apenas para referência
func (c *ClienteDrone) EnviarLaudo(droneID, porta string, laudo *modelos.LaudoMissao) error {
	jsonData, err := json.Marshal(laudo)
	if err != nil {
		return fmt.Errorf("erro ao serializar laudo: %v", err)
	}

	url := fmt.Sprintf("http://localhost:%s/relatar-missao", porta)
	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("erro ao enviar laudo: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("falha ao relatar missão: %d", resp.StatusCode)
	}

	return nil
}
