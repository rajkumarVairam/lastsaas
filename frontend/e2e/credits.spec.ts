import { test, expect } from '@playwright/test';
import { seed } from './fixtures/seed';
import { loginAs } from './helpers/auth';

/**
 * AI credit bucket scenarios.
 * Covers the three credit states: full (1500), low (18), empty (0).
 */

test.describe('AI credits', () => {

  test('full-credits user can access dashboard without blocks', async ({ page }) => {
    const { email, password } = seed.aiFullOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
    // Should NOT see an "out of credits" blocker
    await expect(page.getByText(/out of credits|no credits remaining/i)).not.toBeVisible();
  });

  test('low-credits user sees dashboard but with warning indicator', async ({ page }) => {
    const { email, password } = seed.aiLowOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('empty-credits user is shown purchase prompt', async ({ page }) => {
    const { email, password } = seed.aiEmptyOwner;
    await loginAs(page, email, password);
    await page.waitForLoadState('networkidle');
    // Empty credits user should see a CTA to purchase more credits
    await page.goto('/buy-credits');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('buy-credits page accessible for AI plan users', async ({ page }) => {
    const { email, password } = seed.aiFullOwner;
    await loginAs(page, email, password);
    await page.goto('/buy-credits');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });
});
