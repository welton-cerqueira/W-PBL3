#!/bin/bash

echo "=== TESTE SEGURO W-PBL3 ==="

# 1. Registrar drone
echo -e "\n1. Registrando drone..."
curl -s -X POST http://localhost:8080/drone/registrar \
  -H "Content-Type: application/json" \
  -d '{"drone_id":"drone1","porta":"9001"}'
echo ""

# 2. Verificar saldo
echo -e "\n2. Saldo COMP-A:"
curl -s http://localhost:8080/saldo/COMP-A
echo ""

# 3. Primeira requisição
echo -e "\n3. Primeira requisição..."
RESP1=$(curl -s -X POST http://localhost:8080/requisitar-drone \
  -H "Content-Type: application/json" \
  -d '{"companhia_id":"COMP-A","rota":"Rota Norte"}')
echo "$RESP1"

REQ_ID1=$(echo "$RESP1" | grep -o '"id_requisicao":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "ID: $REQ_ID1"

# 4. Enviar laudo
echo -e "\n4. Enviando laudo..."
HASH1="hash_$(date +%s)_${REQ_ID1:0:8}"

curl -s -X POST http://localhost:8080/drone/relatar-missao \
  -H "Content-Type: application/json" \
  -d "{
    \"id_requisicao\": \"$REQ_ID1\",
    \"drone_id\": \"drone1\",
    \"rota\": \"Rota Norte\",
    \"resultado\": \"sucesso\",
    \"obstaculos\": [\"Teste\"],
    \"incidentes\": [],
    \"data_hora_inicio\": $(date +%s),
    \"data_hora_fim\": $(($(date +%s) + 5)),
    \"hash_anterior\": \"\",
    \"hash_verificacao\": \"$HASH1\"
  }"
echo ""

# 5. Segunda requisição
echo -e "\n5. Segunda requisição..."
RESP2=$(curl -s -X POST http://localhost:8080/requisitar-drone \
  -H "Content-Type: application/json" \
  -d '{"companhia_id":"COMP-A","rota":"Rota Sul"}')
echo "$RESP2"

# 6. Verificar saldo final
echo -e "\n6. Saldo final COMP-A:"
curl -s http://localhost:8080/saldo/COMP-A
echo ""

# 7. Verificar cadeia
echo -e "\n7. Verificando cadeia de laudos:"
curl -s http://localhost:8080/verificar-cadeia
echo ""

echo -e "\n=== FIM ==="