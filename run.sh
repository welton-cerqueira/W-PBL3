#!/bin/bash

echo "=== INICIANDO SISTEMA DISTRIBUÍDO W-PBL3 ==="
echo ""

# Limpar containers antigos
echo "1. Limpando containers antigos..."
docker-compose down -v

# Build das imagens
echo -e "\n2. Build das imagens..."
docker-compose build

# Iniciar sistema
echo -e "\n3. Iniciando sistema distribuído..."
docker-compose up -d

echo -e "\n4. Aguardando inicialização dos componentes..."
sleep 10

echo -e "\n5. Verificando status dos componentes..."
docker-compose ps

echo -e "\n6. Monitorando logs (Ctrl+C para parar)..."
echo "   - Brokers: 4 nós"
echo "   - Drones: 8 drones"
echo "   - Companhias: 2 companhias autônomas"
echo ""

# Mostrar logs em tempo real
docker-compose logs -f --tail=50