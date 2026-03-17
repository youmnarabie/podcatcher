// internal/ingester/ingester.go
package ingester

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/youmnarabie/poo/internal/store"
)

type Ingester struct {
	store  *store.Store
	client *http.Client
}

func New(s *store.Store) *Ingester {
	return &Ingester{store: s, client: &http.Client{Timeout: 30 * time.Second}}
}

func (ing *Ingester) FetchAndIngest(ctx context.Context, feedID, url string) error {
	resp, err := ing.client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return ing.IngestData(ctx, feedID, data)
}

func (ing *Ingester) IngestData(ctx context.Context, feedID string, data []byte) error {
	parsed, err := ParseRSS(data)
	if err != nil {
		return err
	}
	_ = ing.store.UpdateFeedMeta(ctx, feedID, parsed.Title, parsed.Description, parsed.ImageURL)

	rules, err := ing.store.ListRules(ctx, feedID)
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}
	detectorRules := make([]Rule, len(rules))
	for i, r := range rules {
		detectorRules[i] = Rule{Pattern: r.Pattern, Priority: r.Priority}
	}

	for _, ep := range parsed.Episodes {
		storeEp := &store.Episode{
			FeedID:   feedID,
			GUID:     ep.GUID,
			Title:    ep.Title,
			AudioURL: ep.AudioURL,
		}
		if ep.Description != "" {
			storeEp.Description = &ep.Description
		}
		// Use pointer types directly (RSSEpisode uses *int, *time.Time, *string)
		storeEp.DurationSeconds = ep.DurationSeconds
		storeEp.PublishedAt = ep.PublishedAt
		storeEp.RawSeason = ep.RawSeason
		storeEp.RawEpisodeNumber = ep.RawEpisodeNumber

		inserted, err := ing.store.UpsertEpisode(ctx, storeEp)
		if err != nil {
			return fmt.Errorf("upsert episode %q: %w", ep.GUID, err)
		}

		if len(detectorRules) == 0 {
			continue
		}

		result := DetectSeries(ep.Title, detectorRules)
		if result == nil {
			continue
		}

		existing, err := ing.store.FindSeriesByName(ctx, feedID, result.SeriesName)
		var seriesID string
		if err != nil {
			ser, err := ing.store.UpsertSeries(ctx, feedID, strings.TrimSpace(result.SeriesName))
			if err != nil {
				return fmt.Errorf("upsert series: %w", err)
			}
			seriesID = ser.ID
		} else {
			seriesID = existing.ID
		}

		// Additive, manual=false: ON CONFLICT DO NOTHING — won't touch manual rows
		if err := ing.store.AssignEpisodeToSeries(ctx, inserted.ID, seriesID, result.EpisodeNumber, false); err != nil {
			return fmt.Errorf("assign series: %w", err)
		}
	}
	return nil
}
