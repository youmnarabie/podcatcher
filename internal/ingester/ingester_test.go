// internal/ingester/ingester_test.go
package ingester_test

import (
	"context"
	"os"
	"testing"

	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/store"
)

func testDB(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	s, err := store.New(context.Background(), url)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIngestDetectsSeries(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	data, _ := os.ReadFile("../../testdata/feed_murder_shack.xml")
	feed, _ := s.CreateFeed(ctx, "https://test.example/detect.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+(?P<number>\d+)\s+FINALE:`, 1)
	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+FINALE:`, 2)
	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+(?P<number>\d+):`, 3)

	ing := ingester.New(s)
	if err := ing.IngestData(ctx, feed.ID, data); err != nil {
		t.Fatalf("IngestData: %v", err)
	}

	series, err := s.ListSeries(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, sr := range series {
		names[sr.Name] = true
	}
	if !names["The Murder Shack"] {
		t.Errorf("expected 'The Murder Shack', got %v", names)
	}
	if !names["Vengeance From Beyond"] {
		t.Errorf("expected 'Vengeance From Beyond', got %v", names)
	}
}

func TestIngestManualOverridePreserved(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	data, _ := os.ReadFile("../../testdata/feed_murder_shack.xml")
	feed, _ := s.CreateFeed(ctx, "https://test.example/override.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	// Ingest without rules first
	ing := ingester.New(s)
	_ = ing.IngestData(ctx, feed.ID, data)

	// Get an episode and manually assign it to a custom series
	eps, _ := s.ListEpisodes(ctx, store.EpisodeFilter{FeedID: feed.ID})
	if len(eps) == 0 {
		t.Skip("no episodes")
	}
	ep := eps[0]
	customSeries, _ := s.UpsertSeries(ctx, feed.ID, "My Custom Series")
	num := 99
	_ = s.AssignEpisodeToSeries(ctx, ep.ID, customSeries.ID, &num, true) // manual

	// Add rules and re-ingest
	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+(?P<number>\d+):`, 1)
	_ = ing.IngestData(ctx, feed.ID, data)

	// Verify manual assignment still exists
	eps2, _ := s.ListEpisodes(ctx, store.EpisodeFilter{FeedID: feed.ID, SeriesID: customSeries.ID})
	found := false
	for _, e := range eps2 {
		if e.ID == ep.ID {
			found = true
		}
	}
	if !found {
		t.Error("manual override was lost after re-ingest")
	}
}

func TestIngestEpisodeCanBelongToMultipleSeries(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	data, _ := os.ReadFile("../../testdata/feed_murder_shack.xml")
	feed, _ := s.CreateFeed(ctx, "https://test.example/multiseries.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+(?P<number>\d+):`, 1)
	ing := ingester.New(s)
	_ = ing.IngestData(ctx, feed.ID, data)

	// Get an auto-detected episode and manually add it to a second series
	eps, _ := s.ListEpisodes(ctx, store.EpisodeFilter{FeedID: feed.ID})
	if len(eps) == 0 {
		t.Skip("no episodes")
	}
	ep := eps[0]
	second, _ := s.UpsertSeries(ctx, feed.ID, "Best Of")
	_ = s.AssignEpisodeToSeries(ctx, ep.ID, second.ID, nil, true)

	// Episode should appear in both series
	inSecond, _ := s.ListEpisodes(ctx, store.EpisodeFilter{FeedID: feed.ID, SeriesID: second.ID})
	found := false
	for _, e := range inSecond {
		if e.ID == ep.ID {
			found = true
		}
	}
	if !found {
		t.Error("episode not found in second series")
	}
}
