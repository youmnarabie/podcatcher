package store_test

import (
	"context"
	"testing"

	"github.com/youmnarabie/poo/internal/store"
)

func TestSearch_EmptyQuery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	result, err := s.Search(ctx, "")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Episodes) != 0 || len(result.Feeds) != 0 {
		t.Fatal("expected empty results for empty query")
	}
}

func TestSearch_EpisodeTitleMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://search-test-title.example/feed.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })
	_ = s.UpdateFeedMeta(ctx, feed.ID, "Test Show", "", "")

	_, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:   feed.ID,
		GUID:     "search-test-title-1",
		Title:    "UniqueEpisodeTitleXYZ",
		AudioURL: "https://example.com/ep.mp3",
	})
	if err != nil {
		t.Fatalf("UpsertEpisode: %v", err)
	}

	result, err := s.Search(ctx, "UniqueEpisodeTitleXYZ")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Episodes) == 0 {
		t.Fatal("expected at least one episode result")
	}
	if result.Episodes[0].Title != "UniqueEpisodeTitleXYZ" {
		t.Fatalf("expected title %q got %q", "UniqueEpisodeTitleXYZ", result.Episodes[0].Title)
	}
	if result.Episodes[0].FeedTitle != "Test Show" {
		t.Fatalf("expected FeedTitle %q got %q", "Test Show", result.Episodes[0].FeedTitle)
	}
}

func TestSearch_EpisodeDescriptionMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://search-test-desc.example/feed.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })
	_ = s.UpdateFeedMeta(ctx, feed.ID, "Desc Show", "", "")

	desc := "UniqueDescriptionABC"
	_, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:      feed.ID,
		GUID:        "search-test-desc-1",
		Title:       "Some Episode",
		Description: &desc,
		AudioURL:    "https://example.com/ep2.mp3",
	})
	if err != nil {
		t.Fatalf("UpsertEpisode: %v", err)
	}

	result, err := s.Search(ctx, "UniqueDescriptionABC")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Episodes) == 0 {
		t.Fatal("expected at least one episode result from description match")
	}
}

func TestSearch_FeedTitleMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://search-test-feed.example/feed.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })
	_ = s.UpdateFeedMeta(ctx, feed.ID, "UniqueFeedTitleQQQ", "", "")

	result, err := s.Search(ctx, "UniqueFeedTitleQQQ")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Feeds) == 0 {
		t.Fatal("expected at least one feed result")
	}
	if *result.Feeds[0].Title != "UniqueFeedTitleQQQ" {
		t.Fatalf("expected feed title %q got %q", "UniqueFeedTitleQQQ", *result.Feeds[0].Title)
	}
}

func TestSearch_NoResults(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	result, err := s.Search(ctx, "zzzNOTHINGMATCHESTHISzzz")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Episodes) != 0 || len(result.Feeds) != 0 {
		t.Fatal("expected no results")
	}
}
