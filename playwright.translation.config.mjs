import { defineConfig, devices } from '@playwright/test'
import base from './playwright.config.mjs'

export default defineConfig({
  ...base,
  workers: 1,
  retries: 0,
  projects: [
    { name: 'chrome', use: { ...devices['Desktop Chrome'], channel: 'chrome', permissions: ['local-network-access'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
})
