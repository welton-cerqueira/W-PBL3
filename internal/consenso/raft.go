package consenso

import (
	"fmt"
	"log"
	"sync"
)

// NoRaft representa um nó participante do consenso
// Esta é uma implementação SIMPLIFICADA para demonstração
// Em produção, usaríamos hashicorp/raft
type NoRaft struct {
	id        string
	ehLider   bool
	liderID   string
	estado    *EstadoLedger
	outrosNos []string // Endereços dos outros nós (ex: "localhost:8081")
	mu        sync.RWMutex
}

// NovoNoRaft cria uma nova instância do nó Raft
func NovoNoRaft(id string, estado *EstadoLedger, outrosNos []string) *NoRaft {
	return &NoRaft{
		id:        id,
		ehLider:   false,
		liderID:   "",
		estado:    estado,
		outrosNos: outrosNos,
	}
}

// EhLider retorna se este nó é o líder atual
func (n *NoRaft) EhLider() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.ehLider
}

// ObterLiderID retorna o ID do nó líder atual
func (n *NoRaft) ObterLiderID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.liderID == "" {
		return "desconhecido"
	}
	return n.liderID
}

// AplicarTransacao aplica uma transação ao cluster Raft
// Se for o líder, aplica localmente e replica
// Se não for o líder, encaminha para o líder
func (n *NoRaft) AplicarTransacao(transacao *Transacao) error {
	n.mu.RLock()
	ehLider := n.ehLider
	liderID := n.liderID
	n.mu.RUnlock()

	if !ehLider {
		return fmt.Errorf("este nó não é o líder. O líder é: %s", liderID)
	}

	// Aplica localmente
	if err := n.estado.AplicarTransacao(transacao); err != nil {
		return err
	}

	// Replica para outros nós (simplificado - em produção seria via Raft)
	log.Printf("[RAFT] Líder %s aplicou transação %s e replicaria para %d nós",
		n.id, transacao.ID, len(n.outrosNos))

	return nil
}

// TornarLider força este nó a se tornar líder (para simulação)
func (n *NoRaft) TornarLider() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ehLider = true
	n.liderID = n.id
	log.Printf("[RAFT] Nó %s tornou-se LÍDER", n.id)
}

// TornarSeguidor torna este nó um seguidor
func (n *NoRaft) TornarSeguidor(liderID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ehLider = false
	n.liderID = liderID
	log.Printf("[RAFT] Nó %s agora é SEGUIDOR do líder %s", n.id, liderID)
}
