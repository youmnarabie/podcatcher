# Podcatcher Design
_2026-03-16, updated 2026-03-17_

## Overview

A self-hosted pod catcher, RSS feed reader, and podcast player with advanced series detection and filtering. The primary problem it solves: podcast feeds that publish multiple named sub-series non-chronologically, with inconsistent naming, and RSS metadata that cannot be relied on for series identification.

Initial delivery is a POC web app. A native iOS app and multi-user support are future milestones that the API design must not preclude. See `2026-03-17-podcatcher-prd.md` for the full product requirements.

---

## Architecture

```
[React SPA] <--REST/JSON--> [Go API Server] <---> [Postgres]
                                   |
                            [RSS Poller goroutine]
```

A single Go binary runs two things:
- An HTTP API server
- A background RSS poller (configurable interval, default 1h)

The React SPA is served statically. No SSR. Talks to the backend via `/api/v1/...`.

Series detection runs at ingest time when new episodes are fetched. Each feed has an ordered list of regex rules stored in Postgres. Rules use named capture groups (`(?P<series>...)`, `(?P<number>...)`) so extraction works regardless of whether the number appears before or after the series name.

---

## Naming Patterns

Observed in the wild (from real feeds). Rules must handle all of these:

| Pattern | Example |
|---|---|
| `{Series Name} {##}: '{Subtitle}'` | `The Murder Shack 03: 'Benedict Cumberbatch'` |
| `{Series Name} {##} FINALE: '{Subtitle}'` | `The Murder Shack 06 FINALE: 'Closure?'` |
| `{Series Name} FINALE: '{Subtitle}'` | `Vengeance From Beyond FINALE: 'The Ties That Bind'` |
| `{##} {Series Name}` | `03 Working for the Washington Brothers` |
| `{#} {Series Name}` | `3 Working for the Washington Brothers` |

Series names must be matched **case-insensitively** when deduplicating — e.g. "Miami Mince—Yule Regret It" and "Miami Mince—Yule Regret it" are the same series.

RSS iTunes metadata (`itunes:season`, `itunes:episode`) is preserved but **not used** for series detection — it is unreliable and misaligned with actual series names.

---

## Data Model

```sql
feeds
  id                    uuid primary key
  url                   text not null unique
  title                 text
  description           text
  image_url             text
  poll_interval_seconds int not null default 3600
  last_fetched_at       timestamptz
  created_at            timestamptz not null default now()

feed_rules                        -- ordered regex rules per feed
  id                    uuid primary key
  feed_id               uuid references feeds(id) on delete cascade
  pattern               text not null   -- Go regex with named groups
  priority              int not null    -- lower = higher priority
  created_at            timestamptz not null default now()

episodes
  id                    uuid primary key
  feed_id               uuid references feeds(id) on delete cascade
  guid                  text not null
  title                 text not null
  description           text
  audio_url             text not null
  duration_seconds      int
  published_at          timestamptz
  raw_season            text    -- from itunes:season, preserved as-is
  raw_episode_number    text    -- from itunes:episode, preserved as-is
  created_at            timestamptz not null default now()
  unique(feed_id, guid)         -- deduplication key

series
  id                    uuid primary key
  feed_id               uuid references feeds(id) on delete cascade
  name                  text not null
  created_at            timestamptz not null default now()
  unique(feed_id, name)

series_episodes                   -- resolved series-episode mapping
  id                    uuid primary key
  series_id             uuid references series(id) on delete cascade
  episode_id            uuid references episodes(id) on delete cascade
  episode_number        int
  is_manual_override    bool not null default false
  created_at            timestamptz not null default now()
  unique(series_id, episode_id)   -- one row per series-episode pair (episode can belong to many series)

playback_state
  id                    uuid primary key
  episode_id            uuid references episodes(id) on delete cascade unique
  position_seconds      int not null default 0
  completed             bool not null default false
  updated_at            timestamptz not null default now()
```

**Key decisions:**
- `episodes.guid` is the RSS `<guid>` field — deduplication key on re-fetch
- `raw_season` / `raw_episode_number` preserve RSS metadata without trusting it
- An episode can belong to multiple series — `series_episodes` rows are additive; auto-detection never removes existing rows
- `series_episodes.is_manual_override = true` on a specific row prevents the poller from ever removing or replacing that particular series assignment
- Auto-detection only adds new `series_episodes` rows; it never touches rows where `is_manual_override = true`
- `playback_state` is upserted on every position update from the player
- No user auth in POC — single-user assumed

---

## API Endpoints

```
# Feeds
GET    /api/v1/feeds                      list all feeds
POST   /api/v1/feeds                      add feed { url }
DELETE /api/v1/feeds/{id}
POST   /api/v1/feeds/{id}/refresh         on-demand fetch

# OPML
POST   /api/v1/opml/import                upload OPML file (multipart)
GET    /api/v1/opml/export                download OPML file

# Episodes
GET    /api/v1/episodes                   list episodes
                                          ?feed_id=&series_id=&played=true|false
                                          ?sort=published_at|duration|title
                                          ?order=asc|desc
                                          ?date_from=&date_to=   (ISO 8601)
                                          ?limit=&offset=
GET    /api/v1/episodes/{id}
GET    /api/v1/episodes/{id}/playback     get playback state

# Series
GET    /api/v1/feeds/{id}/series          list series for a feed
POST   /api/v1/feeds/{id}/series          create series manually { name }
PATCH  /api/v1/series/{id}               rename series { name }

# Series-episode assignment (additive — does not replace existing assignments)
POST   /api/v1/episodes/{id}/series       add series membership { series_id, episode_number }
DELETE /api/v1/episodes/{id}/series/{series_id}   remove specific series assignment

# Feed rules
GET    /api/v1/feeds/{id}/rules
POST   /api/v1/feeds/{id}/rules           { pattern, priority }
PATCH  /api/v1/rules/{id}                { pattern, priority }
DELETE /api/v1/rules/{id}

# Playback
PUT    /api/v1/episodes/{id}/playback     upsert { position_seconds, completed }
```

All responses: `application/json`. List endpoints paginate via `?limit=&offset=`.

---

## Player

Custom player component (not native `<audio>` defaults):
- Play / pause
- Seek bar
- Playback speed control
- Position persisted to backend on pause, seek, and on a periodic heartbeat (every 10s while playing)
- Position restored on episode open

---

## Views

| View | Description |
|---|---|
| All Episodes | Flat list across all feeds |
| By Show | Episodes grouped by feed |
| By Series | Episodes grouped by series within a feed |
| Unplayed | Episodes where `completed = false` |
| Played | Episodes where `completed = true` |

**Sorting** (available in all views): publish date (default: newest first), duration, title (alphabetical).

**Filtering** (composable across all views): feed, series, played/unplayed state, date range.

Played/unplayed filter is composable with show and series views.

---

## Testing Strategy

### Unit tests — pure Go, no DB or network
- Series detection: rule application, priority ordering, named capture group extraction
- Case-insensitive series name normalisation / deduplication
- RSS XML parsing: title extraction, guid parsing, OPML import/export
- All four naming pattern variants handled correctly (including FINALE-only and prefix-number)

### Integration tests — real Postgres via Docker, no network
- Feed CRUD and rule management
- Episode ingest + auto series assignment end-to-end
- Episode can be assigned to multiple series; auto-detection does not remove manual assignments
- Manual override survives re-fetch (`is_manual_override` respected per row)
- Playback state upsert and retrieval

### Acceptance tests — full stack over HTTP against a running test server
- Add feed → episodes appear → series detected correctly
- Episode can be assigned to multiple series via API
- Manual series assignment survives re-fetch
- Sorting and filtering parameters return correct episode subsets
- Played/unplayed filter returns correct episodes
- OPML round-trip: import then export produces equivalent feed list
- On-demand refresh (`POST /feeds/{id}/refresh`) triggers ingest
- Player position persists across page reload

### Frontend
- React Testing Library: component-level tests for player, episode list, series views
- Playwright: end-to-end — play episode, seek, reload, confirm position restored

### Test fixtures
- Fixture RSS XML files checked into `testdata/` based on real feed patterns, including:
  - Consistent naming (`The Murder Shack 01–06 FINALE`)
  - Capitalisation inconsistency (`Miami Mince—Yule Regret It` vs `it`)
  - FINALE without episode number (`Vengeance From Beyond FINALE`)
  - Prefix-number format (`03 Series Name`)

---

## Project Structure (initial)

```
/
├── cmd/
│   └── server/         -- main entrypoint
├── internal/
│   ├── api/            -- HTTP handlers
│   ├── ingester/       -- RSS fetch + series detection
│   ├── store/          -- Postgres queries
│   └── poller/         -- background polling goroutine
├── migrations/         -- SQL migration files
├── testdata/           -- fixture RSS XML files
├── web/                -- React SPA
│   ├── src/
│   └── ...
└── docs/
    └── plans/
```

---

## Out of Scope for POC

The following are deferred to later milestones — not ruled out. See the PRD for the full roadmap.

- User authentication / multi-user (Milestone 2)
- Social sign-in (Milestone 2)
- Native iOS app (Milestone 3)
- Push notifications
- Chapter support
- Sleep timer
- Queue management
- Full-text episode search
