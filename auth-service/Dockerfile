# --- Estágio 1: Build ---
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Copia primeiro os arquivos de dependência (melhora cache do Docker)
COPY go.mod go.sum ./
RUN go mod download

# Copia o resto do código e compila um binário estático
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

# --- Estágio 2: Runtime ---
FROM alpine:3.21

RUN apk update && apk upgrade --no-cache && apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8001

CMD ["./server"]
