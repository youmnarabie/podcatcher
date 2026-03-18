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
	result := &SearchResult{
		Episodes: make([]*EpisodeWithFeed, 0),
		Feeds:    make([]*Feed, 0),
	}
	if query == "" {
		return result, nil
	}

	pattern := "%" + query + "%"
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		rows, err := s.db.Query(ctx, `
			SELECT e.id, e.feed_id, e.guid, e.title, e.description, e.audio_url,
			       e.duration_seconds, e.published_at, e.raw_season, e.raw_episode_number,
			       e.created_at, f.title
			FROM episodes e
			JOIN feeds f ON f.id = e.feed_id
			WHERE e.title ILIKE $1 OR e.description ILIKE $1
			ORDER BY e.published_at DESC NULLS LAST
			LIMIT 50`, pattern)
		if err != nil {
			return err
		}
		defer rows.Close()
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
			result.Episodes = append(result.Episodes, &ewf)
		}
		return rows.Err()
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
		for rows.Next() {
			var f Feed
			if err := rows.Scan(
				&f.ID, &f.URL, &f.Title, &f.Description, &f.ImageURL,
				&f.PollIntervalSeconds, &f.LastFetchedAt, &f.CreatedAt,
			); err != nil {
				return err
			}
			result.Feeds = append(result.Feeds, &f)
		}
		return rows.Err()
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}
