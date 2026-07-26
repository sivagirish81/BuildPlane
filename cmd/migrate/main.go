package main

import (
	"context"
	"log"

	"github.com/buildplane/buildplane/internal/config"
	"github.com/buildplane/buildplane/internal/database"
)

func main() {
	ctx := context.Background()
	cfg := config.Load("migrate")
	store, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
}
