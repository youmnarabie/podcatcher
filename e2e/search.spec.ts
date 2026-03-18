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
