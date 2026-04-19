import { test, expect } from '@playwright/test';
import { seed } from './fixtures/seed';
import { loginAs } from './helpers/auth';

/**
 * Quick smoke tests — the first things to check if something breaks.
 * These run fast and cover the highest-value paths.
 */
test.describe('Smoke tests', () => {
  test('app loads at root URL', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/(login|setup)?$/);
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('login page is accessible', async ({ page }) => {
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('input[type="email"]')).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('input[type="password"]')).toBeVisible({ timeout: 10_000 });
  });

  test('signup page is accessible', async ({ page }) => {
    await page.goto('/signup');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('input[type="email"]')).toBeVisible({ timeout: 10_000 });
  });

  test('full login → dashboard → logout flow for active user', async ({ page }) => {
    const { email, password } = seed.activeOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
    // Logout via storage clear (simulates session expiry)
    await page.evaluate(() => { localStorage.clear(); sessionStorage.clear(); });
    await page.goto('/dashboard');
    await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
  });

  test('admin login → admin panel flow', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/last');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });
});
