#!/bin/bash

while true; do
    clear
    echo "=== MONITORAMENTO W-PBL3 ==="
    echo "Data: $(date)"
    echo ""
    
    echo "📊 ESTATÍSTICAS:"
    curl -s http://localhost:8080/estatisticas | jq .
    
    echo ""
    echo "💰 SALDOS:"
    echo -n "  COMP-A: "
    curl -s http://localhost:8080/saldo/COMP-A | jq -r '.saldo'
    echo -n "  COMP-B: "
    curl -s http://localhost:8080/saldo/COMP-B | jq -r '.saldo'
    
    echo ""
    echo "🔗 CADEIA:"
    curl -s http://localhost:8080/verificar-cadeia | jq -r '.mensagem'
    
    echo ""
    echo "🕒 Atualizando em 5 segundos... (Ctrl+C para sair)"
    sleep 5
done