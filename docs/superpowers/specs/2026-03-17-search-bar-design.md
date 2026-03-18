# Search Bar Design

**Date:** 2026-03-17
**Project:** poo (podcast catcher)
**Status:** Approved

## Overview

Add a globally visible search bar to the nav that searches episodes and feeds (shows) as the user types. Results are served by a new dedicated `GET /api/search` endpoint.

## Backend

### New endpoint

`GET /api/search?q=<text>`

- Empty or missing `q` → return `{"episodes":[],"feeds":[]}` immediately, no DB query
- Otherwise, delegate to `store.Search`

Response shape:
```json
{
  "episodes": [{ "...Episode fields...", "feed_title": "string" }],
  "feeds":    [{ "...Feed fields..." }]
}
```

### New files

**`internal/store/search.go`**

Adds `Search(ctx context.Context, query string) (*SearchResult, error)` to `Store`.

The method runs two queries **concurrently** using `errgroup.WithContext` from `golang.org/x/sync/errgroup` (already in `go.mod`).

1. **Episodes query** — joins feeds to retrieve `feed_title`:
   ```sql
   SELECT e.id, e.feed_id, e.guid, e.title, e.description, e.audio_url,
          e.duration_seconds, e.published_at, e.raw_season, e.raw_episode_number,
          e.created_at, f.title AS feed_title
   FROM episodes e
   JOIN feeds f ON f.id = e.feed_id
   WHERE e.title ILIKE $1 OR e.description ILIKE $1
   ORDER BY e.published_at DESC NULLS LAST
   LIMIT 50
   ```

2. **Feeds query** (same column set as `ListFeeds`):
   ```sql
   SELECT id, url, title, description, image_url, poll_interval_seconds, last_fetched_at, created_at
   FROM feeds
   WHERE title ILIKE $1
   LIMIT 50
   ```

Both queries receive `'%' + query + '%'` as the `$1` argument (Go string concatenation before binding).

Types defined in `internal/store/search.go`. `Episode` is an exported **value type** (not a pointer) in `episodes.go` — embedding it directly in `EpisodeWithFeed` is safe and JSON-marshals correctly. `Store` is a concrete struct (no interface to update).

```go
type EpisodeWithFeed struct {
    Episode
    FeedTitle string `json:"feed_title"`
}

type SearchResult struct {
    Episodes []*EpisodeWithFeed `json:"episodes"`
    Feeds    []*Feed            `json:"feeds"`
}
```

**`internal/api/search.go`**

Handler method `(s *Server) search(w http.ResponseWriter, r *http.Request)`:
- Reads `q` from query params
- Empty `q` → `writeJSON(w, 200, map[string]any{"episodes": []any{}, "feeds": []any{}})`
- Calls `s.store.Search(r.Context(), q)`
- DB error → `writeError(w, 500, err.Error())`
- Success → `writeJSON(w, 200, result)`

Route registered inside the `/api/v1` subrouter in `server.go`, alongside existing routes:
```go
r.Get("/search", srv.search)
```
Full path: `GET /api/v1/search` — consistent with the axios `baseURL: '/api/v1'` in `api.ts`.

### Testing

**`internal/store/search_test.go`** — unit tests (requires a real DB connection, following the existing test pattern):
- Empty query → returns empty results without hitting DB
- Episode title match (seed one episode, search by partial title)
- Episode description match (seed one episode, search by partial description)
- Feed title match (seed one feed, search by partial title)
- No results (search for a string that matches nothing)

**`internal/api/acceptance_test.go`** — new test:
- Seeds one feed and one episode with a known title (e.g. `"UniqueSearchTerm"`)
- `GET /api/search?q=UniqueSearchTerm`
- Asserts 200, `episodes` array has length 1 with matching title, `feeds` array is empty (feed title doesn't match)

## Frontend

### SearchBar component (`web/src/components/SearchBar.tsx`)

- Controlled `<input>` rendered in the nav in `App.tsx`, placed after existing nav links
- On mount (and when location changes), initializes `inputValue` from `useSearchParams` so the input stays in sync with the URL (e.g. on page load or back-navigation)
- 300ms debounce: `useEffect` clears a `setTimeout` on each `inputValue` change, fires navigation after 300ms
- When debounced value is non-empty: `navigate('/search?q=' + encodeURIComponent(debouncedQ), { replace: true })`
- When input is cleared (value becomes empty): `navigate('/', { replace: true })`
- No loading/error state on the input itself — feedback is in the results page

### Route registration

In `App.tsx`, add to the `<Routes>`:
```tsx
<Route path="/search" element={<SearchResults onPlay={handlePlay} />} />
```

Also render `<SearchBar />` inside the `<nav>` in `App.tsx`.

### SearchResults page (`web/src/components/SearchResults.tsx`)

Props: `onPlay: (ep: Episode) => void`

- Reads `q` from URL search params via `useSearchParams`
- Skips fetch when `q` is empty (React Query `enabled: !!q`)
- Fetches `api.search(q)` via React Query (query key: `['search', q]`)
- `q` passed to `api.search` is the decoded value from `useSearchParams` — `api.search` re-encodes it for the HTTP request
- Renders two sections:
  - **Episodes** — reuses `EpisodeItem`, `feed_title` shown as subtitle context; passes `onPlay`
  - **Shows** — minimal feed card: name as a link to `/feeds/:id/episodes`
- States:
  - `q` empty → "Type to search"
  - Loading → "Searching…"
  - No results (both arrays empty) → `No results for "…"`
  - Error → "Search failed. Please try again."

### API client additions

**`web/src/api.ts`** — new method added to the `api` object (same pattern as existing methods):
```ts
search: (q: string) =>
  client.get<{ episodes: EpisodeWithFeed[]; feeds: Feed[] }>(`/search?q=${encodeURIComponent(q)}`).then(r => r.data),
```
`q` received here is the decoded value from `useSearchParams` — `encodeURIComponent` here is correct and there is no double-encoding (axios does not re-encode URL strings passed as raw string paths).

**`web/src/types.ts`** — new type:
```ts
export interface EpisodeWithFeed extends Episode {
  feed_title: string;
}
```
