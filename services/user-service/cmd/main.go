package main

import (
	"log"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
