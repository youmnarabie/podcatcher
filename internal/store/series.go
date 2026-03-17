// internal/store/series.go
package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Series struct {
	ID        string
	FeedID    string
	Name      string
	CreatedAt time.Time
}

func (s *Store) UpsertSeries(ctx context.Context, feedID, name string) (*Series, error) {
	var ser Series
	err := s.db.QueryRow(ctx, `
		INSERT INTO series (feed_id, name) VALUES ($1, $2)
		ON CONFLICT (feed_id, name) DO UPDATE SET name=EXCLUDED.name
		RETURNING id, feed_id, name, created_at`, feedID, name,
	).Scan(&ser.ID, &ser.FeedID, &ser.Name, &ser.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert series: %w", err)
	}
	return &ser, nil
}

func (s *Store) FindSeriesByName(ctx context.Context, feedID, name string) (*Series, error) {
	var ser Series
	err := s.db.QueryRow(ctx,
		`SELECT id, feed_id, name, created_at FROM series
		 WHERE feed_id=$1 AND LOWER(name)=LOWER($2)`, feedID, name,
	).Scan(&ser.ID, &ser.FeedID, &ser.Name, &ser.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &ser, nil
}

func (s *Store) ListSeries(ctx context.Context, feedID string) ([]*Series, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, feed_id, name, created_at FROM series WHERE feed_id=$1 ORDER BY name`, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Series
	for rows.Next() {
		var ser Series
		if err := rows.Scan(&ser.ID, &ser.FeedID, &ser.Name, &ser.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &ser)
	}
	return out, rows.Err()
}

func (s *Store) RenameSeries(ctx context.Context, id, name string) error {
	_, err := s.db.Exec(ctx, `UPDATE series SET name=$2 WHERE id=$1`, id, strings.TrimSpace(name))
	return err
}

// AssignEpisodeToSeries adds a series membership for an episode.
// manual=false: INSERT, skip if row already exists (never replace a manual row).
// manual=true:  INSERT or UPDATE, always marks is_manual_override=true.
func (s *Store) AssignEpisodeToSeries(ctx context.Context, episodeID, seriesID string, episodeNumber *int, manual bool) error {
	if manual {
		_, err := s.db.Exec(ctx, `
			INSERT INTO series_episodes (series_id, episode_id, episode_number, is_manual_override)
			VALUES ($1,$2,$3,true)
			ON CONFLICT (series_id, episode_id) DO UPDATE SET
			  episode_number=EXCLUDED.episode_number, is_manual_override=true`,
			seriesID, episodeID, episodeNumber)
		return err
	}
	// Auto: only insert if no row exists for this (series, episode) pair
	_, err := s.db.Exec(ctx, `
		INSERT INTO series_episodes (series_id, episode_id, episode_number, is_manual_override)
		VALUES ($1,$2,$3,false)
		ON CONFLICT (series_id, episode_id) DO NOTHING`,
		seriesID, episodeID, episodeNumber)
	return err
}

// RemoveEpisodeFromSeries removes a specific series assignment by episode+series ID.
func (s *Store) RemoveEpisodeFromSeries(ctx context.Context, episodeID, seriesID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM series_episodes WHERE episode_id=$1 AND series_id=$2`, episodeID, seriesID)
	return err
}
