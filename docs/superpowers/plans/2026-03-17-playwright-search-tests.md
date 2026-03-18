# Playwright E2E Search Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Playwright E2E tests for the search bar feature covering navigation, episode results, feed results, no-results, clear behaviour, and a screenshot.

**Architecture:** A Go seed helper seeds a feed+episode via the store package; a cleanup helper deletes them. `playwright.config.ts` starts the Go server via `webServer`. `search.spec.ts` runs 6 tests using `beforeAll`/`afterAll` to manage test data.

**Tech Stack:** Go (store package), TypeScript (`@playwright/test` from `web/node_modules`), Playwright browser automation

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `e2e/seed/main.go` | Create | Seeds feed + episode, prints `{"feedID":"...","episodeID":"..."}` |
| `e2e/cleanup/main.go` | Create | Deletes feed by `FEED_ID` env var (cascades to episode) |
| `e2e/playwright.config.ts` | Create | Playwright config: webServer, baseURL |
| `e2e/search.spec.ts` | Create | 6 E2E test scenarios |
| `.gitignore` | Modify | Add `e2e/screenshots/` |

---

## Task 1: Go seed and cleanup helpers

**Files:**
- Create: `e2e/seed/main.go`
- Create: `e2e/cleanup/main.go`

**Context:** These are standalone Go programs that use the `store` package to set up and tear down test data. They must be in separate packages so `go run` can target them individually. The store package is at `github.com/youmnarabie/poo/internal/store`. `store.New` requires a context and `DATABASE_URL`. `store.DeleteFeed` cascades to episodes.

- [ ] **Step 1: Create `e2e/seed/main.go`**

```go
// e2e/seed/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/youmnarabie/poo/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}

	ctx := context.Background()
	s, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	feed, err := s.CreateFeed(ctx, "https://e2e-test.example/feed.rss")
	if err != nil {
		log.Fatalf("CreateFeed: %v", err)
	}

	if err := s.UpdateFeedMeta(ctx, feed.ID, "E2E Test Show", "", ""); err != nil {
		log.Fatalf("UpdateFeedMeta: %v", err)
	}

	ep, err := s.UpsertEpisode(ctx, &store.Episode{
		FeedID:   feed.ID,
		GUID:     "e2e-ep-1",
		Title:    "E2E Unique Episode",
		AudioURL: "https://e2e-test.example/ep.mp3",
	})
	if err != nil {
		log.Fatalf("UpsertEpisode: %v", err)
	}

	out, _ := json.Marshal(map[string]string{
		"feedID":    feed.ID,
		"episodeID": ep.ID,
	})
	fmt.Println(string(out))
}
```

- [ ] **Step 2: Create `e2e/cleanup/main.go`**

```go
// e2e/cleanup/main.go
package main

import (
	"context"
	"log"
	"os"

	"github.com/youmnarabie/poo/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	feedID := os.Getenv("FEED_ID")
	if dbURL == "" || feedID == "" {
		log.Fatal("DATABASE_URL and FEED_ID required")
	}

	ctx := context.Background()
	s, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("store.New: %v", err)
	}
	defer s.Close()

	if err := s.DeleteFeed(ctx, feedID); err != nil {
		log.Fatalf("DeleteFeed: %v", err)
	}
}
```

- [ ] **Step 3: Verify both helpers compile**

```bash
go build ./e2e/seed && go build ./e2e/cleanup && echo "OK"
```
Expected: `OK` with no errors. Remove the built binaries afterward (`rm seed cleanup`).

- [ ] **Step 4: Commit**

```bash
git add e2e/seed/main.go e2e/cleanup/main.go
git commit -m "feat: add E2E seed and cleanup Go helpers"
```

---

## Task 2: Playwright config

**Files:**
- Create: `e2e/playwright.config.ts`
- Modify: `.gitignore`

**Context:** `@playwright/test` is installed in `web/node_modules`. The config lives in `e2e/` but Playwright is invoked from the repo root: `DATABASE_URL=... npx playwright test --config e2e/playwright.config.ts`. `testDir: '.'` means Playwright looks for `*.spec.ts` files inside `e2e/`. The `webServer` starts `go run ./cmd/server` from the repo root with the required env vars.

- [ ] **Step 1: Create `e2e/playwright.config.ts`**

```ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
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
      MIGRATIONS_PATH: 'migrations',
    },
  },
});
```

- [ ] **Step 2: Add `e2e/screenshots/` to `.gitignore`**

Append to `.gitignore` (or create it if absent):
```
e2e/screenshots/
```

- [ ] **Step 3: Verify config is valid TypeScript**

```bash
cd e2e && npx --prefix ../web tsc --noEmit playwright.config.ts 2>&1 || true
```
Note: this may warn about missing types — that's OK as long as there are no syntax errors. The real validation is in Task 4 when the tests run.

- [ ] **Step 4: Commit**

```bash
git add e2e/playwright.config.ts .gitignore
git commit -m "feat: add Playwright config for E2E search tests"
```

---

## Task 3: `search.spec.ts`

**Files:**
- Create: `e2e/search.spec.ts`
- Create: `e2e/screenshots/` (directory — created at runtime by the test, not committed)

**Context:**
- `execSync` from Node's `child_process` module runs the seed/cleanup Go helpers synchronously
- `beforeAll` seeds once for the whole file; `afterAll` cleans up
- The search input selector is `input[type=search]` (from `SearchBar.tsx` which renders `<input type="search">`)
- The 300ms debounce means tests must wait after typing — use `page.waitForURL` with a generous timeout
- `page.fill` clears and fills; `page.locator('input[type=search]').clear()` clears without typing
- Screenshots go to `e2e/screenshots/search-results.png`; `fs.mkdirSync` ensures the directory exists

- [ ] **Step 1: Create `e2e/search.spec.ts`**

```ts
import { test, expect } from '@playwright/test';
import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

let feedID: string;

test.beforeAll(() => {
  const out = execSync('go run ./e2e/seed').toString().trim();
  const data = JSON.parse(out);
  feedID = data.feedID;
});

test.afterAll(() => {
  execSync(`FEED_ID=${feedID} go run ./e2e/cleanup`);
});

test('typing in search bar navigates to /search', async ({ page }) => {
  await page.goto('/');
  await page.locator('input[type=search]').fill('E2E');
  await page.waitForURL('**/search?q=E2E', { timeout: 2000 });
  expect(page.url()).toContain('/search?q=E2E');
});

test('episode results appear with feed subtitle', async ({ page }) => {
  await page.goto('/search?q=E2E+Unique+Episode');
  await expect(page.getByText('E2E Unique Episode')).toBeVisible();
  await expect(page.getByText('E2E Test Show')).toBeVisible();
});

test('feed results appear in Shows section', async ({ page }) => {
  await page.goto('/search?q=E2E+Test+Show');
  await expect(page.getByText('E2E Test Show')).toBeVisible();
});

test('no results message appears when nothing matches', async ({ page }) => {
  await page.goto('/search?q=zzzNOTHINGzzz');
  await expect(page.getByText('No results')).toBeVisible();
});

test('clearing search input navigates back to /', async ({ page }) => {
  await page.goto('/search?q=E2E');
  await expect(page.getByText('E2E Unique Episode')).toBeVisible();
  await page.locator('input[type=search]').clear();
  await page.waitForURL('http://localhost:8080/', { timeout: 2000 });
  expect(page.url()).toBe('http://localhost:8080/');
});

test('screenshot of search results', async ({ page }) => {
  const screenshotsDir = path.join(__dirname, 'screenshots');
  fs.mkdirSync(screenshotsDir, { recursive: true });

  await page.goto('/search?q=E2E');
  await expect(page.getByText('E2E Unique Episode')).toBeVisible();
  await page.screenshot({ path: path.join(screenshotsDir, 'search-results.png') });
});
```

- [ ] **Step 2: Commit**

```bash
git add e2e/search.spec.ts
git commit -m "feat: add Playwright E2E search tests"
```

---

## Task 4: Run the tests

**Context:** Requires `DATABASE_URL` pointing to a running Postgres instance with migrations applied. The `go run ./cmd/server` cold start takes 3-5 seconds; Playwright waits until port 8080 responds. Run from the **repo root**.

- [ ] **Step 1: Run the tests**

```bash
DATABASE_URL="postgres://..." npx --prefix web playwright test --config e2e/playwright.config.ts --reporter=list
```
Expected: 6 tests pass. `e2e/screenshots/search-results.png` is created.

If `DATABASE_URL` is not set locally, tests will be skipped by the server failing to start — that's acceptable for environments without a DB. CI should set `DATABASE_URL`.

- [ ] **Step 2: Install Playwright browsers if needed**

If the above fails with "browser not found":
```bash
npx --prefix web playwright install chromium
```
Then re-run Step 1.

- [ ] **Step 3: Commit screenshot if desired**

If you want to commit the baseline screenshot:
```bash
# Remove e2e/screenshots/ from .gitignore first (or add an exception)
git add e2e/screenshots/search-results.png
git commit -m "test: add baseline search results screenshot"
```
This step is optional — screenshots are gitignored by default.

- [ ] **Step 4: Push**

```bash
git push
```
