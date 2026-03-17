// internal/store/episodes.go
package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Episode struct {
	ID               string
	FeedID           string
	GUID             string
	Title            string
	Description      *string
	AudioURL         string
	DurationSeconds  *int
	PublishedAt      *time.Time
	RawSeason        *string
	RawEpisodeNumber *string
	CreatedAt        time.Time
}

type EpisodeFilter struct {
	FeedID   string
	SeriesID string
	Played   *bool
	Sort     string     // "published_at" | "duration" | "title" — default "published_at"
	Order    string     // "asc" | "desc" — default "desc"
	DateFrom *time.Time
	DateTo   *time.Time
	Limit    int
	Offset   int
}

var allowedSortCols = map[string]string{
	"published_at": "e.published_at",
	"duration":     "e.duration_seconds",
	"title":        "e.title",
}

func (s *Store) UpsertEpisode(ctx context.Context, e *Episode) (*Episode, error) {
	var out Episode
	err := s.db.QueryRow(ctx, `
		INSERT INTO episodes
		  (feed_id, guid, title, description, audio_url, duration_seconds, published_at, raw_season, raw_episode_number)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (feed_id, guid) DO UPDATE SET
		  title=EXCLUDED.title, description=EXCLUDED.description,
		  audio_url=EXCLUDED.audio_url, duration_seconds=EXCLUDED.duration_seconds,
		  published_at=EXCLUDED.published_at, raw_season=EXCLUDED.raw_season,
		  raw_episode_number=EXCLUDED.raw_episode_number
		RETURNING id, feed_id, guid, title, description, audio_url,
		          duration_seconds, published_at, raw_season, raw_episode_number, created_at`,
		e.FeedID, e.GUID, e.Title, e.Description, e.AudioURL,
		e.DurationSeconds, e.PublishedAt, e.RawSeason, e.RawEpisodeNumber,
	).Scan(&out.ID, &out.FeedID, &out.GUID, &out.Title, &out.Description,
		&out.AudioURL, &out.DurationSeconds, &out.PublishedAt,
		&out.RawSeason, &out.RawEpisodeNumber, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert episode: %w", err)
	}
	return &out, nil
}

func (s *Store) GetEpisode(ctx context.Context, id string) (*Episode, error) {
	var e Episode
	err := s.db.QueryRow(ctx, `
		SELECT id, feed_id, guid, title, description, audio_url,
		       duration_seconds, published_at, raw_season, raw_episode_number, created_at
		FROM episodes WHERE id=$1`, id,
	).Scan(&e.ID, &e.FeedID, &e.GUID, &e.Title, &e.Description,
		&e.AudioURL, &e.DurationSeconds, &e.PublishedAt,
		&e.RawSeason, &e.RawEpisodeNumber, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get episode: %w", err)
	}
	return &e, nil
}

func (s *Store) ListEpisodes(ctx context.Context, f EpisodeFilter) ([]*Episode, error) {
	limit := f.Limit
	if limit == 0 {
		limit = 50
	}

	sortCol, ok := allowedSortCols[f.Sort]
	if !ok {
		sortCol = "e.published_at"
	}
	order := "DESC"
	if strings.ToLower(f.Order) == "asc" {
		order = "ASC"
	}

	q := fmt.Sprintf(`
		SELECT DISTINCT e.id, e.feed_id, e.guid, e.title, e.description, e.audio_url,
		       e.duration_seconds, e.published_at, e.raw_season, e.raw_episode_number, e.created_at
		FROM episodes e
		LEFT JOIN series_episodes se ON se.episode_id = e.id
		LEFT JOIN playback_state ps ON ps.episode_id = e.id
		WHERE ($1='' OR e.feed_id=$1::uuid)
		  AND ($2='' OR se.series_id=$2::uuid)
		  AND ($3::bool IS NULL OR COALESCE(ps.completed, false) = $3)
		  AND ($4::timestamptz IS NULL OR e.published_at >= $4)
		  AND ($5::timestamptz IS NULL OR e.published_at <= $5)
		ORDER BY %s %s NULLS LAST
		LIMIT $6 OFFSET $7`, sortCol, order)

	rows, err := s.db.Query(ctx, q,
		f.FeedID, f.SeriesID, f.Played, f.DateFrom, f.DateTo, limit, f.Offset)
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	defer rows.Close()
	var eps []*Episode
	for rows.Next() {
		var e Episode
		if err := rows.Scan(&e.ID, &e.FeedID, &e.GUID, &e.Title, &e.Description,
			&e.AudioURL, &e.DurationSeconds, &e.PublishedAt,
			&e.RawSeason, &e.RawEpisodeNumber, &e.CreatedAt); err != nil {
			return nil, err
		}
		eps = append(eps, &e)
	}
	return eps, rows.Err()
}
