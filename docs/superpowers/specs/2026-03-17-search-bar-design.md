# Search Bar Design

**Date:** 2026-03-17
**Project:** poo (podcast catcher)
**Status:** Approved

## Overview

Add a globally visible search bar to the nav that searches episodes and feeds (shows) as the user types. Results are served by a new dedicated API endpoint.

## Backend

### New endpoint

`GET /api/search?q=<text>`

- Empty or missing `q` → return `{"episodes":[],"feeds":[]}` immediately, no DB query
- Otherwise, run ILIKE-based searches against episodes and feeds in parallel

Response shape:
```json
{
  "episodes": [{ ...Episode fields..., "FeedTitle": "string" }],
  "feeds":    [{ ...Feed fields... }]
}
```

### New files

**`internal/store/search.go`**
Adds `Search(ctx context.Context, query string) (*SearchResult, error)` to `Store`.
- Episodes query: `SELECT ... FROM episodes e JOIN feeds f ON f.id = e.feed_id WHERE e.title ILIKE $1 OR e.description ILIKE $1` — returns episode fields plus `f.title` as `FeedTitle`
- Feeds query: `SELECT ... FROM feeds WHERE title ILIKE $1`
- Both use `'%' || query || '%'` as the pattern
- Results capped at 50 each

`SearchResult` struct:
```go
type SearchResult struct {
    Episodes []*EpisodeWithFeed
    Feeds    []*Feed
}

type EpisodeWithFeed struct {
    Episode
    FeedTitle string
}
```

**`internal/api/search.go`**
Registers `GET /search` on the router (alongside existing routes in `server.go`).
Handler reads `q`, calls `s.store.Search`, writes JSON.

### Error handling

- DB error → 500 + `{"error":"..."}` JSON
- No results → 200 + empty arrays

## Frontend

### SearchBar component (`web/src/components/SearchBar.tsx`)

- Controlled `<input>` in the nav in `App.tsx`
- 300ms debounce on the input value
- On debounced change, updates the URL: navigates to `/search?q=<text>` (replaces history entry)
- Clearing the input navigates back to `/`

### SearchResults page (`web/src/components/SearchResults.tsx`)

- Mounted at route `/search`
- Reads `q` from URL search params
- Fetches `api.search(q)` via React Query (query key: `['search', q]`)
- Renders two sections:
  - **Episodes** — reuses `EpisodeItem`, shows feed name as subtitle
  - **Shows** — minimal feed card (name + link to feed episodes)
- States:
  - `q` empty → prompt: "Type to search"
  - Loading → "Searching…"
  - No results → "No results for '…'"
  - Error → generic error message

### API client (`web/src/api.ts`)

New method:
```ts
search(q: string): Promise<{ episodes: EpisodeWithFeed[]; feeds: Feed[] }>
```

New type in `types.ts`:
```ts
interface EpisodeWithFeed extends Episode { FeedTitle: string }
```

## Testing

- Backend: unit test for `store.Search` covering: empty query, title match, description match, feed title match, no results
- Existing acceptance tests are unaffected (new endpoint only)
- Frontend: manual smoke test (React Query + debounce make unit testing non-trivial without additional test infrastructure)
