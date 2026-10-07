FROM golang:1.26.2-alpine3.22 AS builder

WORKDIR /src
COPY proto/go.mod proto/go.sum proto/
COPY services/auth-service/go.mod services/auth-service/go.sum services/auth-service/
WORKDIR /src/services/auth-service
RUN go mod download

WORKDIR /src
COPY proto/ proto/
COPY services/auth-service/ services/auth-service/
WORKDIR /src/services/auth-service
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/auth ./cmd

# ──────────────────────────────────────────────
FROM alpine:3.22.4

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/auth .

CMD ["./auth"]
