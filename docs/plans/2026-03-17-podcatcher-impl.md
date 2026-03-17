# Podcatcher Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a self-hosted podcast catcher with RSS ingestion, regex-based series detection, sorting/filtering, and a React player UI.

**Architecture:** Single Go binary runs an HTTP API server and a background RSS poller. React SPA served statically from the same binary. PostgreSQL stores all state. Episodes can belong to multiple series; series assignments are additive and manual overrides are never removed by auto-detection.

**Tech Stack:** Go 1.22+, chi router, pgx/v5, golang-migrate, gofeed, React 18 + TypeScript, Vite, TanStack Query, Howler.js

**Reference docs:**
- `docs/plans/2026-03-17-podcatcher-prd.md` — product requirements
- `docs/plans/2026-03-16-podcatcher-design.md` — architecture and data model

---

## Phase 1: Foundation

### Task 1: Go module + project scaffold

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/api/.gitkeep`
- Create: `internal/ingester/.gitkeep`
- Create: `internal/store/.gitkeep`
- Create: `internal/poller/.gitkeep`
- Create: `migrations/.gitkeep`
- Create: `testdata/.gitkeep`

**Step 1: Initialize Go module**

```bash
go mod init github.com/youmnarabie/poo
```

**Step 2: Add dependencies**

```bash
go get github.com/go-chi/chi/v5
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/pgx/v5
go get github.com/golang-migrate/migrate/v4/source/file
go get github.com/mmcdole/gofeed
go get github.com/google/uuid
go get github.com/rs/cors
```

**Step 3: Write minimal main.go**

```go
package main

import (
	"log"
	"os"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}
	log.Println("starting podcatcher, db:", dbURL)
}
```

**Step 4: Verify it compiles**

```bash
go build ./...
```
Expected: no output (success)

**Step 5: Commit**

```bash
git add go.mod go.sum cmd/ internal/ migrations/ testdata/
git commit -m "feat: scaffold Go module and project structure"
```

---

### Task 2: Database migrations

**Files:**
- Create: `migrations/001_initial.up.sql`
- Create: `migrations/001_initial.down.sql`
- Create: `internal/store/migrate.go`

**Step 1: Write up migration**

```sql
-- migrations/001_initial.up.sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE feeds (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  url                   TEXT NOT NULL UNIQUE,
  title                 TEXT,
  description           TEXT,
  image_url             TEXT,
  poll_interval_seconds INT NOT NULL DEFAULT 3600,
  last_fetched_at       TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE feed_rules (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feed_id    UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  pattern    TEXT NOT NULL,
  priority   INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE episodes (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feed_id            UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  guid               TEXT NOT NULL,
  title              TEXT NOT NULL,
  description        TEXT,
  audio_url          TEXT NOT NULL,
  duration_seconds   INT,
  published_at       TIMESTAMPTZ,
  raw_season         TEXT,
  raw_episode_number TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(feed_id, guid)
);

CREATE TABLE series (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feed_id    UUID NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(feed_id, name)
);

CREATE TABLE series_episodes (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  series_id          UUID NOT NULL REFERENCES series(id) ON DELETE CASCADE,
  episode_id         UUID NOT NULL REFERENCES episodes(id) ON DELETE CASCADE,
  episode_number     INT,
  is_manual_override BOOL NOT NULL DEFAULT FALSE,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(series_id, episode_id)
);

CREATE TABLE playback_state (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  episode_id       UUID NOT NULL REFERENCES episodes(id) ON DELETE CASCADE UNIQUE,
  position_seconds INT NOT NULL DEFAULT 0,
  completed        BOOL NOT NULL DEFAULT FALSE,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Note: `series_episodes` uses `UNIQUE(series_id, episode_id)` — an episode can belong to many series.

**Step 2: Write down migration**

```sql
-- migrations/001_initial.down.sql
DROP TABLE IF EXISTS playback_state;
DROP TABLE IF EXISTS series_episodes;
DROP TABLE IF EXISTS series;
DROP TABLE IF EXISTS episodes;
DROP TABLE IF EXISTS feed_rules;
DROP TABLE IF EXISTS feeds;
```

**Step 3: Write migration runner**

```go
// internal/store/migrate.go
package store

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dbURL, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("new migrate: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
```

**Step 4: Compile check**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add migrations/ internal/store/migrate.go
git commit -m "feat: database migrations"
```

---

### Task 3: Store — feeds CRUD

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/feeds.go`
- Create: `internal/store/feeds_test.go`

**Step 1: Write the test**

```go
// internal/store/feeds_test.go
package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/youmnarabie/poo/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	s, err := store.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestFeedCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	feed, err := s.CreateFeed(ctx, "https://example.com/feed.rss")
	if err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}
	if feed.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	got, err := s.GetFeed(ctx, feed.ID)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	if got.ID != feed.ID {
		t.Fatal("ID mismatch")
	}

	feeds, err := s.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) == 0 {
		t.Fatal("expected at least one feed")
	}

	if err := s.DeleteFeed(ctx, feed.ID); err != nil {
		t.Fatalf("DeleteFeed: %v", err)
	}
	if _, err = s.GetFeed(ctx, feed.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}
```

**Step 2: Run to confirm failure**

```bash
TEST_DATABASE_URL="postgres://localhost/podcatcher_test?sslmode=disable" go test ./internal/store/... -run TestFeedCRUD -v
```
Expected: FAIL — `store.New` undefined

**Step 3: Write store.go**

```go
// internal/store/store.go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func New(ctx context.Context, dbURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{db: pool}, nil
}

func (s *Store) Close() { s.db.Close() }
```

**Step 4: Write feeds.go**

```go
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
```

**Step 5: Run test**

```bash
TEST_DATABASE_URL="postgres://localhost/podcatcher_test?sslmode=disable" go test ./internal/store/... -run TestFeedCRUD -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/store/
git commit -m "feat: store layer — feeds CRUD"
```

---

### Task 4: Store — episodes

**Files:**
- Create: `internal/store/episodes.go`

Episodes support sorting (`published_at`, `duration`, `title`) and filtering by date range in addition to feed/series/played.

**Step 1: Write episodes.go**

```go
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
```

**Step 2: Compile check**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/store/episodes.go
git commit -m "feat: store — episodes with sort/filter support"
```

---

### Task 5: Store — series, rules, playback

**Files:**
- Create: `internal/store/series.go`
- Create: `internal/store/rules.go`
- Create: `internal/store/playback.go`

**Step 1: Write series.go**

Key change from old design: `AssignEpisodeToSeries` is now additive. For auto-detection (`manual=false`), it inserts and skips if the row already exists. For manual (`manual=true`), it upserts and marks `is_manual_override=true`. `RemoveEpisodeFromSeries` now requires a `seriesID`.

```go
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
```

**Step 2: Write rules.go**

```go
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
```

**Step 3: Write playback.go**

```go
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
```

**Step 4: Compile check**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: store — series (additive assignments), rules, playback"
```

---

## Phase 2: Core Logic

### Task 6: RSS parser

**Files:**
- Create: `internal/ingester/rss.go`
- Create: `internal/ingester/rss_test.go`
- Create: `testdata/feed_murder_shack.xml`
- Create: `testdata/feed_miami_mince.xml`
- Create: `testdata/feed_prefixnum.xml`

**Step 1: Create fixture XML files**

```xml
<!-- testdata/feed_murder_shack.xml -->
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Test Fiction Podcast</title>
    <description>A test feed</description>
    <item>
      <guid>murder-shack-01</guid>
      <title>The Murder Shack 01: 'Pilot'</title>
      <enclosure url="https://example.com/ep1.mp3" type="audio/mpeg"/>
      <itunes:duration>3600</itunes:duration>
      <pubDate>Mon, 01 Jan 2024 00:00:00 +0000</pubDate>
    </item>
    <item>
      <guid>murder-shack-06-finale</guid>
      <title>The Murder Shack 06 FINALE: 'Closure?'</title>
      <enclosure url="https://example.com/ep6.mp3" type="audio/mpeg"/>
      <pubDate>Mon, 08 Jan 2024 00:00:00 +0000</pubDate>
    </item>
    <item>
      <guid>vengeance-finale</guid>
      <title>Vengeance From Beyond FINALE: 'The Ties That Bind'</title>
      <enclosure url="https://example.com/vfb.mp3" type="audio/mpeg"/>
      <pubDate>Mon, 15 Jan 2024 00:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>
```

```xml
<!-- testdata/feed_miami_mince.xml -->
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Miami Mince Podcast</title>
    <item>
      <guid>miami-01</guid>
      <title>Miami Mince—Yule Regret It 01: 'Eggnog'</title>
      <enclosure url="https://example.com/mm1.mp3" type="audio/mpeg"/>
    </item>
    <item>
      <guid>miami-02</guid>
      <title>Miami Mince—Yule Regret it 02: 'Holly'</title>
      <enclosure url="https://example.com/mm2.mp3" type="audio/mpeg"/>
    </item>
  </channel>
</rss>
```

```xml
<!-- testdata/feed_prefixnum.xml -->
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Prefix Number Podcast</title>
    <item>
      <guid>pn-03</guid>
      <title>03 Working for the Washington Brothers</title>
      <enclosure url="https://example.com/pn3.mp3" type="audio/mpeg"/>
    </item>
    <item>
      <guid>pn-3</guid>
      <title>3 Working for the Washington Brothers</title>
      <enclosure url="https://example.com/pn3b.mp3" type="audio/mpeg"/>
    </item>
  </channel>
</rss>
```

**Step 2: Write rss_test.go**

```go
// internal/ingester/rss_test.go
package ingester_test

import (
	"os"
	"testing"

	"github.com/youmnarabie/poo/internal/ingester"
)

func TestParseRSS_MurderShack(t *testing.T) {
	data, _ := os.ReadFile("../../testdata/feed_murder_shack.xml")
	feed, err := ingester.ParseRSS(data)
	if err != nil {
		t.Fatalf("ParseRSS: %v", err)
	}
	if feed.Title != "Test Fiction Podcast" {
		t.Errorf("got title %q", feed.Title)
	}
	if len(feed.Episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(feed.Episodes))
	}
	if feed.Episodes[0].AudioURL == "" {
		t.Error("missing audio URL")
	}
}

func TestParseRSS_SkipsMissingEnclosure(t *testing.T) {
	xml := `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>
		<item><guid>g1</guid><title>No audio</title></item>
	</channel></rss>`
	feed, err := ingester.ParseRSS([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Episodes) != 0 {
		t.Errorf("expected 0 episodes, got %d", len(feed.Episodes))
	}
}
```

**Step 3: Run to confirm failure**

```bash
go test ./internal/ingester/... -run TestParseRSS -v
```
Expected: FAIL — package undefined

**Step 4: Write rss.go**

```go
// internal/ingester/rss.go
package ingester

import (
	"fmt"

	"github.com/mmcdole/gofeed"
)

type ParsedFeed struct {
	Title       string
	Description string
	ImageURL    string
	Episodes    []ParsedEpisode
}

type ParsedEpisode struct {
	GUID             string
	Title            string
	Description      string
	AudioURL         string
	DurationSeconds  int
	PublishedAt      *int64
	RawSeason        string
	RawEpisodeNumber string
}

func ParseRSS(data []byte) (*ParsedFeed, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseString(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}
	out := &ParsedFeed{Title: feed.Title, Description: feed.Description}
	if feed.Image != nil {
		out.ImageURL = feed.Image.URL
	}
	for _, item := range feed.Items {
		if len(item.Enclosures) == 0 {
			continue
		}
		ep := ParsedEpisode{
			GUID:        item.GUID,
			Title:       item.Title,
			Description: item.Description,
			AudioURL:    item.Enclosures[0].URL,
		}
		if item.ITunesExt != nil {
			ep.RawSeason = item.ITunesExt.Season
			ep.RawEpisodeNumber = item.ITunesExt.Episode
			ep.DurationSeconds = parseDuration(item.ITunesExt.Duration)
		}
		if item.PublishedParsed != nil {
			t := item.PublishedParsed.Unix()
			ep.PublishedAt = &t
		}
		out.Episodes = append(out.Episodes, ep)
	}
	return out, nil
}

func parseDuration(s string) int {
	var h, m, sec int
	if n, _ := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec); n == 3 {
		return h*3600 + m*60 + sec
	}
	if n, _ := fmt.Sscanf(s, "%d:%d", &m, &sec); n == 2 {
		return m*60 + sec
	}
	fmt.Sscanf(s, "%d", &sec)
	return sec
}
```

**Step 5: Run tests**

```bash
go test ./internal/ingester/... -run TestParseRSS -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add internal/ingester/rss.go internal/ingester/rss_test.go testdata/
git commit -m "feat: RSS parser with fixture test data"
```

---

### Task 7: Series detector

**Files:**
- Create: `internal/ingester/detector.go`
- Create: `internal/ingester/detector_test.go`

**Step 1: Write detector_test.go**

```go
// internal/ingester/detector_test.go
package ingester_test

import (
	"testing"

	"github.com/youmnarabie/poo/internal/ingester"
)

var defaultRules = []ingester.Rule{
	{Pattern: `(?P<series>.+?)\s+(?P<number>\d+)\s+FINALE:`, Priority: 1},
	{Pattern: `(?P<series>.+?)\s+FINALE:`, Priority: 2},
	{Pattern: `(?P<series>.+?)\s+(?P<number>\d+):`, Priority: 3},
	{Pattern: `^(?P<number>\d+)\s+(?P<series>.+)$`, Priority: 4},
}

func TestDetect_SeriesWithNumber(t *testing.T) {
	r := ingester.DetectSeries("The Murder Shack 03: 'Benedict Cumberbatch'", defaultRules)
	if r == nil {
		t.Fatal("expected match")
	}
	if r.SeriesName != "The Murder Shack" {
		t.Errorf("got %q", r.SeriesName)
	}
	if r.EpisodeNumber == nil || *r.EpisodeNumber != 3 {
		t.Errorf("got number %v", r.EpisodeNumber)
	}
}

func TestDetect_FinaleWithNumber(t *testing.T) {
	r := ingester.DetectSeries("The Murder Shack 06 FINALE: 'Closure?'", defaultRules)
	if r == nil || r.SeriesName != "The Murder Shack" {
		t.Fatalf("got %+v", r)
	}
	if r.EpisodeNumber == nil || *r.EpisodeNumber != 6 {
		t.Errorf("got number %v", r.EpisodeNumber)
	}
}

func TestDetect_FinaleNoNumber(t *testing.T) {
	r := ingester.DetectSeries("Vengeance From Beyond FINALE: 'The Ties That Bind'", defaultRules)
	if r == nil || r.SeriesName != "Vengeance From Beyond" {
		t.Fatalf("got %+v", r)
	}
	if r.EpisodeNumber != nil {
		t.Errorf("expected no number, got %d", *r.EpisodeNumber)
	}
}

func TestDetect_PrefixNumber(t *testing.T) {
	r := ingester.DetectSeries("03 Working for the Washington Brothers", defaultRules)
	if r == nil || r.SeriesName != "Working for the Washington Brothers" {
		t.Fatalf("got %+v", r)
	}
	if r.EpisodeNumber == nil || *r.EpisodeNumber != 3 {
		t.Errorf("got number %v", r.EpisodeNumber)
	}
}

func TestDetect_SingleDigitPrefix(t *testing.T) {
	r := ingester.DetectSeries("3 Working for the Washington Brothers", defaultRules)
	if r == nil || r.EpisodeNumber == nil || *r.EpisodeNumber != 3 {
		t.Fatalf("got %+v", r)
	}
}

func TestDetect_CaseInsensitiveDedup(t *testing.T) {
	r1 := ingester.DetectSeries("Miami Mince—Yule Regret It 01: 'Eggnog'", defaultRules)
	r2 := ingester.DetectSeries("Miami Mince—Yule Regret it 02: 'Holly'", defaultRules)
	if r1 == nil || r2 == nil {
		t.Fatal("expected both to match")
	}
	if !ingester.SameSeriesName(r1.SeriesName, r2.SeriesName) {
		t.Errorf("%q vs %q should be same series", r1.SeriesName, r2.SeriesName)
	}
}

func TestDetect_NoMatch(t *testing.T) {
	if r := ingester.DetectSeries("Just a random episode title", defaultRules); r != nil {
		t.Errorf("expected no match, got %+v", r)
	}
}
```

**Step 2: Run to confirm failure**

```bash
go test ./internal/ingester/... -run TestDetect -v
```
Expected: FAIL

**Step 3: Write detector.go**

```go
// internal/ingester/detector.go
package ingester

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Rule struct {
	Pattern  string
	Priority int
}

type DetectResult struct {
	SeriesName    string
	EpisodeNumber *int
}

func DetectSeries(title string, rules []Rule) *DetectResult {
	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })

	for _, rule := range sorted {
		re, err := regexp.Compile("(?i)" + rule.Pattern)
		if err != nil {
			continue
		}
		match := re.FindStringSubmatch(title)
		if match == nil {
			continue
		}
		result := &DetectResult{}
		for i, name := range re.SubexpNames() {
			if i >= len(match) {
				break
			}
			switch name {
			case "series":
				result.SeriesName = strings.TrimSpace(match[i])
			case "number":
				trimmed := strings.TrimLeft(match[i], "0")
				if trimmed == "" {
					trimmed = "0"
				}
				if n, err := strconv.Atoi(trimmed); err == nil {
					result.EpisodeNumber = &n
				}
			}
		}
		if result.SeriesName != "" {
			return result
		}
	}
	return nil
}

func SameSeriesName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
```

**Step 4: Run tests**

```bash
go test ./internal/ingester/... -run TestDetect -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ingester/
git commit -m "feat: series detector with named capture groups"
```

---

### Task 8: Episode ingester

**Files:**
- Create: `internal/ingester/ingester.go`
- Create: `internal/ingester/ingester_test.go`

The ingester adds series assignments additively. It never removes existing rows. Manual rows (`is_manual_override=true`) are left untouched because `AssignEpisodeToSeries` with `manual=false` uses `ON CONFLICT DO NOTHING`.

**Step 1: Write ingester_test.go**

```go
// internal/ingester/ingester_test.go
package ingester_test

import (
	"context"
	"os"
	"testing"

	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/store"
)

func testDB(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	s, err := store.New(context.Background(), url)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIngestDetectsSeries(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	data, _ := os.ReadFile("../../testdata/feed_murder_shack.xml")
	feed, _ := s.CreateFeed(ctx, "https://test.example/detect.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+(?P<number>\d+)\s+FINALE:`, 1)
	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+FINALE:`, 2)
	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+(?P<number>\d+):`, 3)

	ing := ingester.New(s)
	if err := ing.IngestData(ctx, feed.ID, data); err != nil {
		t.Fatalf("IngestData: %v", err)
	}

	series, err := s.ListSeries(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, sr := range series {
		names[sr.Name] = true
	}
	if !names["The Murder Shack"] {
		t.Errorf("expected 'The Murder Shack', got %v", names)
	}
	if !names["Vengeance From Beyond"] {
		t.Errorf("expected 'Vengeance From Beyond', got %v", names)
	}
}

func TestIngestManualOverridePreserved(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	data, _ := os.ReadFile("../../testdata/feed_murder_shack.xml")
	feed, _ := s.CreateFeed(ctx, "https://test.example/override.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	// Ingest without rules first
	ing := ingester.New(s)
	_ = ing.IngestData(ctx, feed.ID, data)

	// Get an episode and manually assign it to a custom series
	eps, _ := s.ListEpisodes(ctx, store.EpisodeFilter{FeedID: feed.ID})
	if len(eps) == 0 {
		t.Skip("no episodes")
	}
	ep := eps[0]
	customSeries, _ := s.UpsertSeries(ctx, feed.ID, "My Custom Series")
	num := 99
	_ = s.AssignEpisodeToSeries(ctx, ep.ID, customSeries.ID, &num, true) // manual

	// Add rules and re-ingest
	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+(?P<number>\d+):`, 1)
	_ = ing.IngestData(ctx, feed.ID, data)

	// Verify manual assignment still exists
	eps2, _ := s.ListEpisodes(ctx, store.EpisodeFilter{FeedID: feed.ID, SeriesID: customSeries.ID})
	found := false
	for _, e := range eps2 {
		if e.ID == ep.ID {
			found = true
		}
	}
	if !found {
		t.Error("manual override was lost after re-ingest")
	}
}

func TestIngestEpisodeCanBelongToMultipleSeries(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	data, _ := os.ReadFile("../../testdata/feed_murder_shack.xml")
	feed, _ := s.CreateFeed(ctx, "https://test.example/multiseries.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })

	s.CreateRule(ctx, feed.ID, `(?P<series>.+?)\s+(?P<number>\d+):`, 1)
	ing := ingester.New(s)
	_ = ing.IngestData(ctx, feed.ID, data)

	// Get an auto-detected episode and manually add it to a second series
	eps, _ := s.ListEpisodes(ctx, store.EpisodeFilter{FeedID: feed.ID})
	if len(eps) == 0 {
		t.Skip("no episodes")
	}
	ep := eps[0]
	second, _ := s.UpsertSeries(ctx, feed.ID, "Best Of")
	_ = s.AssignEpisodeToSeries(ctx, ep.ID, second.ID, nil, true)

	// Episode should appear in both series
	inSecond, _ := s.ListEpisodes(ctx, store.EpisodeFilter{FeedID: feed.ID, SeriesID: second.ID})
	found := false
	for _, e := range inSecond {
		if e.ID == ep.ID {
			found = true
		}
	}
	if !found {
		t.Error("episode not found in second series")
	}
}
```

**Step 2: Run to confirm failure**

```bash
TEST_DATABASE_URL="postgres://localhost/podcatcher_test?sslmode=disable" go test ./internal/ingester/... -run TestIngest -v
```
Expected: FAIL — `ingester.New` undefined

**Step 3: Write ingester.go**

```go
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
		if ep.DurationSeconds > 0 {
			storeEp.DurationSeconds = &ep.DurationSeconds
		}
		if ep.PublishedAt != nil {
			t := time.Unix(*ep.PublishedAt, 0)
			storeEp.PublishedAt = &t
		}
		if ep.RawSeason != "" {
			storeEp.RawSeason = &ep.RawSeason
		}
		if ep.RawEpisodeNumber != "" {
			storeEp.RawEpisodeNumber = &ep.RawEpisodeNumber
		}

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
```

**Step 4: Run tests**

```bash
TEST_DATABASE_URL="postgres://localhost/podcatcher_test?sslmode=disable" go test ./internal/ingester/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ingester/
git commit -m "feat: episode ingester — additive series assignment, manual override preserved"
```

---

## Phase 3: HTTP API

### Task 9: HTTP server + feeds handlers

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/feeds.go`
- Create: `internal/api/stubs.go`

**Step 1: Write server.go**

```go
// internal/api/server.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/store"
)

type Server struct {
	store    *store.Store
	ingester *ingester.Ingester
	router   *chi.Mux
}

func New(s *store.Store, ing *ingester.Ingester) *Server {
	srv := &Server{store: s, ingester: ing}
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/feeds", srv.listFeeds)
		r.Post("/feeds", srv.createFeed)
		r.Delete("/feeds/{id}", srv.deleteFeed)
		r.Post("/feeds/{id}/refresh", srv.refreshFeed)
		r.Get("/feeds/{id}/series", srv.listSeries)
		r.Post("/feeds/{id}/series", srv.createSeries)
		r.Get("/feeds/{id}/rules", srv.listRules)
		r.Post("/feeds/{id}/rules", srv.createRule)

		r.Get("/episodes", srv.listEpisodes)
		r.Get("/episodes/{id}", srv.getEpisode)
		r.Get("/episodes/{id}/playback", srv.getPlayback)
		r.Put("/episodes/{id}/playback", srv.upsertPlayback)
		r.Post("/episodes/{id}/series", srv.addEpisodeSeries)
		r.Delete("/episodes/{id}/series/{seriesID}", srv.removeEpisodeSeries)

		r.Patch("/series/{id}", srv.renameSeries)
		r.Patch("/rules/{id}", srv.updateRule)
		r.Delete("/rules/{id}", srv.deleteRule)

		r.Post("/opml/import", srv.opmlImport)
		r.Get("/opml/export", srv.opmlExport)
	})

	srv.router = r
	return srv
}

func (s *Server) Handler() http.Handler { return s.router }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

**Step 2: Write feeds.go**

```go
// internal/api/feeds.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := s.store.ListFeeds(r.Context())
	if err != nil {
		writeError(w, 500, "internal error")
		return
	}
	writeJSON(w, 200, feeds)
}

func (s *Server) createFeed(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		writeError(w, 400, "url required")
		return
	}
	feed, err := s.store.CreateFeed(r.Context(), body.URL)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	go func() { _ = s.ingester.FetchAndIngest(r.Context(), feed.ID, feed.URL) }()
	writeJSON(w, 201, feed)
}

func (s *Server) deleteFeed(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteFeed(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) refreshFeed(w http.ResponseWriter, r *http.Request) {
	feed, err := s.store.GetFeed(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, "feed not found")
		return
	}
	go func() { _ = s.ingester.FetchAndIngest(r.Context(), feed.ID, feed.URL) }()
	writeJSON(w, 202, map[string]string{"status": "refreshing"})
}
```

**Step 3: Write stubs.go for everything not yet implemented**

```go
// internal/api/stubs.go
package api

import "net/http"

func (s *Server) listSeries(w http.ResponseWriter, r *http.Request)      { w.WriteHeader(501) }
func (s *Server) createSeries(w http.ResponseWriter, r *http.Request)    { w.WriteHeader(501) }
func (s *Server) renameSeries(w http.ResponseWriter, r *http.Request)    { w.WriteHeader(501) }
func (s *Server) listRules(w http.ResponseWriter, r *http.Request)       { w.WriteHeader(501) }
func (s *Server) createRule(w http.ResponseWriter, r *http.Request)      { w.WriteHeader(501) }
func (s *Server) updateRule(w http.ResponseWriter, r *http.Request)      { w.WriteHeader(501) }
func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request)      { w.WriteHeader(501) }
func (s *Server) listEpisodes(w http.ResponseWriter, r *http.Request)    { w.WriteHeader(501) }
func (s *Server) getEpisode(w http.ResponseWriter, r *http.Request)      { w.WriteHeader(501) }
func (s *Server) getPlayback(w http.ResponseWriter, r *http.Request)     { w.WriteHeader(501) }
func (s *Server) upsertPlayback(w http.ResponseWriter, r *http.Request)  { w.WriteHeader(501) }
func (s *Server) addEpisodeSeries(w http.ResponseWriter, r *http.Request)    { w.WriteHeader(501) }
func (s *Server) removeEpisodeSeries(w http.ResponseWriter, r *http.Request) { w.WriteHeader(501) }
func (s *Server) opmlImport(w http.ResponseWriter, r *http.Request)      { w.WriteHeader(501) }
func (s *Server) opmlExport(w http.ResponseWriter, r *http.Request)      { w.WriteHeader(501) }
```

**Step 4: Compile**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat: HTTP server scaffold and feeds endpoints"
```

---

### Task 10: Remaining API handlers

**Files:**
- Create: `internal/api/episodes.go`
- Create: `internal/api/series.go`
- Create: `internal/api/rules.go`
- Create: `internal/api/playback.go`
- Create: `internal/api/opml.go`
- Delete: `internal/api/stubs.go`

**Step 1: Write episodes.go**

Note the `?sort`, `?order`, `?date_from`, `?date_to` query params, and the new `POST /series` + `DELETE /series/{seriesID}` handlers.

```go
// internal/api/episodes.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/youmnarabie/poo/internal/store"
)

func (s *Server) listEpisodes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.EpisodeFilter{
		FeedID:   q.Get("feed_id"),
		SeriesID: q.Get("series_id"),
		Sort:     q.Get("sort"),
		Order:    q.Get("order"),
	}
	if played := q.Get("played"); played == "true" {
		t := true; f.Played = &t
	} else if played == "false" {
		v := false; f.Played = &v
	}
	if df := q.Get("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			f.DateFrom = &t
		}
	}
	if dt := q.Get("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			f.DateTo = &t
		}
	}
	if l, _ := strconv.Atoi(q.Get("limit")); l > 0 {
		f.Limit = l
	}
	f.Offset, _ = strconv.Atoi(q.Get("offset"))

	eps, err := s.store.ListEpisodes(r.Context(), f)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, eps)
}

func (s *Server) getEpisode(w http.ResponseWriter, r *http.Request) {
	ep, err := s.store.GetEpisode(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 404, "not found")
		return
	}
	writeJSON(w, 200, ep)
}

func (s *Server) addEpisodeSeries(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SeriesID      string `json:"series_id"`
		EpisodeNumber *int   `json:"episode_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SeriesID == "" {
		writeError(w, 400, "series_id required")
		return
	}
	err := s.store.AssignEpisodeToSeries(r.Context(),
		chi.URLParam(r, "id"), body.SeriesID, body.EpisodeNumber, true)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) removeEpisodeSeries(w http.ResponseWriter, r *http.Request) {
	err := s.store.RemoveEpisodeFromSeries(r.Context(),
		chi.URLParam(r, "id"), chi.URLParam(r, "seriesID"))
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}
```

**Step 2: Write series.go**

```go
// internal/api/series.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listSeries(w http.ResponseWriter, r *http.Request) {
	series, err := s.store.ListSeries(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, series)
}

func (s *Server) createSeries(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, 400, "name required")
		return
	}
	ser, err := s.store.UpsertSeries(r.Context(), chi.URLParam(r, "id"), body.Name)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, ser)
}

func (s *Server) renameSeries(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, 400, "name required")
		return
	}
	if err := s.store.RenameSeries(r.Context(), chi.URLParam(r, "id"), body.Name); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}
```

**Step 3: Write rules.go**

```go
// internal/api/rules.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListRules(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, rules)
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pattern  string `json:"pattern"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Pattern == "" {
		writeError(w, 400, "pattern required")
		return
	}
	rule, err := s.store.CreateRule(r.Context(), chi.URLParam(r, "id"), body.Pattern, body.Priority)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, rule)
}

func (s *Server) updateRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pattern  string `json:"pattern"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	if err := s.store.UpdateRule(r.Context(), chi.URLParam(r, "id"), body.Pattern, body.Priority); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteRule(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}
```

**Step 4: Write playback.go**

```go
// internal/api/playback.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) getPlayback(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetPlayback(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		// No state yet — return zero state
		writeJSON(w, 200, map[string]any{"position_seconds": 0, "completed": false})
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) upsertPlayback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PositionSeconds int  `json:"position_seconds"`
		Completed       bool `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	p, err := s.store.UpsertPlayback(r.Context(), chi.URLParam(r, "id"), body.PositionSeconds, body.Completed)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, p)
}
```

**Step 5: Write opml.go**

```go
// internal/api/opml.go
package api

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Body    opmlBody `xml:"body"`
}
type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}
type opmlOutline struct {
	Text   string `xml:"text,attr"`
	Type   string `xml:"type,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

func (s *Server) opmlImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, 400, "multipart parse error")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "file field required")
		return
	}
	defer file.Close()
	data, _ := io.ReadAll(file)
	var doc opmlDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		writeError(w, 400, "invalid OPML")
		return
	}
	var imported, skipped int
	for _, o := range doc.Body.Outlines {
		if o.XMLURL == "" {
			skipped++
			continue
		}
		if _, err := s.store.CreateFeed(r.Context(), o.XMLURL); err != nil {
			skipped++
			continue
		}
		imported++
	}
	writeJSON(w, 200, map[string]int{"imported": imported, "skipped": skipped})
}

func (s *Server) opmlExport(w http.ResponseWriter, r *http.Request) {
	feeds, err := s.store.ListFeeds(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	doc := opmlDoc{Version: "2.0"}
	for _, f := range feeds {
		title := f.URL
		if f.Title != nil {
			title = *f.Title
		}
		doc.Body.Outlines = append(doc.Body.Outlines, opmlOutline{Text: title, Type: "rss", XMLURL: f.URL})
	}
	out, _ := xml.MarshalIndent(doc, "", "  ")
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", `attachment; filename="podcatcher.opml"`)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>%s`, string(out))
}
```

**Step 6: Delete stubs.go**

```bash
rm internal/api/stubs.go
```

**Step 7: Compile**

```bash
go build ./...
```

**Step 8: Commit**

```bash
git add internal/api/
git commit -m "feat: all API handlers — episodes, series, rules, playback, OPML"
```

---

### Task 11: Background poller + main.go

**Files:**
- Create: `internal/poller/poller.go`
- Modify: `cmd/server/main.go`

**Step 1: Write poller.go**

```go
// internal/poller/poller.go
package poller

import (
	"context"
	"log"
	"time"

	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/store"
)

type Poller struct {
	store    *store.Store
	ingester *ingester.Ingester
	interval time.Duration
}

func New(s *store.Store, ing *ingester.Ingester, interval time.Duration) *Poller {
	return &Poller{store: s, ingester: ing, interval: interval}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollAll(ctx)
		}
	}
}

func (p *Poller) pollAll(ctx context.Context) {
	feeds, err := p.store.ListFeeds(ctx)
	if err != nil {
		log.Printf("poller: list feeds: %v", err)
		return
	}
	for _, f := range feeds {
		if err := p.ingester.FetchAndIngest(ctx, f.ID, f.URL); err != nil {
			log.Printf("poller: ingest %s: %v", f.URL, err)
		}
	}
}
```

**Step 2: Write cmd/server/main.go**

```go
// cmd/server/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rs/cors"
	"github.com/youmnarabie/poo/internal/api"
	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/poller"
	"github.com/youmnarabie/poo/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	if err := store.RunMigrations(dbURL, migrationsPath); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	s, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer s.Close()

	ing := ingester.New(s)
	srv := api.New(s, ing)

	poll := poller.New(s, ing, time.Hour)
	go poll.Run(ctx)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders: []string{"Content-Type"},
	})

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, c.Handler(srv.Handler())); err != nil {
		log.Fatal(err)
	}
}
```

**Step 3: Compile**

```bash
go build ./cmd/server
```

**Step 4: Commit**

```bash
git add cmd/server/main.go internal/poller/
git commit -m "feat: background poller and main entrypoint"
```

---

## Phase 4: Acceptance Tests

### Task 12: HTTP acceptance tests

**Files:**
- Create: `internal/api/acceptance_test.go`

**Step 1: Write acceptance_test.go**

```go
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
	resp, _ := http.Get(ts.URL + "/api/v1/opml/export")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("export: got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/xml" {
		t.Errorf("expected application/xml, got %q", ct)
	}
}
```

**Step 2: Run acceptance tests**

```bash
TEST_DATABASE_URL="postgres://localhost/podcatcher_test?sslmode=disable" go test ./internal/api/... -v
```
Expected: PASS

**Step 3: Commit**

```bash
git add internal/api/acceptance_test.go
git commit -m "test: acceptance tests — playback, multi-series, sorting, OPML"
```

---

## Phase 5: React Frontend

### Task 13: Vite + React scaffold

**Step 1: Scaffold**

```bash
cd web && npm create vite@latest . -- --template react-ts
npm install
npm install @tanstack/react-query react-router-dom axios howler
npm install -D @types/howler
```

**Step 2: Verify**

```bash
npm run build
```

**Step 3: Commit**

```bash
cd .. && git add web/
git commit -m "feat: Vite React TypeScript scaffold"
```

---

### Task 14: API client + types

**Files:**
- Create: `web/src/types.ts`
- Create: `web/src/api.ts`
- Modify: `web/src/main.tsx`

**Step 1: Write types.ts**

```typescript
// web/src/types.ts
export interface Feed {
  ID: string; URL: string; Title: string | null; Description: string | null;
  ImageURL: string | null; PollIntervalSeconds: number;
  LastFetchedAt: string | null; CreatedAt: string;
}
export interface Episode {
  ID: string; FeedID: string; GUID: string; Title: string;
  Description: string | null; AudioURL: string; DurationSeconds: number | null;
  PublishedAt: string | null; RawSeason: string | null;
  RawEpisodeNumber: string | null; CreatedAt: string;
}
export interface Series { ID: string; FeedID: string; Name: string; CreatedAt: string; }
export interface PlaybackState {
  ID: string; EpisodeID: string; PositionSeconds: number;
  Completed: boolean; UpdatedAt: string;
}
export interface FeedRule {
  ID: string; FeedID: string; Pattern: string; Priority: number; CreatedAt: string;
}
export interface EpisodeListParams {
  feed_id?: string; series_id?: string; played?: boolean;
  sort?: 'published_at' | 'duration' | 'title';
  order?: 'asc' | 'desc';
  date_from?: string; date_to?: string;
  limit?: number; offset?: number;
}
```

**Step 2: Write api.ts**

```typescript
// web/src/api.ts
import axios from 'axios';
import type { Episode, EpisodeListParams, Feed, FeedRule, PlaybackState, Series } from './types';

const client = axios.create({ baseURL: '/api/v1' });

export const api = {
  listFeeds: () => client.get<Feed[]>('/feeds').then(r => r.data),
  createFeed: (url: string) => client.post<Feed>('/feeds', { url }).then(r => r.data),
  deleteFeed: (id: string) => client.delete(`/feeds/${id}`),
  refreshFeed: (id: string) => client.post(`/feeds/${id}/refresh`),

  listEpisodes: (params: EpisodeListParams) =>
    client.get<Episode[]>('/episodes', { params }).then(r => r.data),
  getEpisode: (id: string) => client.get<Episode>(`/episodes/${id}`).then(r => r.data),
  getPlayback: (id: string) => client.get<PlaybackState>(`/episodes/${id}/playback`).then(r => r.data),
  upsertPlayback: (id: string, position_seconds: number, completed: boolean) =>
    client.put<PlaybackState>(`/episodes/${id}/playback`, { position_seconds, completed }).then(r => r.data),
  addEpisodeSeries: (id: string, series_id: string, episode_number?: number) =>
    client.post(`/episodes/${id}/series`, { series_id, episode_number }),
  removeEpisodeSeries: (id: string, seriesId: string) =>
    client.delete(`/episodes/${id}/series/${seriesId}`),

  listSeries: (feedId: string) =>
    client.get<Series[]>(`/feeds/${feedId}/series`).then(r => r.data),
  createSeries: (feedId: string, name: string) =>
    client.post<Series>(`/feeds/${feedId}/series`, { name }).then(r => r.data),
  renameSeries: (id: string, name: string) => client.patch(`/series/${id}`, { name }),

  listRules: (feedId: string) =>
    client.get<FeedRule[]>(`/feeds/${feedId}/rules`).then(r => r.data),
  createRule: (feedId: string, pattern: string, priority: number) =>
    client.post<FeedRule>(`/feeds/${feedId}/rules`, { pattern, priority }).then(r => r.data),
  updateRule: (id: string, pattern: string, priority: number) =>
    client.patch(`/rules/${id}`, { pattern, priority }),
  deleteRule: (id: string) => client.delete(`/rules/${id}`),

  exportOPML: () => client.get('/opml/export', { responseType: 'blob' }),
};
```

**Step 3: Update main.tsx**

```tsx
// web/src/main.tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App.tsx';
import './index.css';

const queryClient = new QueryClient();

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>
);
```

**Step 4: Compile check**

```bash
npm run build
```

**Step 5: Commit**

```bash
git add web/src/
git commit -m "feat: API client, types, React Query setup"
```

---

### Task 15: Views — feeds, episodes, series nav

**Files:**
- Modify: `web/src/App.tsx`
- Create: `web/src/components/FeedList.tsx`
- Create: `web/src/components/EpisodeList.tsx`
- Create: `web/src/components/EpisodeItem.tsx`
- Create: `web/src/components/SeriesNav.tsx`
- Create: `web/src/components/EpisodeFilters.tsx`

**Step 1: Write EpisodeFilters.tsx**

```tsx
// web/src/components/EpisodeFilters.tsx
import type { EpisodeListParams } from '../types';

interface Props {
  params: EpisodeListParams;
  onChange: (p: EpisodeListParams) => void;
}

export default function EpisodeFilters({ params, onChange }: Props) {
  return (
    <div style={{ display: 'flex', gap: 8, marginBottom: 12, flexWrap: 'wrap' }}>
      <select value={params.sort ?? 'published_at'}
        onChange={e => onChange({ ...params, sort: e.target.value as EpisodeListParams['sort'] })}>
        <option value="published_at">Date</option>
        <option value="duration">Duration</option>
        <option value="title">Title</option>
      </select>
      <select value={params.order ?? 'desc'}
        onChange={e => onChange({ ...params, order: e.target.value as 'asc' | 'desc' })}>
        <option value="desc">Newest first</option>
        <option value="asc">Oldest first</option>
      </select>
      <select value={params.played === undefined ? '' : String(params.played)}
        onChange={e => {
          const v = e.target.value;
          onChange({ ...params, played: v === '' ? undefined : v === 'true' });
        }}>
        <option value="">All</option>
        <option value="false">Unplayed</option>
        <option value="true">Played</option>
      </select>
      <input type="date" placeholder="From"
        value={params.date_from?.slice(0, 10) ?? ''}
        onChange={e => onChange({ ...params, date_from: e.target.value ? e.target.value + 'T00:00:00Z' : undefined })} />
      <input type="date" placeholder="To"
        value={params.date_to?.slice(0, 10) ?? ''}
        onChange={e => onChange({ ...params, date_to: e.target.value ? e.target.value + 'T23:59:59Z' : undefined })} />
    </div>
  );
}
```

**Step 2: Write EpisodeItem.tsx**

```tsx
// web/src/components/EpisodeItem.tsx
import type { Episode } from '../types';

interface Props { episode: Episode; onPlay?: (ep: Episode) => void; }

export default function EpisodeItem({ episode, onPlay }: Props) {
  const date = episode.PublishedAt ? new Date(episode.PublishedAt).toLocaleDateString() : '';
  const dur = episode.DurationSeconds
    ? `${Math.floor(episode.DurationSeconds / 60)}m` : '';
  return (
    <div style={{ borderBottom: '1px solid #eee', padding: '8px 0' }}>
      <strong>{episode.Title}</strong>
      {date && <small style={{ marginLeft: 8 }}>{date}</small>}
      {dur && <small style={{ marginLeft: 8 }}>{dur}</small>}
      <br />
      <button onClick={() => onPlay?.(episode)}>▶ Play</button>
    </div>
  );
}
```

**Step 3: Write EpisodeList.tsx**

```tsx
// web/src/components/EpisodeList.tsx
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { api } from '../api';
import type { Episode, EpisodeListParams } from '../types';
import EpisodeItem from './EpisodeItem';
import EpisodeFilters from './EpisodeFilters';

interface Props { onPlay?: (ep: Episode) => void; played?: boolean; }

export default function EpisodeList({ onPlay, played }: Props) {
  const { feedId, seriesId } = useParams();
  const [filterParams, setFilterParams] = useState<EpisodeListParams>({
    sort: 'published_at', order: 'desc', played,
  });

  const params: EpisodeListParams = { ...filterParams, feed_id: feedId, series_id: seriesId };
  const { data: episodes = [], isLoading } = useQuery({
    queryKey: ['episodes', params],
    queryFn: () => api.listEpisodes(params),
  });

  if (isLoading) return <p>Loading…</p>;
  return (
    <div>
      <EpisodeFilters params={filterParams} onChange={setFilterParams} />
      {episodes.length === 0 && <p>No episodes.</p>}
      {episodes.map(ep => <EpisodeItem key={ep.ID} episode={ep} onPlay={onPlay} />)}
    </div>
  );
}
```

**Step 4: Write SeriesNav.tsx**

```tsx
// web/src/components/SeriesNav.tsx
import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import { api } from '../api';

export default function SeriesNav() {
  const { feedId } = useParams<{ feedId: string }>();
  const { data: series = [] } = useQuery({
    queryKey: ['series', feedId],
    queryFn: () => api.listSeries(feedId!),
    enabled: !!feedId,
  });
  if (!feedId || series.length === 0) return null;
  return (
    <div style={{ marginBottom: 12 }}>
      <strong>Series: </strong>
      {series.map(s => (
        <Link key={s.ID} to={`/feeds/${feedId}/series/${s.ID}`} style={{ marginRight: 8 }}>
          {s.Name}
        </Link>
      ))}
    </div>
  );
}
```

**Step 5: Write FeedList.tsx**

```tsx
// web/src/components/FeedList.tsx
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { api } from '../api';

export default function FeedList() {
  const qc = useQueryClient();
  const { data: feeds = [], isLoading } = useQuery({ queryKey: ['feeds'], queryFn: api.listFeeds });
  const [url, setUrl] = useState('');

  const addFeed = useMutation({
    mutationFn: (u: string) => api.createFeed(u),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['feeds'] }); setUrl(''); },
  });
  const deleteFeed = useMutation({
    mutationFn: api.deleteFeed,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['feeds'] }),
  });

  if (isLoading) return <p>Loading…</p>;
  return (
    <div>
      <h2>Shows</h2>
      <form onSubmit={e => { e.preventDefault(); addFeed.mutate(url); }}>
        <input value={url} onChange={e => setUrl(e.target.value)}
          placeholder="RSS feed URL" style={{ width: 400 }} />
        <button type="submit">Add</button>
      </form>
      <ul>
        {feeds.map(f => (
          <li key={f.ID}>
            <Link to={`/feeds/${f.ID}/episodes`}>{f.Title ?? f.URL}</Link>
            {' '}
            <button onClick={() => api.refreshFeed(f.ID)}>Refresh</button>
            {' '}
            <button onClick={() => deleteFeed.mutate(f.ID)}>Delete</button>
          </li>
        ))}
      </ul>
    </div>
  );
}
```

**Step 6: Write App.tsx**

```tsx
// web/src/App.tsx
import { Route, Routes, NavLink } from 'react-router-dom';
import FeedList from './components/FeedList';
import EpisodeList from './components/EpisodeList';
import SeriesNav from './components/SeriesNav';
import Player from './components/Player';
import { usePlayer } from './hooks/usePlayer';
import { api } from './api';
import type { Episode } from './types';

export default function App() {
  const player = usePlayer();

  const handlePlay = async (ep: Episode) => {
    let startPos = 0;
    try {
      const pb = await api.getPlayback(ep.ID);
      if (!pb.Completed) startPos = pb.PositionSeconds;
    } catch {}
    player.play(ep, startPos);
  };

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: 16, paddingBottom: 80 }}>
      <nav style={{ marginBottom: 16, display: 'flex', gap: 16 }}>
        <NavLink to="/">All Episodes</NavLink>
        <NavLink to="/feeds">Shows</NavLink>
        <NavLink to="/unplayed">Unplayed</NavLink>
      </nav>
      <Routes>
        <Route path="/" element={<EpisodeList onPlay={handlePlay} />} />
        <Route path="/feeds" element={<FeedList />} />
        <Route path="/feeds/:feedId/episodes" element={
          <><SeriesNav /><EpisodeList onPlay={handlePlay} /></>
        } />
        <Route path="/feeds/:feedId/series/:seriesId" element={
          <><SeriesNav /><EpisodeList onPlay={handlePlay} /></>
        } />
        <Route path="/unplayed" element={<EpisodeList played={false} onPlay={handlePlay} />} />
      </Routes>
      <Player
        episode={player.episode} playing={player.playing}
        position={player.position} duration={player.duration} speed={player.speed}
        onToggle={player.togglePlay} onSeek={player.seek}
        onSpeedChange={player.setPlaybackSpeed}
      />
    </div>
  );
}
```

**Step 7: Compile**

```bash
npm run build
```

**Step 8: Commit**

```bash
git add web/src/
git commit -m "feat: episode list with sort/filter controls, series nav, feed management"
```

---

### Task 16: Audio player

**Files:**
- Create: `web/src/hooks/usePlayer.ts`
- Create: `web/src/components/Player.tsx`

**Step 1: Write usePlayer.ts**

```typescript
// web/src/hooks/usePlayer.ts
import { Howl } from 'howler';
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../api';
import type { Episode } from '../types';

const HEARTBEAT_MS = 10_000;

export function usePlayer() {
  const [episode, setEpisode] = useState<Episode | null>(null);
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [speed, setSpeed] = useState(1);
  const howl = useRef<Howl | null>(null);
  const hb = useRef<ReturnType<typeof setInterval> | null>(null);

  const save = useCallback((ep: Episode, pos: number, completed: boolean) => {
    api.upsertPlayback(ep.ID, Math.floor(pos), completed).catch(() => {});
  }, []);

  const stopHB = () => { if (hb.current) clearInterval(hb.current); };

  const play = useCallback((ep: Episode, startPos = 0) => {
    howl.current?.unload();
    stopHB();
    const h = new Howl({
      src: [ep.AudioURL], html5: true, rate: speed,
      onload: () => { setDuration(h.duration()); h.seek(startPos); h.play(); },
      onplay: () => setPlaying(true),
      onpause: () => { setPlaying(false); save(ep, h.seek() as number, false); },
      onend: () => { setPlaying(false); save(ep, h.duration(), true); stopHB(); },
    });
    howl.current = h;
    setEpisode(ep);
    setPosition(startPos);
    hb.current = setInterval(() => {
      if (h.playing()) { const p = h.seek() as number; setPosition(p); save(ep, p, false); }
    }, HEARTBEAT_MS);
  }, [speed, save]);

  const togglePlay = useCallback(() => {
    if (!howl.current) return;
    howl.current.playing() ? howl.current.pause() : howl.current.play();
  }, []);

  const seek = useCallback((seconds: number) => {
    if (!howl.current || !episode) return;
    howl.current.seek(seconds);
    setPosition(seconds);
    save(episode, seconds, false);
  }, [episode, save]);

  const setPlaybackSpeed = useCallback((s: number) => {
    setSpeed(s); howl.current?.rate(s);
  }, []);

  useEffect(() => {
    const id = setInterval(() => {
      if (howl.current?.playing()) setPosition(howl.current.seek() as number);
    }, 500);
    return () => clearInterval(id);
  }, []);

  useEffect(() => () => { howl.current?.unload(); stopHB(); }, []);

  return { episode, playing, position, duration, speed, play, togglePlay, seek, setPlaybackSpeed };
}
```

**Step 2: Write Player.tsx**

```tsx
// web/src/components/Player.tsx
import type { Episode } from '../types';

interface Props {
  episode: Episode | null; playing: boolean; position: number;
  duration: number; speed: number;
  onToggle: () => void; onSeek: (s: number) => void; onSpeedChange: (s: number) => void;
}

function fmt(s: number) {
  const m = Math.floor(s / 60), sec = Math.floor(s % 60);
  return `${m}:${sec.toString().padStart(2, '0')}`;
}

export default function Player({ episode, playing, position, duration, speed, onToggle, onSeek, onSpeedChange }: Props) {
  if (!episode) return null;
  return (
    <div style={{ position: 'fixed', bottom: 0, left: 0, right: 0, background: '#222', color: '#fff', padding: 12 }}>
      <div style={{ maxWidth: 900, margin: '0 auto', display: 'flex', alignItems: 'center', gap: 12 }}>
        <button onClick={onToggle} style={{ fontSize: 20 }}>{playing ? '⏸' : '▶'}</button>
        <span style={{ minWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {episode.Title}
        </span>
        <span>{fmt(position)}</span>
        <input type="range" min={0} max={duration || 1} step={1} value={position}
          onChange={e => onSeek(Number(e.target.value))} style={{ flex: 1 }} />
        <span>{fmt(duration)}</span>
        <select value={speed} onChange={e => onSpeedChange(Number(e.target.value))}>
          {[0.75, 1, 1.25, 1.5, 1.75, 2].map(s => <option key={s} value={s}>{s}×</option>)}
        </select>
      </div>
    </div>
  );
}
```

**Step 3: Compile**

```bash
npm run build
```

**Step 4: Commit**

```bash
git add web/src/
git commit -m "feat: audio player with heartbeat, seek, speed, position restore"
```

---

### Task 17: Embed frontend into Go binary

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add embed to main.go**

Replace the `main.go` content with:

```go
// cmd/server/main.go
package main

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rs/cors"
	"github.com/youmnarabie/poo/internal/api"
	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/poller"
	"github.com/youmnarabie/poo/internal/store"
)

//go:embed ../../web/dist
var webDist embed.FS

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	if err := store.RunMigrations(dbURL, migrationsPath); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	s, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer s.Close()

	ing := ingester.New(s)
	srv := api.New(s, ing)

	poll := poller.New(s, ing, time.Hour)
	go poll.Run(ctx)

	webFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatalf("web fs: %v", err)
	}
	fileServer := http.FileServer(http.FS(webFS))

	mux := http.NewServeMux()
	mux.Handle("/api/", srv.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try static file; fall back to index.html for SPA routing
		f, err := webFS.Open(r.URL.Path[1:])
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		idx, err := webFS.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer idx.Close()
		http.ServeContent(w, r, "index.html", time.Time{}, idx.(io.ReadSeeker))
	})

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders: []string{"Content-Type"},
	})

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, c.Handler(mux)); err != nil {
		log.Fatal(err)
	}
}
```

**Step 2: Build frontend then binary**

```bash
cd web && npm run build && cd ..
go build ./cmd/server -o podcatcher
```

**Step 3: Smoke test**

```bash
DATABASE_URL="postgres://localhost/podcatcher?sslmode=disable" ./podcatcher &
curl -s http://localhost:8080/api/v1/feeds | jq .
# Open http://localhost:8080 in browser
```

**Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: embed React SPA into Go binary"
```

---

## Summary

| Phase | Tasks | Key deliverables |
|---|---|---|
| 1 — Foundation | 1–5 | Go module, migrations, full store layer with multi-series and sort/filter support |
| 2 — Core Logic | 6–8 | RSS parser, series detector (all 5 patterns), additive ingester |
| 3 — HTTP API | 9–12 | All endpoints + acceptance tests |
| 4 — Frontend | 13–17 | React SPA: feeds, episodes, sort/filter, series nav, player, embed |

**~20 focused commits. Every commit leaves `go build ./...` and `npm run build` green.**

**Changed from previous plan:**
- `series_episodes` uses `UNIQUE(series_id, episode_id)` — multi-series per episode
- `AssignEpisodeToSeries` is additive: auto uses `ON CONFLICT DO NOTHING`, manual upserts
- `RemoveEpisodeFromSeries` takes a `seriesID` parameter
- Episodes API supports `?sort`, `?order`, `?date_from`, `?date_to`
- `GET /episodes/{id}/playback` endpoint added
- Series assignment endpoint changed to `POST` (add) + `DELETE /{seriesID}` (specific removal)
- Acceptance tests cover multi-series assignment and sorting/filtering
