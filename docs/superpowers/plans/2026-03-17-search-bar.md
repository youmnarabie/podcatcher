# Search Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a globally visible, debounced search bar to the nav that searches episode titles/descriptions and feed titles via a dedicated `GET /api/v1/search` endpoint.

**Architecture:** New `store.Search` runs two concurrent Postgres ILIKE queries (episodes + feeds). A new `api.search` handler serves `GET /api/v1/search?q=`. The frontend adds a `SearchBar` input to the nav and a `SearchResults` route at `/search`.

**Tech Stack:** Go (chi, pgx, golang.org/x/sync/errgroup), React + TypeScript (React Query, React Router v6, axios)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/store/search.go` | Create | `Search` method + `EpisodeWithFeed`, `SearchResult` types |
| `internal/store/search_test.go` | Create | Store-level unit tests for `Search` |
| `internal/api/search.go` | Create | HTTP handler for `GET /api/v1/search` |
| `internal/api/server.go` | Modify | Register `/search` route inside `/api/v1` subrouter |
| `internal/api/acceptance_test.go` | Modify | Add `TestSearchEndpoint` acceptance test |
| `web/src/types.ts` | Modify | Add `EpisodeWithFeed` interface |
| `web/src/api.ts` | Modify | Add `search` method |
| `web/src/components/SearchBar.tsx` | Create | Debounced nav input with URL sync |
| `web/src/components/SearchResults.tsx` | Create | Results page with Episodes + Shows sections |
| `web/src/App.tsx` | Modify | Add `SearchBar` to nav, add `/search` route |

---

## Task 1: Store — `Search` types and method (TDD)

**Files:**
- Create: `internal/store/search.go`
- Create: `internal/store/search_test.go`

**Context:** The existing test pattern (in `internal/store/feeds_test.go`) uses a real DB via `TEST_DATABASE_URL` env var and skips if not set. `testStore(t)` helper already exists in that package.

All existing Go struct fields are PascalCase with no json tags — the JSON encoder outputs PascalCase keys. Follow the same convention for `FeedTitle`.

- [ ] **Step 1: Write the failing store tests**

Create `internal/store/search_test.go`:

```go
package store_test

import (
	"context"
	"testing"

	"github.com/youmnarabie/poo/internal/store"
)

func TestSearch_EmptyQuery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	result, err := s.Search(ctx, "")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Episodes) != 0 || len(result.Feeds) != 0 {
		t.Fatal("expected empty results for empty query")
	}
}

func TestSearch_EpisodeTitleMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://search-test-title.example/feed.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })
	_ = s.UpdateFeedMeta(ctx, feed.ID, "Test Show", "", "")

	ep, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:   feed.ID,
		GUID:     "search-test-title-1",
		Title:    "UniqueEpisodeTitleXYZ",
		AudioURL: "https://example.com/ep.mp3",
	})
	if err != nil {
		t.Fatalf("UpsertEpisode: %v", err)
	}
	_ = ep

	result, err := s.Search(ctx, "UniqueEpisodeTitleXYZ")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Episodes) == 0 {
		t.Fatal("expected at least one episode result")
	}
	if result.Episodes[0].Title != "UniqueEpisodeTitleXYZ" {
		t.Fatalf("expected title %q got %q", "UniqueEpisodeTitleXYZ", result.Episodes[0].Title)
	}
}

func TestSearch_EpisodeDescriptionMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://search-test-desc.example/feed.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })
	_ = s.UpdateFeedMeta(ctx, feed.ID, "Desc Show", "", "")

	desc := "UniqueDescriptionABC"
	_, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:      feed.ID,
		GUID:        "search-test-desc-1",
		Title:       "Some Episode",
		Description: &desc,
		AudioURL:    "https://example.com/ep2.mp3",
	})
	if err != nil {
		t.Fatalf("UpsertEpisode: %v", err)
	}

	result, err := s.Search(ctx, "UniqueDescriptionABC")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Episodes) == 0 {
		t.Fatal("expected at least one episode result from description match")
	}
}

func TestSearch_FeedTitleMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://search-test-feed.example/feed.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })
	_ = s.UpdateFeedMeta(ctx, feed.ID, "UniqueFeedTitleQQQ", "", "")

	result, err := s.Search(ctx, "UniqueFeedTitleQQQ")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Feeds) == 0 {
		t.Fatal("expected at least one feed result")
	}
	if *result.Feeds[0].Title != "UniqueFeedTitleQQQ" {
		t.Fatalf("expected feed title %q got %q", "UniqueFeedTitleQQQ", *result.Feeds[0].Title)
	}
}

func TestSearch_NoResults(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	result, err := s.Search(ctx, "zzzNOTHINGMATCHESTHISzzz")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Episodes) != 0 || len(result.Feeds) != 0 {
		t.Fatal("expected no results")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /path/to/repo && TEST_DATABASE_URL="postgres://..." go test ./internal/store/... -run TestSearch -v
```
Expected: `FAIL` — `s.Search undefined`

- [ ] **Step 3: Implement `internal/store/search.go`**

```go
package store

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type EpisodeWithFeed struct {
	Episode
	FeedTitle string
}

type SearchResult struct {
	Episodes []*EpisodeWithFeed
	Feeds    []*Feed
}

func (s *Store) Search(ctx context.Context, query string) (*SearchResult, error) {
	result := &SearchResult{
		Episodes: make([]*EpisodeWithFeed, 0),
		Feeds:    make([]*Feed, 0),
	}
	if query == "" {
		return result, nil
	}

	pattern := "%" + query + "%"
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		rows, err := s.db.Query(ctx, `
			SELECT e.id, e.feed_id, e.guid, e.title, e.description, e.audio_url,
			       e.duration_seconds, e.published_at, e.raw_season, e.raw_episode_number,
			       e.created_at, f.title
			FROM episodes e
			JOIN feeds f ON f.id = e.feed_id
			WHERE e.title ILIKE $1 OR e.description ILIKE $1
			ORDER BY e.published_at DESC NULLS LAST
			LIMIT 50`, pattern)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ewf EpisodeWithFeed
			if err := rows.Scan(
				&ewf.ID, &ewf.FeedID, &ewf.GUID, &ewf.Title, &ewf.Description,
				&ewf.AudioURL, &ewf.DurationSeconds, &ewf.PublishedAt,
				&ewf.RawSeason, &ewf.RawEpisodeNumber, &ewf.CreatedAt,
				&ewf.FeedTitle,
			); err != nil {
				return err
			}
			result.Episodes = append(result.Episodes, &ewf)
		}
		return rows.Err()
	})

	g.Go(func() error {
		rows, err := s.db.Query(ctx, `
			SELECT id, url, title, description, image_url, poll_interval_seconds, last_fetched_at, created_at
			FROM feeds
			WHERE title ILIKE $1
			LIMIT 50`, pattern)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f Feed
			if err := rows.Scan(
				&f.ID, &f.URL, &f.Title, &f.Description, &f.ImageURL,
				&f.PollIntervalSeconds, &f.LastFetchedAt, &f.CreatedAt,
			); err != nil {
				return err
			}
			result.Feeds = append(result.Feeds, &f)
		}
		return rows.Err()
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
TEST_DATABASE_URL="postgres://..." go test ./internal/store/... -run TestSearch -v
```
Expected: all 5 `TestSearch_*` tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/search.go internal/store/search_test.go
git commit -m "feat: add store.Search with concurrent episode+feed ILIKE queries"
```

---

## Task 2: API handler and route registration (TDD)

**Files:**
- Create: `internal/api/search.go`
- Modify: `internal/api/server.go` — add one line inside the `/api/v1` route block
- Modify: `internal/api/acceptance_test.go` — add `TestSearchEndpoint`

**Context:** All routes live inside `r.Route("/api/v1", func(r chi.Router) { ... })` in `server.go`. `writeJSON` and `writeError` helpers are defined in `server.go`. The acceptance test pattern seeds data directly via `store` methods, makes HTTP requests to `httptest.Server`, and decodes JSON responses into `map[string]any`.

- [ ] **Step 1: Write the failing acceptance test**

Append to `internal/api/acceptance_test.go`:

```go
func TestSearchEndpoint(t *testing.T) {
	ts, s := testServer(t)
	ctx := context.Background()

	feed, _ := s.CreateFeed(ctx, "https://search-acceptance.example/feed.rss")
	t.Cleanup(func() { s.DeleteFeed(ctx, feed.ID) })
	_ = s.UpdateFeedMeta(ctx, feed.ID, "Acceptance Show", "", "")

	_, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:   feed.ID,
		GUID:     "search-acceptance-ep-1",
		Title:    "UniqueAcceptanceTitleZZZ",
		AudioURL: "https://example.com/acc.mp3",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(ts.URL + "/api/v1/search?q=UniqueAcceptanceTitleZZZ")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	episodes, ok := result["Episodes"].([]any)
	if !ok || len(episodes) == 0 {
		t.Fatalf("expected episodes array with results, got: %v", result["Episodes"])
	}

	feeds, ok := result["Feeds"].([]any)
	if !ok {
		t.Fatal("expected feeds array in response")
	}
	if len(feeds) != 0 {
		t.Fatalf("expected 0 feed results (feed title doesn't match), got %d", len(feeds))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
TEST_DATABASE_URL="postgres://..." go test ./internal/api/... -run TestSearchEndpoint -v
```
Expected: FAIL — `404` or route not found

- [ ] **Step 3: Create `internal/api/search.go`**

`store.Search` already returns empty results for empty query, so the handler simply delegates for all cases:

```go
package api

import "net/http"

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	result, err := s.store.Search(r.Context(), q)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, result)
}
```

- [ ] **Step 4: Register the route in `internal/api/server.go`**

Inside the `r.Route("/api/v1", func(r chi.Router) { ... })` block, add after the `opml` lines:

```go
r.Get("/search", srv.search)
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
TEST_DATABASE_URL="postgres://..." go test ./internal/api/... -run TestSearchEndpoint -v
```
Expected: PASS

Also run the full test suite to confirm no regressions:
```bash
TEST_DATABASE_URL="postgres://..." go test ./... -v
```
Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/search.go internal/api/server.go internal/api/acceptance_test.go
git commit -m "feat: add GET /api/v1/search endpoint"
```

---

## Task 3: Frontend types and API client

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

**Context:** `types.ts` uses PascalCase field names throughout (matching Go's default JSON output). `api.ts` uses an axios instance with `baseURL: '/api/v1'`. All methods follow the pattern `client.get<T>(path).then(r => r.data)`. The new `EpisodeWithFeed` adds `FeedTitle` (PascalCase, matching Go's output for `FeedTitle string` with no json tag).

- [ ] **Step 1: Add `EpisodeWithFeed` to `web/src/types.ts`**

Append to `web/src/types.ts`:

```ts
export interface EpisodeWithFeed extends Episode {
  FeedTitle: string;
}
```

- [ ] **Step 2: Add `search` method to `web/src/api.ts`**

Add to the `api` object (after `exportOPML`):

```ts
search: (q: string) =>
  client.get<{ Episodes: EpisodeWithFeed[]; Feeds: Feed[] }>(`/search?q=${encodeURIComponent(q)}`).then(r => r.data),
```

Keys are PascalCase (`Episodes`, `Feeds`) matching Go's default JSON output.

Also add `EpisodeWithFeed` to the import at the top of `api.ts`:

```ts
import type { Episode, EpisodeListParams, EpisodeWithFeed, Feed, FeedRule, PlaybackState, Series } from './types';
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd web && npm run build 2>&1 | head -30
```
Expected: no TypeScript errors (build may warn about other things but no type errors on the new code)

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: add EpisodeWithFeed type and api.search client method"
```

---

## Task 4: `SearchResults` component

**Files:**
- Create: `web/src/components/SearchResults.tsx`

**Context:** `EpisodeItem` takes `{ episode: Episode; onPlay?: (ep: Episode) => void }`. Since `EpisodeWithFeed extends Episode`, it can be passed directly to `EpisodeItem`. The `Feed` type has `ID`, `Title` (nullable), `URL`. React Query is imported from `@tanstack/react-query`. `useSearchParams` comes from `react-router-dom`.

- [ ] **Step 1: Create `web/src/components/SearchResults.tsx`**

```tsx
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api';
import type { Episode } from '../types';
import EpisodeItem from './EpisodeItem';

interface Props { onPlay: (ep: Episode) => void; }

export default function SearchResults({ onPlay }: Props) {
  const [searchParams] = useSearchParams();
  const q = searchParams.get('q') ?? '';

  const { data, isLoading, isError } = useQuery({
    queryKey: ['search', q],
    queryFn: () => api.search(q),
    enabled: !!q,
  });

  if (!q) return <p>Type to search</p>;
  if (isLoading) return <p>Searching…</p>;
  if (isError) return <p>Search failed. Please try again.</p>;

  const episodes = data?.Episodes ?? [];
  const feeds = data?.Feeds ?? [];
  const noResults = episodes.length === 0 && feeds.length === 0;

  return (
    <div>
      {noResults && <p>No results for &ldquo;{q}&rdquo;</p>}

      {episodes.length > 0 && (
        <section>
          <h3>Episodes</h3>
          {episodes.map(ep => (
            <div key={ep.ID}>
              <small style={{ color: '#888' }}>{ep.FeedTitle}</small>
              <EpisodeItem episode={ep} onPlay={onPlay} />
            </div>
          ))}
        </section>
      )}

      {feeds.length > 0 && (
        <section>
          <h3>Shows</h3>
          {feeds.map(f => (
            <div key={f.ID} style={{ borderBottom: '1px solid #eee', padding: '8px 0' }}>
              <a href={`/feeds/${f.ID}/episodes`}>{f.Title ?? f.URL}</a>
            </div>
          ))}
        </section>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npm run build 2>&1 | head -30
```
Expected: no type errors

- [ ] **Step 3: Commit**

```bash
git add web/src/components/SearchResults.tsx
git commit -m "feat: add SearchResults component"
```

---

## Task 5: `SearchBar` component

**Files:**
- Create: `web/src/components/SearchBar.tsx`

**Context:** `useNavigate` and `useSearchParams` from `react-router-dom`. The bar navigates to `/search?q=<encoded>` with `{ replace: true }` to avoid polluting browser history with every keystroke. It reads its initial value from the URL so that on page load (or back-navigation to `/search?q=foo`) the input shows the right text.

- [ ] **Step 1: Create `web/src/components/SearchBar.tsx`**

```tsx
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';

export default function SearchBar() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [value, setValue] = useState(() => searchParams.get('q') ?? '');

  // Keep input in sync when URL changes (e.g. browser back/forward)
  useEffect(() => {
    setValue(searchParams.get('q') ?? '');
  }, [searchParams]);

  // Debounced navigation
  useEffect(() => {
    const timer = setTimeout(() => {
      if (value) {
        navigate('/search?q=' + encodeURIComponent(value), { replace: true });
      } else {
        navigate('/', { replace: true });
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [value, navigate]);

  return (
    <input
      type="search"
      placeholder="Search episodes & shows…"
      value={value}
      onChange={e => setValue(e.target.value)}
      style={{ marginLeft: 'auto' }}
    />
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npm run build 2>&1 | head -30
```
Expected: no type errors

- [ ] **Step 3: Commit**

```bash
git add web/src/components/SearchBar.tsx
git commit -m "feat: add SearchBar component with debounce and URL sync"
```

---

## Task 6: Wire everything into `App.tsx`

**Files:**
- Modify: `web/src/App.tsx`

**Context:** `App.tsx` imports components and renders the nav + Routes. Add `SearchBar` to the nav and a `/search` route pointing to `SearchResults`. Use a `<Link>` component from react-router-dom for the feed cards in `SearchResults` (already done in Task 4 with `<a>` — acceptable for now).

- [ ] **Step 1: Update `web/src/App.tsx`**

Add imports at the top:
```tsx
import SearchBar from './components/SearchBar';
import SearchResults from './components/SearchResults';
```

Add `<SearchBar />` at the end of the `<nav>` block:
```tsx
<nav style={{ marginBottom: 16, display: 'flex', gap: 16 }}>
  <NavLink to="/">All Episodes</NavLink>
  <NavLink to="/feeds">Shows</NavLink>
  <NavLink to="/unplayed">Unplayed</NavLink>
  <SearchBar />
</nav>
```

Add a new route inside `<Routes>`:
```tsx
<Route path="/search" element={<SearchResults onPlay={handlePlay} />} />
```

- [ ] **Step 2: Build the frontend**

```bash
cd web && npm run build 2>&1 | head -50
```
Expected: build succeeds with no TypeScript errors

- [ ] **Step 3: Smoke test manually (optional, requires running server)**

```bash
# In one terminal: start the server
./podcatcher

# In another: verify the search endpoint works
curl "http://localhost:8080/api/v1/search?q=test"
# Expected: {"Episodes":[],"Feeds":[]} (or real results if DB has data)
```

- [ ] **Step 4: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat: wire SearchBar and SearchResults into App"
```

---

## Task 7: Final integration check and push

- [ ] **Step 1: Run all Go tests**

```bash
TEST_DATABASE_URL="postgres://..." go test ./... -v
```
Expected: all tests PASS

- [ ] **Step 2: Build frontend**

```bash
cd web && npm run build
```
Expected: clean build

- [ ] **Step 3: Push**

```bash
git push
```
