FROM golang:1.26.2-alpine3.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /auth ./cmd/main.go

FROM alpine:3.22.4

COPY --from=builder /auth /auth

CMD ["/auth"]