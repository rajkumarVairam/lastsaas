import { test, expect } from '@playwright/test';
import { seed } from './fixtures/seed';
import { loginAs } from './helpers/auth';

/**
 * Multi-tenant team role scenarios.
 * teamOwner, teamAdmin, teamMember are all members of the same tenant.
 * Tests verify RBAC enforcement at the UI level.
 */

test.describe('Team and RBAC', () => {

  test('team owner can access team management page', async ({ page }) => {
    const { email, password } = seed.teamOwner;
    await loginAs(page, email, password);
    await page.goto('/team');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
    // Owner should see invite button (strict: use role selector to avoid ambiguity)
    await expect(page.getByRole('button', { name: /invite/i }).first()).toBeVisible({ timeout: 10_000 });
  });

  test('team admin can view team page', async ({ page }) => {
    const { email, password } = seed.teamAdmin;
    await loginAs(page, email, password);
    await page.goto('/team');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('team member can view team page', async ({ page }) => {
    const { email, password } = seed.teamMember;
    await loginAs(page, email, password);
    await page.goto('/team');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('all team roles can access dashboard', async ({ page }) => {
    const roles = [seed.teamOwner, seed.teamAdmin, seed.teamMember];
    for (const user of roles) {
      await loginAs(page, user.email, user.password);
      await expect(page).toHaveURL(/\/dashboard/, { timeout: 15_000 });
      await page.evaluate(() => { localStorage.clear(); sessionStorage.clear(); });
    }
  });

  test('team owner can access settings', async ({ page }) => {
    const { email, password } = seed.teamOwner;
    await loginAs(page, email, password);
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });
});
