package modelos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LaudoMissao representa o relatório gerado por um drone após completar uma missão
type LaudoMissao struct {
	ID              string   `json:"id"`
	IDRequisicao    string   `json:"id_requisicao"`
	DroneID         string   `json:"drone_id"`
	Rota            string   `json:"rota"`
	Resultado       string   `json:"resultado"`
	Obstaculos      []string `json:"obstaculos"`
	Incidentes      []string `json:"incidentes"`
	DataHoraInicio  int64    `json:"data_hora_inicio"`
	DataHoraFim     int64    `json:"data_hora_fim"`
	HashAnterior    string   `json:"hash_anterior"`    // Hash do laudo anterior (encadeamento)
	HashVerificacao string   `json:"hash_verificacao"` // Hash deste laudo
}

// NovoLaudoMissao cria um novo laudo de missão
func NovoLaudoMissao(idRequisicao, droneID, rota, resultado string) *LaudoMissao {
	return &LaudoMissao{
		ID:             uuid.New().String(),
		IDRequisicao:   idRequisicao,
		DroneID:        droneID,
		Rota:           rota,
		Resultado:      resultado,
		Obstaculos:     []string{},
		Incidentes:     []string{},
		DataHoraInicio: time.Now().Unix(),
		HashAnterior:   "", // Será preenchido depois
	}
}

// AdicionarObstaculo adiciona um obstáculo detectado ao laudo
func (l *LaudoMissao) AdicionarObstaculo(obstaculo string) {
	l.Obstaculos = append(l.Obstaculos, obstaculo)
}

// AdicionarIncidente adiciona um incidente ao laudo
func (l *LaudoMissao) AdicionarIncidente(incidente string) {
	l.Incidentes = append(l.Incidentes, incidente)
}

// FinalizarLaudo finaliza o laudo com o timestamp de término e calcula o hash
func (l *LaudoMissao) FinalizarLaudo() {
	l.DataHoraFim = time.Now().Unix()
	l.CalcularHash()
}

// CalcularHash calcula o hash SHA256 do conteúdo do laudo
func (l *LaudoMissao) CalcularHash() {
	// Cria um mapa com os dados que serão usados para o hash
	dadosParaHash := map[string]interface{}{
		"id":               l.ID,
		"id_requisicao":    l.IDRequisicao,
		"drone_id":         l.DroneID,
		"rota":             l.Rota,
		"resultado":        l.Resultado,
		"obstaculos":       l.Obstaculos,
		"incidentes":       l.Incidentes,
		"data_hora_inicio": l.DataHoraInicio,
		"data_hora_fim":    l.DataHoraFim,
		"hash_anterior":    l.HashAnterior,
	}

	// Converte para JSON
	jsonBytes, err := json.Marshal(dadosParaHash)
	if err != nil {
		l.HashVerificacao = "erro_ao_calcular_hash"
		return
	}

	// Calcula o hash SHA256
	hash := sha256.Sum256(jsonBytes)
	l.HashVerificacao = hex.EncodeToString(hash[:])
}

// VerificarIntegridade verifica se o hash do laudo é válido
func (l *LaudoMissao) VerificarIntegridade() bool {
	// Salva o hash atual
	hashAtual := l.HashVerificacao

	// Recalcula o hash
	l.CalcularHash()

	// Compara com o hash armazenado
	return hashAtual == l.HashVerificacao
}

// VincularHashAnterior vincula este laudo ao anterior (encadeamento)
func (l *LaudoMissao) VincularHashAnterior(hashAnterior string) {
	l.HashAnterior = hashAnterior
	l.CalcularHash() // Recalcula o hash com o novo hash anterior
}

// Resultados possíveis para uma missão
const (
	ResultadoSucesso = "sucesso"
	ResultadoFalha   = "falha"
	ResultadoParcial = "parcial"
)
