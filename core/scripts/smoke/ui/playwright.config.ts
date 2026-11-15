import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  timeout: 120_000,
  use: {
    headless: true,
    baseURL: 'http://127.0.0.1:18789',
  },
  reporter: [['list']],
});
