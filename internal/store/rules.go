// internal/store/rules.go
package store

import (
	"context"
	"time"
)

type FeedRule struct {
	ID        string
	FeedID    string
	Pattern   string
	Priority  int
	CreatedAt time.Time
}

func (s *Store) ListRules(ctx context.Context, feedID string) ([]*FeedRule, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, feed_id, pattern, priority, created_at FROM feed_rules
		 WHERE feed_id=$1 ORDER BY priority ASC`, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []*FeedRule
	for rows.Next() {
		var r FeedRule
		if err := rows.Scan(&r.ID, &r.FeedID, &r.Pattern, &r.Priority, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, &r)
	}
	return rules, rows.Err()
}

func (s *Store) CreateRule(ctx context.Context, feedID, pattern string, priority int) (*FeedRule, error) {
	var r FeedRule
	err := s.db.QueryRow(ctx,
		`INSERT INTO feed_rules (feed_id, pattern, priority) VALUES ($1,$2,$3)
		 RETURNING id, feed_id, pattern, priority, created_at`,
		feedID, pattern, priority,
	).Scan(&r.ID, &r.FeedID, &r.Pattern, &r.Priority, &r.CreatedAt)
	return &r, err
}

func (s *Store) UpdateRule(ctx context.Context, id, pattern string, priority int) error {
	_, err := s.db.Exec(ctx,
		`UPDATE feed_rules SET pattern=$2, priority=$3 WHERE id=$1`, id, pattern, priority)
	return err
}

func (s *Store) DeleteRule(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM feed_rules WHERE id=$1`, id)
	return err
}
