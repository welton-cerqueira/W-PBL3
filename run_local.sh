#!/bin/bash

echo "=== INICIANDO SISTEMA COMPLETO W-PBL3 ==="

# Matar processos antigos
echo "1. Parando processos antigos..."
pkill -f "bin/broker" 2>/dev/null
pkill -f "bin/drone" 2>/dev/null
pkill -f "bin/company" 2>/dev/null
sleep 2

# Iniciar broker
echo "2. Iniciando broker..."
./bin/broker -id=broker1 -porta=8080 -lider=true &
sleep 3

# Verificar broker
if curl -s http://localhost:8080/health > /dev/null; then
    echo "   ✅ Broker rodando"
else
    echo "   ❌ Falha no broker"
    exit 1
fi

# Iniciar 8 drones
echo "3. Iniciando 8 drones..."
for i in {1..8}; do
    PORTA=$((9000 + i))
    ./bin/drone -id=drone$i -broker=http://localhost:8080 -porta=$PORTA &
    echo "   ✅ Drone $i (porta $PORTA)"
    sleep 1
done

# Iniciar companhias
echo "4. Iniciando companhias..."
./bin/company -id=COMP-A -broker=http://localhost:8080 &
./bin/company -id=COMP-B -broker=http://localhost:8080 &
echo "   ✅ COMP-A e COMP-B iniciadas"

echo ""
echo "=== SISTEMA COMPLETO INICIADO ==="
echo "Broker: http://localhost:8080"
echo "Drones: 8 drones disponíveis (portas 9001-9008)"
echo "Companhias: COMP-A e COMP-B (requisições automáticas)"
echo ""
echo "Monitorar: curl http://localhost:8080/estatisticas"
echo "Parar: pkill -f 'bin/(broker|drone|company)'"