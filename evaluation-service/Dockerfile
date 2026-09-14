# --- Estágio 1: Build ---
FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

# --- Estágio 2: Runtime ---
FROM alpine:3.21

RUN apk update && apk upgrade --no-cache && apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8004

CMD ["./server"]
