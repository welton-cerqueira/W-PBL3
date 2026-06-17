package modelos

import (
	"time"

	"github.com/google/uuid"
)

// Companhia representa uma empresa de navegação que pode requisitar drones
type Companhia struct {
	ID    string `json:"id"`
	Nome  string `json:"nome"`
	Saldo int    `json:"saldo"`
}

// NovaCompanhia cria uma nova instância de companhia
func NovaCompanhia(id, nome string, saldoInicial int) *Companhia {
	return &Companhia{
		ID:    id,
		Nome:  nome,
		Saldo: saldoInicial,
	}
}

// RequisicaoEscolta representa o pedido de uma companhia para usar um drone
type RequisicaoEscolta struct {
	IDRequisicao string `json:"id_requisicao"`
	CompanhiaID  string `json:"companhia_id"`
	Rota         string `json:"rota"`
	Timestamp    int64  `json:"timestamp"`
	Status       string `json:"status"`
	DroneID      string `json:"drone_id,omitempty"`
}

// Status possíveis para uma requisição
const (
	StatusPendente  = "pendente"
	StatusAprovada  = "aprovada"
	StatusNegada    = "negada"
	StatusConcluida = "concluida"
)

// NovaRequisicaoEscolta cria uma nova requisição com ID automático
func NovaRequisicaoEscolta(companhiaID, rota string) *RequisicaoEscolta {
	return &RequisicaoEscolta{
		IDRequisicao: uuid.New().String(),
		CompanhiaID:  companhiaID,
		Rota:         rota,
		Timestamp:    time.Now().Unix(),
		Status:       StatusPendente,
	}
}
