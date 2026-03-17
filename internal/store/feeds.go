// internal/store/feeds.go
package store

import (
	"context"
	"fmt"
	"time"
)

type Feed struct {
	ID                  string
	URL                 string
	Title               *string
	Description         *string
	ImageURL            *string
	PollIntervalSeconds int
	LastFetchedAt       *time.Time
	CreatedAt           time.Time
}

func (s *Store) CreateFeed(ctx context.Context, url string) (*Feed, error) {
	var f Feed
	err := s.db.QueryRow(ctx,
		`INSERT INTO feeds (url) VALUES ($1)
		 RETURNING id, url, title, description, image_url, poll_interval_seconds, last_fetched_at, created_at`,
		url,
	).Scan(&f.ID, &f.URL, &f.Title, &f.Description, &f.ImageURL, &f.PollIntervalSeconds, &f.LastFetchedAt, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert feed: %w", err)
	}
	return &f, nil
}

func (s *Store) GetFeed(ctx context.Context, id string) (*Feed, error) {
	var f Feed
	err := s.db.QueryRow(ctx,
		`SELECT id, url, title, description, image_url, poll_interval_seconds, last_fetched_at, created_at
		 FROM feeds WHERE id = $1`, id,
	).Scan(&f.ID, &f.URL, &f.Title, &f.Description, &f.ImageURL, &f.PollIntervalSeconds, &f.LastFetchedAt, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}
	return &f, nil
}

func (s *Store) ListFeeds(ctx context.Context) ([]*Feed, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, url, title, description, image_url, poll_interval_seconds, last_fetched_at, created_at
		 FROM feeds ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var feeds []*Feed
	for rows.Next() {
		var f Feed
		if err := rows.Scan(&f.ID, &f.URL, &f.Title, &f.Description, &f.ImageURL,
			&f.PollIntervalSeconds, &f.LastFetchedAt, &f.CreatedAt); err != nil {
			return nil, err
		}
		feeds = append(feeds, &f)
	}
	return feeds, rows.Err()
}

func (s *Store) UpdateFeedMeta(ctx context.Context, id, title, description, imageURL string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE feeds SET title=$2, description=$3, image_url=$4, last_fetched_at=NOW() WHERE id=$1`,
		id, title, description, imageURL)
	return err
}

func (s *Store) DeleteFeed(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM feeds WHERE id = $1`, id)
	return err
}
