package modelos

import (
	"time"

	"github.com/google/uuid"
)

// Pagamento representa uma transação de créditos entre companhias ou para o sistema
type Pagamento struct {
	ID              string `json:"id"`
	DeCompanhiaID   string `json:"de_companhia_id"`
	ParaCompanhiaID string `json:"para_companhia_id"`
	Valor           int    `json:"valor"`
	Motivo          string `json:"motivo"` // "requisicao_drone", "recarga", etc.
	Timestamp       int64  `json:"timestamp"`
}

// NovoPagamento cria um novo registro de pagamento
func NovoPagamento(deCompanhiaID, paraCompanhiaID string, valor int, motivo string) *Pagamento {
	return &Pagamento{
		ID:              uuid.New().String(),
		DeCompanhiaID:   deCompanhiaID,
		ParaCompanhiaID: paraCompanhiaID,
		Valor:           valor,
		Motivo:          motivo,
		Timestamp:       time.Now().Unix(),
	}
}

// RecargaCreditos representa uma operação de adicionar créditos a uma companhia (administrativa)
type RecargaCredito struct {
	CompanhiaID   string `json:"companhia_id"`
	Valor         int    `json:"valor"`
	AutorizadoPor string `json:"autorizado_por"` // "sistema" ou "admin"
	Timestamp     int64  `json:"timestamp"`
	ID            string `json:"id"`
}

// NovaRecargaCredito cria uma nova operação de recarga
func NovaRecargaCredito(companhiaID string, valor int, autorizadoPor string) *RecargaCredito {
	return &RecargaCredito{
		ID:            uuid.New().String(),
		CompanhiaID:   companhiaID,
		Valor:         valor,
		AutorizadoPor: autorizadoPor,
		Timestamp:     time.Now().Unix(),
	}
}
