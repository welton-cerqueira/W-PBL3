package consenso

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TipoTransacao define os tipos possíveis de transações no ledger
type TipoTransacao string

const (
	TipoPagamento TipoTransacao = "pagamento"
	TipoRecarga   TipoTransacao = "recarga"
	TipoLaudo     TipoTransacao = "laudo"
)

// Transacao representa uma entrada no ledger distribuído
// Esta é a estrutura que será replicada entre todos os nós do Raft
type Transacao struct {
	ID        string          `json:"id"`
	Tipo      TipoTransacao   `json:"tipo"`
	Timestamp int64           `json:"timestamp"`
	Dados     json.RawMessage `json:"dados"` // Dados específicos da transação
}

// NovaTransacao cria uma nova transação com ID único
func NovaTransacao(tipo TipoTransacao, dados interface{}) (*Transacao, error) {
	dadosJSON, err := json.Marshal(dados)
	if err != nil {
		return nil, err
	}

	return &Transacao{
		ID:        uuid.New().String(),
		Tipo:      tipo,
		Timestamp: time.Now().Unix(),
		Dados:     dadosJSON,
	}, nil
}

// DadosPagamento estrutura para transações de pagamento
type DadosPagamento struct {
	DeCompanhiaID   string `json:"de_companhia_id"`
	ParaCompanhiaID string `json:"para_companhia_id"`
	Valor           int    `json:"valor"`
	Motivo          string `json:"motivo"`
	IDRequisicao    string `json:"id_requisicao,omitempty"`
}

// DadosRecarga estrutura para transações de recarga de créditos
type DadosRecarga struct {
	CompanhiaID   string `json:"companhia_id"`
	Valor         int    `json:"valor"`
	AutorizadoPor string `json:"autorizado_por"`
}

// DadosLaudo estrutura para transações de laudo de missão
type DadosLaudo struct {
	IDRequisicao   string   `json:"id_requisicao"`
	DroneID        string   `json:"drone_id"`
	Rota           string   `json:"rota"`
	Resultado      string   `json:"resultado"`
	Obstaculos     []string `json:"obstaculos"`
	Incidentes     []string `json:"incidentes"`
	DataHoraInicio int64    `json:"data_hora_inicio"`
	DataHoraFim    int64    `json:"data_hora_fim"`
	Hash           string   `json:"hash"`          // Hash do laudo
	HashAnterior   string   `json:"hash_anterior"` // Hash do laudo anterior
}
