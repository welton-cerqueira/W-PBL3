package modelos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// LaudoMissao representa o relatório gerado por um drone após completar uma missão
type LaudoMissao struct {
	ID              string   `json:"id"`
	IDRequisicao    string   `json:"id_requisicao"`
	DroneID         string   `json:"drone_id"`
	CompanhiaID     string   `json:"companhia_id"` // NOVO
	Rota            string   `json:"rota"`
	Resultado       string   `json:"resultado"`
	Obstaculos      []string `json:"obstaculos"`
	Incidentes      []string `json:"incidentes"`
	DataHoraInicio  int64    `json:"data_hora_inicio"`
	DataHoraFim     int64    `json:"data_hora_fim"`
	HashAnterior    string   `json:"hash_anterior"`
	HashVerificacao string   `json:"hash_verificacao"`
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

	// Criar JSON exatamente no mesmo formato do cliente (sem espaços, em uma linha)
	// A ordem dos campos precisa ser a mesma!
	jsonStr := fmt.Sprintf(`{"id_requisicao":"%s","drone_id":"%s","rota":"%s","resultado":"%s","obstaculos":%s,"incidentes":%s,"data_hora_inicio":%d,"data_hora_fim":%d,"hash_anterior":"%s"}`,
		l.IDRequisicao,
		l.DroneID,
		l.Rota,
		l.Resultado,
		obstaculosToJSON(l.Obstaculos),
		incidentesToJSON(l.Incidentes),
		l.DataHoraInicio,
		l.DataHoraFim,
		l.HashAnterior,
	)

	// Calcula o hash SHA256
	hash := sha256.Sum256([]byte(jsonStr))
	hashCalculado := hex.EncodeToString(hash[:])

	// Log para depuração
	log.Printf("[VERIFICACAO_HASH] String para hash: %s", jsonStr)
	log.Printf("[VERIFICACAO_HASH] Hash recebido: %s", hashAtual)
	log.Printf("[VERIFICACAO_HASH] Hash calculado: %s", hashCalculado)

	return hashCalculado == hashAtual
}

// Funções auxiliares para converter slices para JSON string
func obstaculosToJSON(obstaculos []string) string {
	if len(obstaculos) == 0 {
		return "[]"
	}
	result := "["
	for i, o := range obstaculos {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf(`"%s"`, o)
	}
	result += "]"
	return result
}

func incidentesToJSON(incidentes []string) string {
	if len(incidentes) == 0 {
		return "[]"
	}
	result := "["
	for i, inc := range incidentes {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf(`"%s"`, inc)
	}
	result += "]"
	return result
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
