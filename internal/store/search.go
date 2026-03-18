// internal/store/search.go
package store

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type EpisodeWithFeed struct {
	Episode
	FeedTitle string
}

type SearchResult struct {
	Episodes []*EpisodeWithFeed
	Feeds    []*Feed
}

func (s *Store) Search(ctx context.Context, query string) (*SearchResult, error) {
	if query == "" {
		return &SearchResult{
			Episodes: make([]*EpisodeWithFeed, 0),
			Feeds:    make([]*Feed, 0),
		}, nil
	}

	pattern := "%" + query + "%"
	g, ctx := errgroup.WithContext(ctx)

	// episodes and feeds are written exclusively by their respective goroutines
	// and are only safe to read after g.Wait() returns.
	var episodes []*EpisodeWithFeed
	var feeds []*Feed

	g.Go(func() error {
		rows, err := s.db.Query(ctx, `
			SELECT e.id, e.feed_id, e.guid, e.title, e.description, e.audio_url,
			       e.duration_seconds, e.published_at, e.raw_season, e.raw_episode_number,
			       e.created_at, COALESCE(f.title, '') AS feed_title
			FROM episodes e
			JOIN feeds f ON f.id = e.feed_id
			WHERE e.title ILIKE $1 OR e.description ILIKE $1
			ORDER BY e.published_at DESC NULLS LAST
			LIMIT 50`, pattern)
		if err != nil {
			return err
		}
		defer rows.Close()
		var eps []*EpisodeWithFeed
		for rows.Next() {
			var ewf EpisodeWithFeed
			if err := rows.Scan(
				&ewf.ID, &ewf.FeedID, &ewf.GUID, &ewf.Title, &ewf.Description,
				&ewf.AudioURL, &ewf.DurationSeconds, &ewf.PublishedAt,
				&ewf.RawSeason, &ewf.RawEpisodeNumber, &ewf.CreatedAt,
				&ewf.FeedTitle,
			); err != nil {
				return err
			}
			eps = append(eps, &ewf)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		episodes = eps
		return nil
	})

	g.Go(func() error {
		rows, err := s.db.Query(ctx, `
			SELECT id, url, title, description, image_url, poll_interval_seconds, last_fetched_at, created_at
			FROM feeds
			WHERE title ILIKE $1
			LIMIT 50`, pattern)
		if err != nil {
			return err
		}
		defer rows.Close()
		var fs []*Feed
		for rows.Next() {
			var f Feed
			if err := rows.Scan(
				&f.ID, &f.URL, &f.Title, &f.Description, &f.ImageURL,
				&f.PollIntervalSeconds, &f.LastFetchedAt, &f.CreatedAt,
			); err != nil {
				return err
			}
			fs = append(fs, &f)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		feeds = fs
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if episodes == nil {
		episodes = make([]*EpisodeWithFeed, 0)
	}
	if feeds == nil {
		feeds = make([]*Feed, 0)
	}

	return &SearchResult{Episodes: episodes, Feeds: feeds}, nil
}
