// e2e/seed/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/youmnarabie/poo/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}

	ctx := context.Background()
	s, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	feed, err := s.CreateFeed(ctx, "https://e2e-test.example/feed.rss")
	if err != nil {
		log.Fatalf("CreateFeed: %v", err)
	}

	if err := s.UpdateFeedMeta(ctx, feed.ID, "E2E Test Show", "", ""); err != nil {
		log.Fatalf("UpdateFeedMeta: %v", err)
	}

	ep, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:   feed.ID,
		GUID:     "e2e-ep-1",
		Title:    "E2E Unique Episode",
		AudioURL: "https://e2e-test.example/ep.mp3",
	})
	if err != nil {
		log.Fatalf("UpsertEpisode: %v", err)
	}

	out, _ := json.Marshal(map[string]string{
		"feedID":    feed.ID,
		"episodeID": ep.ID,
	})
	fmt.Println(string(out))
}
