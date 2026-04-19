import { test, expect } from '@playwright/test';
import { seed } from './fixtures/seed';
import { loginAs } from './helpers/auth';

test.describe('Authentication flows', () => {
  test('login page renders correctly', async ({ page }) => {
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('input[type="email"]')).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible();
  });

  test('invalid credentials shows error', async ({ page }) => {
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await page.locator('input[type="email"]').fill('nonexistent@test.com');
    await page.locator('input[type="password"]').fill('WrongPassword1!');
    await page.getByRole('button', { name: /sign in/i }).click();
    // Accept either the bad-credentials message or the rate-limit message —
    // both mean the login was correctly rejected.
    await expect(
      page.getByText(/invalid email or password|rate limit|too many/i).first()
    ).toBeVisible({ timeout: 15_000 });
    // Must not navigate away from login
    await expect(page).toHaveURL(/\/login/);
  });

  test('protected pages redirect unauthenticated users to login', async ({ page }) => {
    for (const route of ['/dashboard', '/team', '/plan', '/settings']) {
      await page.goto(route);
      await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
    }
  });

  test('active user login reaches dashboard', async ({ page }) => {
    const { email, password } = seed.activeOwner;
    await loginAs(page, email, password);
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
  });

  test('root admin login reaches admin panel', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/last');
    await expect(page).toHaveURL(/\/last/, { timeout: 10_000 });
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('non-root user cannot access admin routes', async ({ page }) => {
    const { email, password } = seed.activeOwner;
    await loginAs(page, email, password);
    await page.goto('/last');
    await expect(page).not.toHaveURL('/last', { timeout: 10_000 });
  });

  test('signup form is accessible', async ({ page }) => {
    await page.goto('/signup');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('input[type="email"]')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole('button', { name: /sign up|create account/i })).toBeVisible();
  });

  test('login page links to signup', async ({ page }) => {
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('link', { name: /sign up/i })).toBeVisible({ timeout: 10_000 });
  });

  test('signup page links to login', async ({ page }) => {
    await page.goto('/signup');
    await page.waitForLoadState('networkidle');
    await expect(page.getByRole('link', { name: /sign in|log in/i })).toBeVisible({ timeout: 10_000 });
  });
});
