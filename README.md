# podcatcher

A self-hosted podcast catcher. Single Go binary serves an HTTP API and a React SPA. RSS feeds are polled in the background. PostgreSQL stores all state.

## Prerequisites

- Go 1.22+
- Node.js 18+ and npm
- PostgreSQL

## Running

### 1. Create the database

```bash
createdb podcatcher
```

### 2. Build the frontend

The Go binary embeds the compiled frontend. Build it first:

```bash
cd web
npm install
npm run build
cd ..
```

### 3. Build and run the server

```bash
go build -o podcatcher ./cmd/server
DATABASE_URL="postgres://localhost/podcatcher?sslmode=disable" ./podcatcher
```

Open http://localhost:8080 in a browser.

#### Environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | *(required)* | PostgreSQL connection string |
| `ADDR` | `:8080` | Listen address |
| `MIGRATIONS_PATH` | `migrations` | Path to migration files |

Migrations run automatically at startup.

## Development

Run the frontend dev server and Go API server separately so Vite's hot-reload works:

```bash
# Terminal 1 — Go API on :8080
DATABASE_URL="postgres://localhost/podcatcher?sslmode=disable" go run ./cmd/server

# Terminal 2 — Vite dev server on :5173 (proxies /api to :8080)
cd web && npm run dev
```

The Vite config already proxies `/api` requests to the Go server.

## Testing

### Unit / integration tests

The acceptance tests in `internal/api/acceptance_test.go` require a real PostgreSQL database. Create one and point `TEST_DATABASE_URL` at it:

```bash
createdb podcatcher_test

TEST_DATABASE_URL="postgres://localhost/podcatcher_test?sslmode=disable" \
  go test ./internal/api/... -v
```

Tests skip automatically when `TEST_DATABASE_URL` is not set, so a plain `go test ./...` is safe to run without a database:

```bash
go test ./...
```

### What the acceptance tests cover

| Test | What it checks |
|---|---|
| `TestFeedsEndpoints` | Create and list feeds via HTTP |
| `TestPlaybackPersistence` | PUT and GET playback position round-trips |
| `TestEpisodeSortingAndFiltering` | Episode list accepts `sort`/`order` query params |
| `TestMultipleSeriesPerEpisode` | Additive series assignment — episode appears in both series |
| `TestOPMLExport` | Export returns HTTP 200 with `application/xml` |

### Verify the build compiles

```bash
go build ./...
go vet ./...
```

## API overview

All endpoints are under `/api/v1`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/feeds` | List feeds |
| `POST` | `/feeds` | Add feed `{"url": "..."}` |
| `DELETE` | `/feeds/{id}` | Remove feed |
| `POST` | `/feeds/{id}/refresh` | Trigger immediate RSS fetch |
| `GET` | `/feeds/{id}/series` | List series for a feed |
| `POST` | `/feeds/{id}/series` | Create series `{"name": "..."}` |
| `GET` | `/feeds/{id}/rules` | List regex rules |
| `POST` | `/feeds/{id}/rules` | Create rule `{"pattern": "...", "priority": 0}` |
| `GET` | `/episodes` | List episodes (filters: `feed_id`, `series_id`, `sort`, `order`, `played`, `date_from`, `date_to`, `limit`, `offset`) |
| `GET` | `/episodes/{id}` | Get episode |
| `GET` | `/episodes/{id}/playback` | Get playback state |
| `PUT` | `/episodes/{id}/playback` | Upsert playback `{"position_seconds": 42, "completed": false}` |
| `POST` | `/episodes/{id}/series` | Assign episode to series `{"series_id": "..."}` |
| `DELETE` | `/episodes/{id}/series/{seriesID}` | Remove episode from series |
| `PATCH` | `/series/{id}` | Rename series `{"name": "..."}` |
| `PATCH` | `/rules/{id}` | Update rule |
| `DELETE` | `/rules/{id}` | Delete rule |
| `POST` | `/opml/import` | Import OPML (multipart `file` field) |
| `GET` | `/opml/export` | Export feeds as OPML XML |
