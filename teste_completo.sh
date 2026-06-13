#!/bin/bash

echo "=== TESTE DE IMUTABILIDADE COM HASH ==="

# Solicitar drone
echo "1. Solicitando drone..."
RESPONSE=$(curl -s -X POST http://localhost:8080/requisitar-drone \
  -H "Content-Type: application/json" \
  -d '{"companhia_id":"COMP-A","rota":"Rota Teste"}')
REQ_ID=$(echo $RESPONSE | jq -r '.id_requisicao')
echo "ID Requisição: $REQ_ID"

# Criar laudo com hash
echo -e "\n2. Criando laudo com hash..."
INICIO=$(date +%s)
sleep 2
FIM=$(date +%s)

# Criar laudo em arquivo temporário
cat > /tmp/laudo.json << EOF
{
  "id": "$(uuidgen)",
  "id_requisicao": "$REQ_ID",
  "drone_id": "drone1",
  "rota": "Rota Teste",
  "resultado": "sucesso",
  "obstaculos": ["Obstáculo 1", "Obstáculo 2"],
  "incidentes": ["Incidente A"],
  "data_hora_inicio": $INICIO,
  "data_hora_fim": $FIM,
  "hash_anterior": "",
  "hash_verificacao": ""
}
EOF

# Calcular hash usando Python (ou outro método)
HASH=$(cat /tmp/laudo.json | sha256sum | cut -d' ' -f1)
echo "Hash calculado: $HASH"

# Adicionar hash ao laudo
jq --arg hash "$HASH" '.hash_verificacao = $hash' /tmp/laudo.json > /tmp/laudo_com_hash.json

# Enviar laudo
echo -e "\n3. Enviando laudo com hash..."
curl -s -X POST http://localhost:8080/drone/relatar-missao \
  -H "Content-Type: application/json" \
  -d @/tmp/laudo_com_hash.json | jq .

# Verificar cadeia
echo -e "\n4. Verificando integridade da cadeia..."
curl -s http://localhost:8080/verificar-cadeia | jq .

echo -e "\n=== TESTE DE TENTATIVA DE ADULTERAÇÃO ==="

# Simular adulteração - modificar o laudo
echo -e "\n5. Tentando enviar laudo adulterado..."
jq '.obstaculos = ["Obstáculo FALSO"]' /tmp/laudo_com_hash.json > /tmp/laudo_adulterado.json

# Recalcular hash (não vai bater)
curl -s -X POST http://localhost:8080/drone/relatar-missao \
  -H "Content-Type: application/json" \
  -d @/tmp/laudo_adulterado.json | jq .

echo -e "\n=== FIM DO TESTE ==="