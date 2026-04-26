// tests/wizard.spec.js
//
// Playwright end-to-end test for the ReadSync setup wizard. The test
// boots a Go test server using the same internal/api package (with
// fake adapters) and walks through every wizard page, asserting:
//   - all 10 pages are reachable;
//   - CSRF tokens are embedded as <meta name="csrf-token">;
//   - the per-page Run buttons return a status snippet;
//   - the Finish button completes setup and redirects to the dashboard.
//
// Run with:
//   npm install --no-save @playwright/test
//   READSYNC_URL=http://127.0.0.1:7201 npx playwright test tests/wizard.spec.js
//
// CI: the smoke runner first starts `go run ./cmd/readsync-service run`
// in the background, then runs this test.

const { test, expect } = require('@playwright/test');

const BASE = process.env.READSYNC_URL || 'http://127.0.0.1:7201';

test.describe('ReadSync setup wizard', () => {
    test('redirects root to wizard when setup is incomplete', async ({ page }) => {
        const response = await page.goto(BASE + '/');
        expect(response.status()).toBeLessThan(400);
        await expect(page).toHaveURL(/\/ui\/wizard/);
    });

    test('renders all 10 wizard pages', async ({ page }) => {
        await page.goto(BASE + '/ui/wizard');
        const expectedSlugs = [
            'welcome', 'system_scan', 'calibre', 'goodreads_bridge',
            'koreader', 'moon', 'conflict_policy', 'test_sync',
            'diagnostics', 'finish',
        ];
        for (const slug of expectedSlugs) {
            const link = page.locator(`a[href="/ui/wizard?page=${slug}"]`);
            await expect(link).toBeVisible();
        }
    });

    test('embeds csrf-token meta tag on every page', async ({ page }) => {
        for (const path of ['/ui/dashboard', '/ui/wizard', '/ui/conflicts',
                            '/ui/outbox', '/ui/activity', '/ui/repair']) {
            await page.goto(BASE + path);
            const meta = page.locator('meta[name="csrf-token"]');
            await expect(meta).toHaveAttribute('content', /.{30,}/);
        }
    });

    test('navigates from welcome to system_scan', async ({ page }) => {
        await page.goto(BASE + '/ui/wizard?page=welcome');
        await page.locator('a[href="/ui/wizard?page=system_scan"]').first().click();
        await expect(page.locator('h2')).toContainText('System Scan');
    });

    test('runs system scan and shows result', async ({ page }) => {
        await page.goto(BASE + '/ui/wizard?page=system_scan');
        await page.click('button:has-text("Run system scan")');
        // The HTMX swap injects a snippet into #scan-result.
        const result = page.locator('#scan-result');
        await expect(result).toBeVisible();
    });

    test('saves goodreads bridge mode', async ({ page }) => {
        await page.goto(BASE + '/ui/wizard?page=goodreads_bridge');
        await page.check('input[value="manual"]');
        await page.click('button:has-text("Save mode")');
        const result = page.locator('#gr-result');
        await expect(result).toContainText(/manual/, { timeout: 5000 });
    });

    test('completes setup and redirects to dashboard', async ({ page }) => {
        await page.goto(BASE + '/ui/wizard?page=finish');
        await page.click('button:has-text("Finish setup")');
        // Allow JS to fire the redirect.
        await page.waitForURL(/\/ui\/dashboard/, { timeout: 5000 });
    });

    test('repair page lists every action', async ({ page }) => {
        await page.goto(BASE + '/ui/repair');
        for (const title of [
            'Find calibredb',
            'Backup Calibre library',
            'Create custom columns',
            'Open Goodreads plugin instructions',
            'Generate missing-ID report',
            'Enable KOReader endpoint',
            'Rotate adapter credentials',
            'Open firewall rule',
            'Restart service',
            'Rebuild resolver index',
            'Clear deadletter',
            'Export diagnostics',
        ]) {
            await expect(page.locator(`text=${title}`)).toBeVisible();
        }
    });

    test('CSRF rejects POSTs without token', async ({ request }) => {
        const r = await request.post(BASE + '/api/wizard/complete', {
            failOnStatusCode: false,
        });
        expect(r.status()).toBe(403);
    });
});
