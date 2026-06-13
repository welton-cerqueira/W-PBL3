#!/bin/bash

echo "=== INICIANDO SISTEMA LOCALMENTE ==="

# Matar processos anteriores
pkill -f "broker/main.go" 2>/dev/null
pkill -f "drone-simulator" 2>/dev/null
pkill -f "company/main.go" 2>/dev/null

# Iniciar broker líder
echo "Iniciando broker líder..."
go run cmd/broker/main.go -id=broker1 -porta=8080 -lider=true &
BROKER_PID=$!
sleep 3

# Verificar se broker está rodando
if curl -s http://localhost:8080/health > /dev/null; then
    echo "✅ Broker líder rodando na porta 8080"
else
    echo "❌ Falha ao iniciar broker"
    exit 1
fi

# Iniciar 4 drones (para teste local)
echo -e "\nIniciando drones..."
for i in {1..4}; do
    go run cmd/drone-simulator/main.go -id=drone$i -broker=http://localhost:8080 &
    echo "  ✅ Drone $i iniciado"
    sleep 1
done

# Iniciar companhias
echo -e "\nIniciando companhias..."
go run cmd/company/main.go -id=COMP-A -broker=http://localhost:8080 &
go run cmd/company/main.go -id=COMP-B -broker=http://localhost:8080 &

echo -e "\n=== SISTEMA INICIADO ==="
echo "Broker: http://localhost:8080"
echo "Drones: 4 drones disponíveis"
echo "Companhias: COMP-A e COMP-B (requisições automáticas a cada 10-30s)"
echo ""
echo "Pressione Ctrl+C para parar"
echo ""

# Aguardar sinais
wait