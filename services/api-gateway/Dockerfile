# Контекст сборки — корень репозитория: сервису нужен общий модуль proto/.
FROM golang:1.26.2-alpine3.22 AS builder

WORKDIR /src
COPY proto/go.mod proto/go.sum proto/
COPY services/api-gateway/go.mod services/api-gateway/go.sum services/api-gateway/
WORKDIR /src/services/api-gateway
RUN go mod download

WORKDIR /src
COPY proto/ proto/
COPY services/api-gateway/ services/api-gateway/
WORKDIR /src/services/api-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/gateway ./cmd

# ──────────────────────────────────────────────
FROM alpine:3.22.4

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/gateway .

CMD ["./gateway"]
