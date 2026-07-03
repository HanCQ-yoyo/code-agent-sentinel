import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir: './tests',
  use: { baseURL: 'http://127.0.0.1:41999' },
  webServer: {
    command: '../bin/sentinel --bind 127.0.0.1 --port 41999 --no-browser --token e2e-test-token-123 --home /tmp/sentinel-e2e-home',
    port: 41999, reuseExistingServer: true, timeout: 30000,
  },
})
