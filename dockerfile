FROM golang:1.21-alpine

WORKDIR /app

# Copiar go.mod e go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fonte
COPY . .

# Compilar
RUN go build -o broker cmd/broker/main.go

EXPOSE 8080

ENTRYPOINT ["./broker"]