package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/app"
)

func main() {
	// `user healthcheck` — проверка контейнера в compose через grpc.health.v1
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := grpcx.HealthProbe(ctx, "localhost:"+os.Getenv("PORT")); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
