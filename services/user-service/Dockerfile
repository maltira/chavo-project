# Контекст сборки — корень репозитория: сервису нужен общий модуль proto/.
FROM golang:1.26.2-alpine3.22 AS builder

WORKDIR /src
COPY proto/go.mod proto/go.sum proto/
COPY services/user-service/go.mod services/user-service/go.sum services/user-service/
WORKDIR /src/services/user-service
RUN go mod download

WORKDIR /src
COPY proto/ proto/
COPY services/user-service/ services/user-service/
WORKDIR /src/services/user-service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/user ./cmd

# ──────────────────────────────────────────────
FROM alpine:3.22.4

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/user .

CMD ["./user"]
