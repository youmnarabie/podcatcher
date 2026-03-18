// internal/api/acceptance_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/youmnarabie/poo/internal/api"
	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/store"
)

func testServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	s, err := store.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	ing := ingester.New(s)
	srv := api.New(s, ing)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	return ts, s
}

func TestFeedsEndpoints(t *testing.T) {
	ts, _ := testServer(t)

	body, _ := json.Marshal(map[string]string{"url": "https://example.com/feed.rss"})
	resp, _ := http.Post(ts.URL+"/api/v1/feeds", "application/json", bytes.NewReader(body))
	if resp.StatusCode != 201 {
		t.Fatalf("create feed: got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp2, _ := http.Get(ts.URL + "/api/v1/feeds")
	var feeds []map[string]any
	json.NewDecoder(resp2.Body).Decode(&feeds)
	resp2.Body.Close()
	if len(feeds) == 0 {
		t.Error("expected at least one feed")
	}
}

func TestPlaybackPersistence(t *testing.T) {
	ts, s := testServer(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://test.example/pb.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	// Manually insert an episode so we don't need a live RSS server
	ep, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:   feed.ID,
		GUID:     "test-ep-1",
		Title:    "Test Episode",
		AudioURL: "https://example.com/ep.mp3",
	})
	if err != nil {
		t.Fatal(err)
	}

	pbBody, _ := json.Marshal(map[string]any{"position_seconds": 42, "completed": false})
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/episodes/%s/playback", ts.URL, ep.ID),
		bytes.NewReader(pbBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	var pb map[string]any
	json.NewDecoder(resp.Body).Decode(&pb)
	resp.Body.Close()

	if pb["PositionSeconds"] != float64(42) {
		t.Errorf("expected 42, got %v", pb["PositionSeconds"])
	}

	// Fetch it back
	getResp, _ := http.Get(fmt.Sprintf("%s/api/v1/episodes/%s/playback", ts.URL, ep.ID))
	var pb2 map[string]any
	json.NewDecoder(getResp.Body).Decode(&pb2)
	getResp.Body.Close()
	if pb2["PositionSeconds"] != float64(42) {
		t.Errorf("GET playback: expected 42, got %v", pb2["PositionSeconds"])
	}
}

func TestEpisodeSortingAndFiltering(t *testing.T) {
	ts, s := testServer(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://test.example/sort.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	// Episodes list with sort param should return 200
	resp, _ := http.Get(fmt.Sprintf(
		"%s/api/v1/episodes?feed_id=%s&sort=title&order=asc", ts.URL, feed.ID))
	if resp.StatusCode != 200 {
		t.Fatalf("list episodes with sort: got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestMultipleSeriesPerEpisode(t *testing.T) {
	ts, s := testServer(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://test.example/multiseries.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	ep, _ := s.UpsertEpisode(ctx, &store.Episode{
		FeedID: feed.ID, GUID: "ms-ep-1", Title: "Test", AudioURL: "https://example.com/a.mp3",
	})
	ser1, _ := s.UpsertSeries(ctx, feed.ID, "Series One")
	ser2, _ := s.UpsertSeries(ctx, feed.ID, "Series Two")

	// Assign to ser1
	body1, _ := json.Marshal(map[string]any{"series_id": ser1.ID})
	req1, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/episodes/%s/series", ts.URL, ep.ID),
		bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	resp1, _ := http.DefaultClient.Do(req1)
	resp1.Body.Close()
	if resp1.StatusCode != 204 {
		t.Fatalf("assign ser1: got %d", resp1.StatusCode)
	}

	// Assign to ser2 (additive)
	body2, _ := json.Marshal(map[string]any{"series_id": ser2.ID})
	req2, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/v1/episodes/%s/series", ts.URL, ep.ID),
		bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req2)
	resp2.Body.Close()
	if resp2.StatusCode != 204 {
		t.Fatalf("assign ser2: got %d", resp2.StatusCode)
	}

	// Episode should appear in both series
	for _, serID := range []string{ser1.ID, ser2.ID} {
		r, _ := http.Get(fmt.Sprintf("%s/api/v1/episodes?feed_id=%s&series_id=%s", ts.URL, feed.ID, serID))
		var eps []map[string]any
		json.NewDecoder(r.Body).Decode(&eps)
		r.Body.Close()
		found := false
		for _, e := range eps {
			if e["ID"] == ep.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("episode not found in series %s", serID)
		}
	}
}

func TestOPMLExport(t *testing.T) {
	ts, _ := testServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/opml/export")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("export: got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/xml" {
		t.Errorf("expected application/xml, got %q", ct)
	}
}

func TestSearchEndpoint(t *testing.T) {
	ts, s := testServer(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://search-acceptance.example/feed.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })
	_ = s.UpdateFeedMeta(ctx, feed.ID, "Acceptance Show", "", "")

	_, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:   feed.ID,
		GUID:     "search-acceptance-ep-1",
		Title:    "UniqueAcceptanceTitleZZZ",
		AudioURL: "https://example.com/acc.mp3",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(ts.URL + "/api/v1/search?q=UniqueAcceptanceTitleZZZ")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	episodes, ok := result["Episodes"].([]any)
	if !ok || len(episodes) == 0 {
		t.Fatalf("expected episodes array with results, got: %v", result["Episodes"])
	}

	feeds, ok := result["Feeds"].([]any)
	if !ok {
		t.Fatal("expected feeds array in response")
	}
	if len(feeds) != 0 {
		t.Fatalf("expected 0 feed results (feed title doesn't match), got %d", len(feeds))
	}
}
