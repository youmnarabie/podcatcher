# Playwright E2E Search Tests Design

**Date:** 2026-03-17
**Project:** poo (podcast catcher)
**Status:** Approved

## Overview

Add Playwright E2E tests for the search bar feature. Tests run against a real Go server started via Playwright's `webServer` and a real Postgres database provided via `DATABASE_URL`. All 6 scenarios cover navigation, results rendering, and screenshots.

## Structure

```
e2e/
  playwright.config.ts   — Playwright config: webServer, baseURL, screenshots dir
  search.spec.ts         — All 6 search test scenarios
  screenshots/           — Screenshot output directory (gitignored)
```

The `e2e/` directory lives at the repo root. Tests are run with:
```bash
DATABASE_URL="postgres://..." npx playwright test --config e2e/playwright.config.ts
```
using the Playwright binary already installed in `web/node_modules`.

## Config (`e2e/playwright.config.ts`)

```ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  use: {
    baseURL: 'http://localhost:8080',
  },
  webServer: {
    command: 'go run ./cmd/server',
    url: 'http://localhost:8080',
    reuseExistingServer: !process.env.CI,
    env: {
      DATABASE_URL: process.env.DATABASE_URL ?? '',
      ADDR: ':8080',
    },
  },
});
```

- `reuseExistingServer: !process.env.CI` — locally, reuse a running server for speed; in CI, always start fresh
- `DATABASE_URL` must be set in the environment before running tests (same requirement as Go tests)

## Test File (`e2e/search.spec.ts`)

### Data setup

`test.beforeAll` seeds via direct HTTP API calls:
- `POST /api/v1/feeds` — creates a feed, captures `feed.ID`
- `POST /api/v1/feeds/:id/refresh` — not needed; episode is upserted directly
- Instead: use the store directly via Go test helpers — **no**, keep it HTTP-only for true E2E purity
- Seed by calling `POST /api/v1/feeds` to create the feed, then rely on a known RSS fixture — **also complex**

Simplest approach: seed via the API:
1. `POST /api/v1/feeds` → get `feedID`
2. The feed title isn't set until ingestion. Since we can't ingest a fake RSS feed in E2E, seed episode data directly by calling `POST` on a test-only endpoint — **no extra endpoints**.

Revised approach: seed via `store` package directly from a Go helper binary, or accept that feed title will be null and only test episode search. **Simpler:** use the existing `UpdateFeedMeta` indirectly — but that's internal.

**Practical solution:** The `PATCH /api/v1/feeds/:id` endpoint doesn't exist, but `POST /api/v1/feeds/:id/refresh` does (it triggers RSS ingestion). Since we can't stand up a real RSS server in E2E, we seed via the Go acceptance test pattern but exposed through a test-only API route — **too invasive**.

**Final approach:** Accept that feed title requires ingestion. For E2E purposes:
- Create the feed via `POST /api/v1/feeds`
- For the feed title match test, search by URL substring (feed URL is always set) — but `title ILIKE` won't match URL
- **Instead:** set `MIGRATIONS_PATH` to run a seed migration in test mode — **too complex**

**Simplest correct approach:** Run a small Go seed helper (`go run ./e2e/seed`) that uses the store package to seed a feed with title and an episode, outputting the IDs as JSON. `beforeAll` runs this helper and captures the IDs; `afterAll` runs a cleanup helper.

### Revised structure

```
e2e/
  playwright.config.ts
  search.spec.ts
  seed/
    main.go    — seeds feed+episode via store, prints JSON {feedID, episodeID}
  cleanup/
    main.go    — deletes feed by ID (cascades to episode)
  screenshots/
```

Seed helper (`e2e/seed/main.go`):
- Reads `DATABASE_URL` from env
- Creates feed with URL `https://e2e-test.example/feed.rss`
- Calls `UpdateFeedMeta` to set title `"E2E Test Show"`
- Upserts episode with title `"E2E Unique Episode"`, feed ID
- Prints `{"feedID": "...", "episodeID": "..."}` to stdout

Cleanup helper (`e2e/cleanup/main.go`):
- Reads `DATABASE_URL` and `FEED_ID` from env
- Calls `store.DeleteFeed` (cascades)

`search.spec.ts`:
- `beforeAll`: runs `go run ./e2e/seed` via `execSync`, parses JSON, stores `feedID`
- `afterAll`: runs `go run ./e2e/cleanup` with `FEED_ID=feedID`

### Test scenarios

1. **Typing navigates to `/search?q=...`**
   - Navigate to `/`
   - Type `"E2E"` into `input[type=search]`
   - Wait for URL to match `/search?q=E2E` (debounce: wait up to 1s)

2. **Episode results appear**
   - Navigate to `/search?q=E2E+Unique+Episode`
   - Wait for `text=E2E Unique Episode` to be visible
   - Assert `text=E2E Test Show` is visible (feed subtitle)

3. **Feed results appear**
   - Navigate to `/search?q=E2E+Test+Show`
   - Wait for `text=E2E Test Show` to be visible in the Shows section

4. **No results message**
   - Navigate to `/search?q=zzzNOTHINGzzz`
   - Wait for `text=No results` to be visible

5. **Clearing input navigates to `/`**
   - Navigate to `/search?q=E2E`
   - Clear the `input[type=search]`
   - Wait for URL to be `http://localhost:8080/` (debounce: wait up to 1s)

6. **Screenshot**
   - Navigate to `/search?q=E2E`
   - Wait for results to appear (`text=E2E Unique Episode`)
   - Take screenshot → `e2e/screenshots/search-results.png`

## Error handling

- If `DATABASE_URL` is not set, `webServer` will fail to start and Playwright reports a clear error
- If seed helper fails (e.g. DB not reachable), `beforeAll` throws and all tests are skipped
- Screenshots directory created by the test if it doesn't exist

## .gitignore

Add `e2e/screenshots/` to `.gitignore`.
