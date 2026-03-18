import { defineConfig } from '@playwright/test';
import * as path from 'path';

export default defineConfig({
  testDir: '.',
  use: {
    baseURL: 'http://localhost:8080',
  },
  webServer: {
    command: 'go run ./cmd/server',
    cwd: path.join(__dirname, '..'),
    url: 'http://localhost:8080',
    reuseExistingServer: !process.env.CI,
    env: {
      DATABASE_URL: process.env.DATABASE_URL ?? '',
      ADDR: ':8080',
      MIGRATIONS_PATH: 'migrations',
    },
  },
});
