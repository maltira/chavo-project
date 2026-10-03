FROM golang:1.26.2-alpine3.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o conversation ./cmd

# ──────────────────────────────────────────────
FROM alpine:3.22.4

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/conversation .

CMD ["./conversation"]
