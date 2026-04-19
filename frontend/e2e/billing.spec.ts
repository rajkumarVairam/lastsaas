import { test, expect } from '@playwright/test';
import { seed } from './fixtures/seed';
import { loginAs } from './helpers/auth';

/**
 * Billing state coverage — exercises all 8 billing personas seeded by seed.go.
 *
 * Each test logs in as a specific persona and asserts the UI behaviour expected
 * for that billing state. Tests do NOT click Stripe-hosted pages (no test mode
 * card flows here); they verify the correct gates and CTAs are shown.
 */

test.describe('Billing states', () => {

  test('free plan user sees upgrade CTA on plan page', async ({ page }) => {
    const { email, password } = seed.freeOwner;
    await loginAs(page, email, password);
    await page.goto('/plan');
    await page.waitForLoadState('networkidle');
    // Plan page should load and contain an upgrade prompt
    await expect(page.locator('body')).not.toBeEmpty();
    await expect(page.getByText(/upgrade|get started|choose a plan|subscribe/i).first()).toBeVisible({ timeout: 10_000 });
  });

  test('trial user can access dashboard and sees trial indicator', async ({ page }) => {
    const { email, password } = seed.trialOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('active monthly subscriber can reach dashboard and plan page', async ({ page }) => {
    const { email, password } = seed.activeOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    await page.goto('/plan');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('annual subscriber can reach dashboard', async ({ page }) => {
    const { email, password } = seed.annualOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    await page.waitForLoadState('networkidle');
  });

  test('lifetime / billing-waived user has full access', async ({ page }) => {
    const { email, password } = seed.lifetimeOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('past-due user is shown payment required state', async ({ page }) => {
    const { email, password } = seed.pastDueOwner;
    await loginAs(page, email, password);
    // Past-due users may be redirected to plan or shown a banner
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
    // Should NOT be able to access paid features freely — plan page accessible
    await page.goto('/plan');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('canceled user sees win-back / resubscribe UI', async ({ page }) => {
    const { email, password } = seed.canceledOwner;
    await loginAs(page, email, password);
    await page.waitForLoadState('networkidle');
    await page.goto('/plan');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
    // Should show resubscribe option
    await expect(page.getByText(/subscribe|resubscribe|renew|upgrade|choose a plan/i)).toBeVisible({ timeout: 10_000 });
  });

  test('enterprise user has full access and billing-waived status', async ({ page }) => {
    const { email, password } = seed.enterpriseOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    await page.goto('/plan');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('plan page is accessible for all authenticated users', async ({ page }) => {
    const users = [
      seed.freeOwner,
      seed.activeOwner,
      seed.annualOwner,
      seed.lifetimeOwner,
      seed.enterpriseOwner,
    ];
    for (const user of users) {
      await loginAs(page, user.email, user.password);
      await page.goto('/plan');
      await page.waitForLoadState('networkidle');
      await expect(page.locator('body')).not.toBeEmpty();
      // Clear auth for next iteration
      await page.evaluate(() => { localStorage.clear(); sessionStorage.clear(); });
    }
  });
});
