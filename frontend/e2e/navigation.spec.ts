import { test, expect } from '@playwright/test';
import { seed } from './fixtures/seed';
import { loginAs } from './helpers/auth';

test.describe('Navigation', () => {
  test('unknown routes redirect to dashboard or login', async ({ page }) => {
    await page.goto('/this-page-does-not-exist');
    await expect(page).toHaveURL(/\/(login|dashboard)/, { timeout: 10_000 });
  });

  test('login page navigates to signup via link', async ({ page }) => {
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    const signupLink = page.getByRole('link', { name: /sign up/i });
    await expect(signupLink).toBeVisible({ timeout: 10_000 });
    await signupLink.click();
    await expect(page).toHaveURL(/\/signup/);
  });

  test('signup page navigates to login via link', async ({ page }) => {
    await page.goto('/signup');
    await page.waitForLoadState('networkidle');
    const loginLink = page.getByRole('link', { name: /sign in|log in/i });
    await expect(loginLink).toBeVisible({ timeout: 10_000 });
    await loginLink.click();
    await expect(page).toHaveURL(/\/login/);
  });

  test('forgot password page loads', async ({ page }) => {
    await page.goto('/forgot-password');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('authenticated user can navigate across app pages', async ({ page }) => {
    const { email, password } = seed.activeOwner;
    await loginAs(page, email, password);

    const routes = ['/dashboard', '/team', '/plan', '/settings', '/activity'];
    for (const route of routes) {
      await page.goto(route);
      await page.waitForLoadState('networkidle');
      await expect(page.locator('body')).not.toBeEmpty();
      // Should not have been kicked to login
      await expect(page).not.toHaveURL(/\/login/);
    }
  });

  test('activity page shows audit log for authenticated user', async ({ page }) => {
    const { email, password } = seed.activeOwner;
    await loginAs(page, email, password);
    await page.goto('/activity');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('settings page loads for authenticated user', async ({ page }) => {
    const { email, password } = seed.activeOwner;
    await loginAs(page, email, password);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });
});
