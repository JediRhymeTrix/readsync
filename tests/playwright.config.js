// tests/playwright.config.js
const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
    testDir: '.',
    timeout: 30_000,
    retries: 0,
    use: {
        actionTimeout: 5_000,
        ignoreHTTPSErrors: true,
        viewport: { width: 1200, height: 800 },
    },
    reporter: [['list']],
});
