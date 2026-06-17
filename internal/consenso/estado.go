package consenso

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

// EstadoLedger representa o estado completo do sistema
// Este é o objeto que será replicado pelo Raft
type EstadoLedger struct {
	mu        sync.RWMutex
	Saldos    map[string]int    `json:"saldos"`    // ID da companhia -> Saldo da companhia
	Historico []Transacao       `json:"historico"` // Histórico imutável de todas as transações
	Missoes   map[string]string `json:"missoes"`   // ID da requisição -> status da missão
}

// NovoEstadoLedger cria um novo ledger com saldos iniciais
func NovoEstadoLedger() *EstadoLedger {
	estado := &EstadoLedger{
		Saldos:    make(map[string]int),
		Historico: []Transacao{},
		Missoes:   make(map[string]string),
	}

	return estado
}

// AplicarTransacao aplica uma transação ao estado (deve ser chamado pelo Raft)
func (e *EstadoLedger) AplicarTransacao(transacao *Transacao) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// pega a nova transação e a joga diretamente no final da lista do histórico
	e.Historico = append(e.Historico, *transacao)

	// Aplica a transação ao estado baseado no tipo
	switch transacao.Tipo {
	case TipoRecarga:
		var dados DadosRecarga
		if err := json.Unmarshal(transacao.Dados, &dados); err != nil {
			return err
		}

		log.Printf("[RECARGA] Antes: %s saldo = %d", dados.CompanhiaID, e.Saldos[dados.CompanhiaID])
		e.Saldos[dados.CompanhiaID] += dados.Valor
		log.Printf("[RECARGA] Depois: %s saldo = %d", dados.CompanhiaID, e.Saldos[dados.CompanhiaID])

	case TipoPagamento:
		var dados DadosPagamento
		if err := json.Unmarshal(transacao.Dados, &dados); err != nil {
			return err
		}
		// Verifica se tem saldo suficiente
		if e.Saldos[dados.DeCompanhiaID] < dados.Valor {
			return fmt.Errorf("saldo insuficiente: %s tem %d, precisa %d",
				dados.DeCompanhiaID, e.Saldos[dados.DeCompanhiaID], dados.Valor)
		}
		//Faz transferência do valor de uma companhia para a outra
		e.Saldos[dados.DeCompanhiaID] -= dados.Valor
		e.Saldos[dados.ParaCompanhiaID] += dados.Valor

		log.Printf("[PAGAMENTO] Pagamento feito por -> %s: %d para -> %s: %d",
			dados.DeCompanhiaID, e.Saldos[dados.DeCompanhiaID], dados.ParaCompanhiaID, e.Saldos[dados.ParaCompanhiaID])

		// Carimba o status da missão como "pago"
		if dados.IDRequisicao != "" {
			e.Missoes[dados.IDRequisicao] = "pago"
		}

	case TipoLaudo:
		var dados DadosLaudo
		if err := json.Unmarshal(transacao.Dados, &dados); err != nil {
			return err
		}
		//atualiza o carimbo da missão de "pago" para "concluida"
		e.Missoes[dados.IDRequisicao] = "concluida"
	}

	return nil
}

// ObterSaldo retorna o saldo de uma companhia
func (e *EstadoLedger) ObterSaldo(companhiaID string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Saldos[companhiaID]
}

// ObterHistorico retorna uma cópia do histórico
func (e *EstadoLedger) ObterHistorico() []Transacao {
	e.mu.RLock()
	defer e.mu.RUnlock()
	historico := make([]Transacao, len(e.Historico))
	copy(historico, e.Historico)
	return historico
}

// VerificarMissaoConcluida verifica se uma missão já foi concluída
func (e *EstadoLedger) VerificarMissaoConcluida(idRequisicao string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	status, existe := e.Missoes[idRequisicao]
	return existe && status == "concluida"
}

// VerificarCadeiaLaudos verifica a integridade da cadeia (apenas encadeamento)
func (e *EstadoLedger) VerificarCadeiaLaudos() (bool, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var hashAnterior string = ""
	laudosEncontrados := 0

	for i, transacao := range e.Historico {
		if transacao.Tipo == TipoLaudo {
			laudosEncontrados++
			var dados DadosLaudo
			if err := json.Unmarshal(transacao.Dados, &dados); err != nil {
				return false, fmt.Sprintf("Erro ao decodificar laudo %d: %v", i, err)
			}

			// Verifica apenas o encadeamento (hash_anterior)
			if laudosEncontrados > 1 {
				// Verificação segura: se hashAnterior estiver vazio e dados.HashAnterior também, ok
				if hashAnterior == "" && dados.HashAnterior == "" {
					// OK, ambos vazios
				} else if hashAnterior != dados.HashAnterior {
					// Função segura para mostrar hashes
					hashAntMostra := hashAnterior
					if len(hashAntMostra) > 16 {
						hashAntMostra = hashAntMostra[:16]
					}
					hashRecMostra := dados.HashAnterior
					if len(hashRecMostra) > 16 {
						hashRecMostra = hashRecMostra[:16]
					}
					return false, fmt.Sprintf("❌ Encadeamento quebrado no laudo %d: esperado %s..., recebido %s...",
						i, hashAntMostra, hashRecMostra)
				}
			}

			// Atualiza o hash anterior
			hashAnterior = dados.Hash
		}
	}

	if laudosEncontrados == 0 {
		return true, "📋 Nenhum laudo encontrado para verificar"
	}

	if laudosEncontrados == 1 {
		return true, fmt.Sprintf("✅ 1 laudo registrado (primeiro da cadeia)")
	}

	return true, fmt.Sprintf("✅ Cadeia de %d laudos íntegra (encadeamento verificado)", laudosEncontrados)
}

// verificarHashLaudo verifica se o hash de um laudo corresponde ao seu conteúdo
func verificarHashLaudo(dados *DadosLaudo) bool {
	// Reconstrói os dados para calcular o hash
	dadosParaHash := map[string]interface{}{
		"id_requisicao":    dados.IDRequisicao,
		"drone_id":         dados.DroneID,
		"rota":             dados.Rota,
		"resultado":        dados.Resultado,
		"obstaculos":       dados.Obstaculos,
		"incidentes":       dados.Incidentes,
		"data_hora_inicio": dados.DataHoraInicio,
		"data_hora_fim":    dados.DataHoraFim,
		"hash_anterior":    dados.HashAnterior,
	}

	// Converte para JSON
	jsonBytes, err := json.Marshal(dadosParaHash)
	if err != nil {
		log.Printf("[ERRO] Falha ao serializar dados para hash: %v", err)
		return false
	}

	// Calcula o hash SHA256
	hash := sha256.Sum256(jsonBytes)
	hashCalculado := hex.EncodeToString(hash[:])

	// Compara com o hash armazenado
	if hashCalculado != dados.Hash {
		log.Printf("[ALERTA] Hash inválido! Calculado: %s, Armazenado: %s",
			hashCalculado[:16], dados.Hash[:16])
		return false
	}

	return true
}

// ObterEstatisticasLaudos retorna estatísticas sobre os laudos no ledger
func (e *EstadoLedger) ObterEstatisticasLaudos() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalLaudos := 0
	sucessos := 0
	falhas := 0

	for _, transacao := range e.Historico {
		if transacao.Tipo == TipoLaudo {
			totalLaudos++
			var dados DadosLaudo
			if err := json.Unmarshal(transacao.Dados, &dados); err == nil {
				if dados.Resultado == "sucesso" {
					sucessos++
				} else if dados.Resultado == "falha" {
					falhas++
				}
			}
		}
	}

	return map[string]interface{}{
		"total_laudos": totalLaudos,
		"sucessos":     sucessos,
		"falhas":       falhas,
		"taxa_sucesso": float64(sucessos) / float64(max(totalLaudos, 1)) * 100,
	}
}

// Função auxiliar para evitar divisão por zero
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
