package main

import (
	"log"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("conversation service terminated with error: %v", err)
	}
}
