# W-PBL3 – Sistema Distribuído de Escolta Marítima com Raft, Ledger Imutável e Assinaturas Digitais

[![Go Report Card](https://goreportcard.com/badge/github.com/welton-cerqueira/W-PBL3)](https://goreportcard.com/report/github.com/welton-cerqueira/W-PBL3)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 📌 Visão Geral

O **W-PBL3** é um sistema distribuído para coordenação de uma frota de drones autônomos de monitoramento marítimo. Ele foi desenvolvido como solução para o **Problema 3** da disciplina de Concorrência e Conectividade, integrando:

- **Cluster Raft TCP** – eleição de líder, replicação de log e snapshots.
- **Ledger Imutável** – cada transação (recarga, pagamento, laudo) é registrada em um histórico à prova de adulteração.
- **Cadeia de Hashes** – laudos encadeados criptograficamente (hash anterior → hash atual).
- **Assinaturas Digitais (ECDSA)** – companhias assinam requisições de pagamento; drones assinam laudos.
- **Auditoria Transparente** – qualquer membro do consórcio pode consultar saldos, histórico e verificar a cadeia de laudos.
- **Alta Disponibilidade** – falha do líder não interrompe o sistema; novo líder é eleito automaticamente.

---

## 🧱 Estrutura do Projeto

```bash
W-PBL3/
├── cmd/
│ ├── broker/ # Ponto de entrada do broker (nó Raft)
│ ├── company/ # Simulador de companhia (cliente autônomo)
│ └── drone-simulator/ # Simulador de drone (servidor HTTP + missões)
├── internal/
│ ├── api/ # Handlers HTTP (rotas, servidor)
│ ├── consenso/ # Implementação do Raft TCP (tcp_raft.go)
│ ├── crypto/ # Assinaturas digitais (ECDSA)
│ └── drone/ # Gerenciamento de drones (pool)
├── pkg/
│ └── modelos/ # Estruturas de dados (companhia, requisição, laudo)
├── docker-compose.yml # Orquestração para ambiente distribuído
├── Dockerfile.broker
├── Dockerfile.drone
├── Dockerfile.company
├── go.mod
└── README.md
```

---

## ⚙️ Pré‑requisitos

- **Go 1.21+** (para compilação manual)
- **Docker** e **Docker Compose** (para execução containerizada)
- Acesso de rede entre as máquinas (para execução distribuída)
- Portas utilizadas:
  - Brokers: `8080` (API), `7002` (Raft)
  - Drones: `9001`–`9008`
  - Companhias: nenhuma porta exposta (apenas clientes)

---

# 🚀 Primeiros passos para compilar o projeto

# Execução sem Docker Compose (RECOMENDADO) - Exemplo de execução no Lab Larsid:

### 0. Clone o projeto em todas as máquinas

```bash
git clone https://github.com/welton-cerqueira/W-PBL3.git
cd W-PBL3
```

### 1. Compilar os binários

```bash
go build -o bin/broker cmd/broker/main.go
go build -o bin/drone cmd/drone-simulator/main.go
go build -o bin/company cmd/company/main.go
```
### 2. Em cada máquina, dê permissão de execução

```bash
chmod +x ~/W-PBL3/bin/*
```
## 3. Subir brokers

### Subir broker 1 (172.16.103.1)

```bash
./bin/broker -id=broker1 -api-addr=172.16.103.1:8080 -raft-addr=172.16.103.1:7002 -lider=true -outros="broker2=172.16.103.2:7002,broker3=172.16.103.3:7002,broker4=172.16.103.4:7002"
```

### Subir broker 2 (172.16.103.2)

```bash
./bin/broker -id=broker2 -api-addr=172.16.103.2:8080 -raft-addr=172.16.103.2:7002 -lider=false -outros="broker1=172.16.103.1:7002,broker3=172.16.103.3:7002,broker4=172.16.103.4:7002"
```

### Subir broker 3 (172.16.103.3)

```bash
./bin/broker -id=broker3 -api-addr=172.16.103.3:8080 -raft-addr=172.16.103.3:7002 -lider=false -outros="broker1=172.16.103.1:7002,broker2=172.16.103.2:7002,broker4=172.16.103.4:7002"
```

### Subir broker 4 (172.16.103.4)

```bash
./bin/broker -id=broker4 -api-addr=172.16.103.4:8080 -raft-addr=172.16.103.4:7002 -lider=false -outros="broker1=172.16.103.1:7002,broker2=172.16.103.2:7002,broker3=172.16.103.3:7002"
```

## 4. Iniciar drones (172.16.103.5) - Pode subir cada drone de forma distribuida

```bash
for i in {1..8}; do
    ./bin/drone -id=drone$i -addr=172.16.103.5:900$i -brokers="broker1=172.16.103.1:8080,broker2=172.16.103.2:8080,broker3=172.16.103.3:8080" &
done
```

## 5. Iniciar Campanhias - Pode rodar em qualquer máquina e de forma distribuida
```bash
./bin/company -id=COMP-A -brokers="broker1=172.16.103.1:8080,broker2=172.16.103.2:8080,broker3=172.16.103.3:8080" &
./bin/company -id=COMP-B -brokers="broker1=172.16.103.1:8080,broker2=172.16.103.2:8080,broker3=172.16.103.3:8080" &
```

## 6. Matar processos em máquinas separadas
```bash
pkill -f "bin/broker"
pkill -f "bin/drone"
pkill -f "bin/company"
```

# 🔗 Verificação da Cadeia (Blockchain)

## Comando
```bash
curl -s http://<ENDERECO_DO_BROKER>:8080/verificar-cadeia | jq .
```
Substitua <ENDERECO_DO_BROKER> pelo IP de qualquer broker ativo (ex.: 172.16.103.1).

## Exemplo de saída
```bash
json
{
  "cadeia_integra": true,
  "mensagem": "✅ Cadeia de 5 laudos íntegra (encadeamento verificado)",
  "total_laudos": 5
}
```

# Visualizar a cadeia completa

## Para ver todos os laudos e seus hashes encadeados (útil para auditoria):

```bash
curl -s http://<ENDERECO_DO_BROKER>:8080/historico | jq '.historico[] | select(.tipo == "laudo") | {id: .dados.id_requisicao, hash_anterior: .dados.hash_anterior, hash: .dados.hash}'
```
## Exemplo de saída (cadeia de um drone)
```bash
json
{
  "id": "e40066e8-8549-4787-929d-739b204f870c",
  "hash_anterior": "",
  "hash": "0e4b1473ede4f811c5e078594534e36231faad8cb31e314fa23bbb92aee8fb74"
}
{
  "id": "131bfe7e-829c-4003-8efe-9c1a37a5738f",
  "hash_anterior": "0e4b1473ede4f811c5e078594534e36231faad8cb31e314fa23bbb92aee8fb74",
  "hash": "a0ee727bf5edc7e783fed30e66961e2e714f06f09a81d9e7d8cc3cf145aca592"
}
```

# 🐳 Execução com Docker Compose

### Ambiente único (todos os containers na mesma máquina) Construir as imagens

```bash
docker build -f Dockerfile.broker -t w-pbl3-broker .
docker build -f Dockerfile.drone -t w-pbl3-drone .
docker build -f Dockerfile.company -t w-pbl3-company .
```
### Em cada máquina, dê permissão de execução

```bash
chmod +x ~/W-PBL3/bin/*
```

```bash
docker-compose up -d
Verificar logs
```
```bash
docker-compose logs -f broker1
Parar e remover
```
```bash
docker-compose down -v
```

## 🔐 Assinaturas Digitais (ECDSA)

Cada companhia gera um par de chaves ao iniciar.
A requisição /requisitar-drone é assinada com a chave privada; o broker verifica a assinatura antes de processar o pagamento.

Cada drone também gera um par de chaves.
O laudo da missão é assinado; o broker rejeita laudos não assinados ou com assinatura inválida.

As assinaturas são armazenadas no ledger (DadosPagamento.Assinatura / DadosLaudo.Assinatura), garantindo não repúdio e rastreabilidade.


## 📊 Monitoramento e Auditoria
### Endpoint	Descrição
```bash
GET /health	Status do broker (online, líder ou seguidor).
GET /saldo/{COMPANY_ID}	Saldo atual da companhia (derivado do ledger).
GET /historico	Lista todas as transações (recargas, pagamentos, laudos).
GET /verificar-cadeia	Verifica o encadeamento dos hashes dos laudos (integridade da cadeia).
GET /estatisticas	Total de laudos, sucessos, falhas e taxa de sucesso.
POST /recarregar	Recarrega créditos de uma companhia (apenas líder).
POST /requisitar-drone	Companhia solicita um drone (pagamento incluso).
POST /drone/registrar	Drone se registra (endereço completo).
POST /drone/relatar-missao	Drone envia laudo assinado.
```

#3 🧪 Teste de Resiliência (falha do líder)
Identifique o líder atual:

```bash
curl http://172.16.103.1:8080/leader
Mate o processo do broker líder (Ctrl+C ou kill).
```
Aguarde 5–10 segundos. Os brokers restantes elegerão um novo líder.

### Consulte novamente o líder:

```bash
curl http://172.16.103.2:8080/leader
```
As companhias e drones redescobrirão o novo líder automaticamente (através do endpoint /leader dos peers) e continuarão operando.

## 🛠️ Solução de Problemas Comuns
Erros comuns, causa provável	e Solução

```bash
bind: address already in use	Porta já ocupada	Altere a porta no comando ou mate o processo antigo.
cannot assign requested address	IP informado não pertence à máquina	Use hostname -I para descobrir o IP correto.
connection refused ao replicar transação	Broker destino ainda não iniciou	Inicie os brokers na ordem (líder primeiro).
saldo insuficiente mesmo após recarga	Recarga aplicada em nó que não é o líder	Envie recargas sempre para o líder.
Assinatura inválida	Chave pública/privada inconsistente	Recompile os binários (as chaves são geradas a cada execução).
```

# 📄 Licença
Este projeto está licenciado sob a MIT License – sinta-se livre para usar, modificar e distribuir.

## 👥 Autores
Welton Cerqueira – arquitetura e implementação

## 🙏 Agradecimentos
Professores da disciplina TEC502 – Concorrência e Conectividade

Comunidade open‑source pelas bibliotecas utilizadas (Fiber, UUID, etc.)