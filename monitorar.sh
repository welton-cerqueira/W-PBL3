#!/bin/bash

echo "=== MONITORAMENTO DO SISTEMA DISTRIBUÍDO ==="

while true; do
    clear
    echo "=== W-PBL3 - Sistema Distribuído ==="
    echo "Data/Hora: $(date)"
    echo ""
    
    # Verificar brokers
    echo "📡 BROKERS:"
    for i in 1 2 3 4; do
        STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:808$((i-1))/health 2>/dev/null)
        if [ "$STATUS" = "200" ]; then
            echo "  ✅ broker$i: online"
        else
            echo "  ❌ broker$i: offline"
        fi
    done
    
    echo ""
    echo "💰 SALDOS:"
    curl -s http://localhost:8080/saldo/COMP-A | jq -r '"  COMP-A: \(.saldo) créditos"'
    curl -s http://localhost:8080/saldo/COMP-B | jq -r '"  COMP-B: \(.saldo) créditos"'
    
    echo ""
    echo "📊 ESTATÍSTICAS:"
    curl -s http://localhost:8080/estatisticas | jq -r '"  Total Laudos: \(.total_laudos)"'
    
    echo ""
    echo "🔗 CADEIA DE LAUDOS:"
    curl -s http://localhost:8080/verificar-cadeia | jq -r '"  \(.mensagem)"'
    
    echo ""
    echo "Pressione Ctrl+C para sair"
    sleep 5
done