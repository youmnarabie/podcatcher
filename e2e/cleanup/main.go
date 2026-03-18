// e2e/cleanup/main.go
package main

import (
	"context"
	"log"
	"os"

	"github.com/youmnarabie/poo/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	feedID := os.Getenv("FEED_ID")
	if dbURL == "" || feedID == "" {
		log.Fatal("DATABASE_URL and FEED_ID required")
	}

	ctx := context.Background()
	s, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	if err := s.DeleteFeed(ctx, feedID); err != nil {
		log.Fatalf("DeleteFeed: %v", err)
	}
}
