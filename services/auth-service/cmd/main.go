package main

import (
	"context"

	"github.com/maltira/chavo-project-backend/services/auth-service/pkg/db"
)

func main() {
	ctx := context.Background()

	pool, err := db.NewPool(ctx)
	if err != nil {
		panic("Failed to connect to database: " + err.Error())
	}
	defer db.ClosePool(pool)
}
