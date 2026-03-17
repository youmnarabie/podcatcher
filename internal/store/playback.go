// internal/store/playback.go
package store

import (
	"context"
	"time"
)

type PlaybackState struct {
	ID              string
	EpisodeID       string
	PositionSeconds int
	Completed       bool
	UpdatedAt       time.Time
}

func (s *Store) UpsertPlayback(ctx context.Context, episodeID string, positionSeconds int, completed bool) (*PlaybackState, error) {
	var p PlaybackState
	err := s.db.QueryRow(ctx, `
		INSERT INTO playback_state (episode_id, position_seconds, completed, updated_at)
		VALUES ($1,$2,$3,NOW())
		ON CONFLICT (episode_id) DO UPDATE SET
		  position_seconds=EXCLUDED.position_seconds,
		  completed=EXCLUDED.completed,
		  updated_at=NOW()
		RETURNING id, episode_id, position_seconds, completed, updated_at`,
		episodeID, positionSeconds, completed,
	).Scan(&p.ID, &p.EpisodeID, &p.PositionSeconds, &p.Completed, &p.UpdatedAt)
	return &p, err
}

func (s *Store) GetPlayback(ctx context.Context, episodeID string) (*PlaybackState, error) {
	var p PlaybackState
	err := s.db.QueryRow(ctx,
		`SELECT id, episode_id, position_seconds, completed, updated_at
		 FROM playback_state WHERE episode_id=$1`, episodeID,
	).Scan(&p.ID, &p.EpisodeID, &p.PositionSeconds, &p.Completed, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
