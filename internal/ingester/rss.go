// internal/ingester/rss.go
package ingester

import (
	"bytes"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

type RSSFeed struct {
	Title       string
	Description string
	ImageURL    string
	Episodes    []RSSEpisode
}

type RSSEpisode struct {
	GUID             string
	Title            string
	Description      string
	AudioURL         string
	DurationSeconds  *int
	PublishedAt      *time.Time
	RawSeason        *string
	RawEpisodeNumber *string
}

func ParseRSS(data []byte) (*RSSFeed, error) {
	fp := gofeed.NewParser()
	feed, err := fp.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}

	out := &RSSFeed{
		Title:       feed.Title,
		Description: feed.Description,
	}
	if feed.Image != nil {
		out.ImageURL = feed.Image.URL
	}

	for _, item := range feed.Items {
		if item.Enclosures == nil || len(item.Enclosures) == 0 {
			continue
		}
		audioURL := ""
		for _, enc := range item.Enclosures {
			if enc.URL != "" {
				audioURL = enc.URL
				break
			}
		}
		if audioURL == "" {
			continue
		}

		ep := RSSEpisode{
			GUID:     item.GUID,
			Title:    item.Title,
			AudioURL: audioURL,
		}
		if item.Description != "" {
			ep.Description = item.Description
		}
		if item.PublishedParsed != nil {
			t := *item.PublishedParsed
			ep.PublishedAt = &t
		}

		// Extract itunes season/episode if present
		if item.ITunesExt != nil {
			if item.ITunesExt.Season != "" {
				s := item.ITunesExt.Season
				ep.RawSeason = &s
			}
			if item.ITunesExt.Episode != "" {
				e := item.ITunesExt.Episode
				ep.RawEpisodeNumber = &e
			}
			if item.ITunesExt.Duration != "" {
				d := parseDurationSeconds(item.ITunesExt.Duration)
				if d > 0 {
					ep.DurationSeconds = &d
				}
			}
		}

		out.Episodes = append(out.Episodes, ep)
	}

	return out, nil
}

func parseDurationSeconds(s string) int {
	// Try HH:MM:SS or MM:SS or plain seconds
	var h, m, sec int
	if n, _ := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec); n == 3 {
		return h*3600 + m*60 + sec
	}
	if n, _ := fmt.Sscanf(s, "%d:%d", &m, &sec); n == 2 {
		return m*60 + sec
	}
	if n, _ := fmt.Sscanf(s, "%d", &sec); n == 1 {
		return sec
	}
	return 0
}
