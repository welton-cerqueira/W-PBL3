#!/bin/bash

echo "=== Iniciando cluster com 4 brokers, 8 drones e 2 companhias ==="

# Compila (se necessário)
go build -o bin/broker cmd/broker/main.go
go build -o bin/drone cmd/drone-simulator/main.go
go build -o bin/company cmd/company/main.go

# Mata processos antigos
pkill -f "bin/broker"
pkill -f "bin/drone"
pkill -f "bin/company"
sleep 2

# Brokers (um por linha, em background)
./bin/broker -id=broker1 -porta=8080 -lider=true -outros=broker2,broker3,broker4 &
sleep 2
./bin/broker -id=broker2 -porta=8081 -lider=false -outros=broker1,broker3,broker4 &
./bin/broker -id=broker3 -porta=8082 -lider=false -outros=broker1,broker2,broker4 &
./bin/broker -id=broker4 -porta=8083 -lider=false -outros=broker1,broker2,broker3 &

# Drones (8)
for i in {1..8}; do
    ./bin/drone -id=drone$i -broker=http://localhost:8080 -porta=$((9000+i)) &
    sleep 0.5
done

# Companhias
./bin/company -id=COMP-A -broker=http://localhost:8080 &
./bin/company -id=COMP-B -broker=http://localhost:8080 &

echo "Sistema iniciado. Pressione Ctrl+C para parar."
wait