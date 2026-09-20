package main

import (
	"log"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("auth service terminated with error: %v", err)
	}
}
