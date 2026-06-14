package drone

import (
	"fmt"
	"log"
	"sync"
)

// GerenciadorDrones gerencia o estado de todos os drones (disponíveis/ocupados)
type GerenciadorDrones struct {
	mu            sync.RWMutex
	drones        map[string]*DroneInfo // ID do drone -> informações
	missoesAtivas map[string]string     // ID da requisição -> ID do drone
}

// DroneInfo contém as informações de um drone
type DroneInfo struct {
	ID          string `json:"id"`
	Addr        string `json:"addr"`         // Endereço completo (ex: "192.168.1.20:9001")
	Status      string `json:"status"`       // "disponivel", "ocupado", "offline"
	MissaoAtual string `json:"missao_atual"` // ID da requisição (se ocupado)
}

// NovoGerenciadorDrones cria um novo gerenciador
func NovoGerenciadorDrones() *GerenciadorDrones {
	return &GerenciadorDrones{
		drones:        make(map[string]*DroneInfo),
		missoesAtivas: make(map[string]string),
	}
}

// RegistrarDrone adiciona um drone ao pool (endereço completo)
func (g *GerenciadorDrones) RegistrarDrone(droneID, addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.drones[droneID] = &DroneInfo{
		ID:          droneID,
		Addr:        addr,
		Status:      "disponivel",
		MissaoAtual: "",
	}
	log.Printf("[DRONE] Drone %s registrado em %s", droneID, addr)
}

// AlocarDrone busca um drone disponível e o aloca para uma missão
func (g *GerenciadorDrones) AlocarDrone(idRequisicao, rota string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Procura um drone disponível
	for id, drone := range g.drones {
		if drone.Status == "disponivel" {
			// Aloca o drone
			drone.Status = "ocupado"
			drone.MissaoAtual = idRequisicao
			g.missoesAtivas[idRequisicao] = id

			log.Printf("[DRONE] Drone %s alocado para missão %s (rota: %s)", id, idRequisicao, rota)
			return id, nil
		}
	}

	return "", fmt.Errorf("nenhum drone disponível")
}

// LiberarDrone libera um drone após a conclusão da missão
func (g *GerenciadorDrones) LiberarDrone(droneID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if drone, existe := g.drones[droneID]; existe {
		// Remove da lista de missões ativas
		if drone.MissaoAtual != "" {
			delete(g.missoesAtivas, drone.MissaoAtual)
		}

		drone.Status = "disponivel"
		drone.MissaoAtual = ""
		log.Printf("[DRONE] Drone %s liberado e disponível novamente", droneID)
	}
}

// ObterDroneStatus retorna o status de um drone específico
func (g *GerenciadorDrones) ObterDroneStatus(droneID string) (string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if drone, existe := g.drones[droneID]; existe {
		return drone.Status, nil
	}
	return "", fmt.Errorf("drone não encontrado")
}

// ListarDrones retorna a lista de todos os drones com seus status
func (g *GerenciadorDrones) ListarDrones() map[string]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	resultado := make(map[string]string)
	for id, drone := range g.drones {
		resultado[id] = drone.Status
	}
	return resultado
}

// ListarMissoesAtivas retorna todas as missões ativas no momento
func (g *GerenciadorDrones) ListarMissoesAtivas() map[string]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	missoes := make(map[string]string)
	for idReq, droneID := range g.missoesAtivas {
		missoes[idReq] = droneID
	}
	return missoes
}

// ObterEnderecoDrone retorna o endereço completo de um drone (para notificação)
func (g *GerenciadorDrones) ObterEnderecoDrone(droneID string) (string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if drone, existe := g.drones[droneID]; existe {
		return drone.Addr, nil
	}
	return "", fmt.Errorf("drone não encontrado")
}
