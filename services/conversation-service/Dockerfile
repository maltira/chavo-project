# Контекст сборки — корень репозитория: сервису нужен общий модуль proto/.
FROM golang:1.26.2-alpine3.22 AS builder

WORKDIR /src
COPY proto/go.mod proto/go.sum proto/
COPY services/conversation-service/go.mod services/conversation-service/go.sum services/conversation-service/
WORKDIR /src/services/conversation-service
RUN go mod download

WORKDIR /src
COPY proto/ proto/
COPY services/conversation-service/ services/conversation-service/
WORKDIR /src/services/conversation-service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/conversation ./cmd

# ──────────────────────────────────────────────
FROM alpine:3.22.4

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/conversation .

CMD ["./conversation"]
