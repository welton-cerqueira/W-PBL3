#!/bin/bash

set -e  # interrompe o script em caso de erro

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

BIN_DIR="./bin"
LOG_DIR="./logs"
mkdir -p $BIN_DIR $LOG_DIR

# Compila os binários se não existirem
compile_if_needed() {
    if [ ! -f "$BIN_DIR/broker" ]; then
        echo -e "${YELLOW}Compilando broker...${NC}"
        go build -o $BIN_DIR/broker cmd/broker/main.go
    fi
    if [ ! -f "$BIN_DIR/drone" ]; then
        echo -e "${YELLOW}Compilando drone...${NC}"
        go build -o $BIN_DIR/drone cmd/drone-simulator/main.go
    fi
    if [ ! -f "$BIN_DIR/company" ]; then
        echo -e "${YELLOW}Compilando company...${NC}"
        go build -o $BIN_DIR/company cmd/company/main.go
    fi
}

# Mata processos antigos
kill_old() {
    echo -e "${YELLOW}Parando processos antigos...${NC}"
    pkill -f "$BIN_DIR/broker" 2>/dev/null || true
    pkill -f "$BIN_DIR/drone" 2>/dev/null || true
    pkill -f "$BIN_DIR/company" 2>/dev/null || true
    sleep 2
}

# Inicia um broker e aguarda ficar saudável
start_broker() {
    local id=$1
    local porta=$2
    local lider=$3
    local log_file="$LOG_DIR/broker_${id}.log"

    echo -e "${GREEN}Iniciando broker $id (porta $porta, líder=$lider)...${NC}"
    if [ "$lider" == "true" ]; then
        $BIN_DIR/broker -id=$id -porta=$porta -lider=true > $log_file 2>&1 &
    else
        $BIN_DIR/broker -id=$id -porta=$porta -lider=false > $log_file 2>&1 &
    fi
    local pid=$!
    echo $pid >> $LOG_DIR/.pids

    # Aguarda o broker responder
    for i in {1..10}; do
        if curl -s "http://localhost:$porta/health" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ Broker $id pronto${NC}"
            return 0
        fi
        sleep 1
    done
    echo -e "${RED}❌ Falha ao iniciar broker $id${NC}"
    return 1
}

# Inicia um drone
start_drone() {
    local id=$1
    local porta=$2
    local log_file="$LOG_DIR/drone_${id}.log"

    echo -e "${GREEN}Iniciando drone $id (porta $porta)...${NC}"
    $BIN_DIR/drone -id=$id -broker=http://localhost:8080 -porta=$porta > $log_file 2>&1 &
    echo $! >> $LOG_DIR/.pids
}

# Inicia uma companhia
start_company() {
    local id=$1
    local log_file="$LOG_DIR/company_${id}.log"

    echo -e "${GREEN}Iniciando companhia $id...${NC}"
    $BIN_DIR/company -id=$id -broker=http://localhost:8080 > $log_file 2>&1 &
    echo $! >> $LOG_DIR/.pids
}

# Main
main() {
    compile_if_needed
    kill_old

    echo -e "${YELLOW}Iniciando sistema distribuído...${NC}"

    # Iniciar brokers (1 líder + 3 seguidores)
    start_broker "broker1" 8080 true
    start_broker "broker2" 8081 false
    start_broker "broker3" 8082 false
    start_broker "broker4" 8083 false

    # Iniciar 8 drones (portas 9001 a 9008)
    for i in {1..8}; do
        start_drone "drone$i" $((9000 + i))
        sleep 0.5
    done

    # Iniciar companhias
    start_company "COMP-A"
    start_company "COMP-B"

    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}✅ Sistema iniciado com sucesso!${NC}"
    echo -e "${GREEN}Broker líder: http://localhost:8080${NC}"
    echo -e "${GREEN}Logs em: $LOG_DIR/${NC}"
    echo -e "${YELLOW}Para monitorar: tail -f $LOG_DIR/broker_broker1.log${NC}"
    echo -e "${YELLOW}Para parar: ./stop_system.sh${NC}"
    echo -e "${GREEN}========================================${NC}"
}

main "$@"