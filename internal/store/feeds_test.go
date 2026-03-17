// internal/store/feeds_test.go
package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/youmnarabie/poo/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	s, err := store.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestFeedCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	feed, err := s.CreateFeed(ctx, "https://example.com/feed.rss")
	if err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}
	if feed.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	got, err := s.GetFeed(ctx, feed.ID)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	if got.ID != feed.ID {
		t.Fatal("ID mismatch")
	}

	feeds, err := s.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) == 0 {
		t.Fatal("expected at least one feed")
	}

	if err := s.DeleteFeed(ctx, feed.ID); err != nil {
		t.Fatalf("DeleteFeed: %v", err)
	}
	if _, err = s.GetFeed(ctx, feed.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}
