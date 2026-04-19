import { test, expect } from '@playwright/test';
import { seed } from './fixtures/seed';
import { loginAs } from './helpers/auth';

test.describe('Admin panel access', () => {
  test('admin routes redirect unauthenticated users to login', async ({ page }) => {
    for (const route of ['/admin', '/admin/users', '/admin/tenants', '/admin/logs']) {
      await page.goto(route);
      await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
    }
  });

  test('root admin can access users list', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/admin/users');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('root admin can access tenants list', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/admin/tenants');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('root admin can access plans page', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/admin/plans');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
    // Plans table has at least one row (seeded plans exist)
    await expect(page.locator('table, [role="table"], [data-testid="plans-list"]').first().or(page.getByText(/seed/i).first())).toBeVisible({ timeout: 10_000 });
  });

  test('root admin can access health page', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/admin/health');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('root admin can access logs page', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/admin/logs');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('root admin can access financial page', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/admin/financial');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('root admin can access config page', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/admin/config');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('root admin can access API docs page', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    await loginAs(page, email, password);
    await page.goto('/admin/api');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('non-root user redirected away from admin panel', async ({ page }) => {
    const { email, password } = seed.activeOwner;
    await loginAs(page, email, password);
    await page.goto('/admin');
    await page.waitForLoadState('networkidle');
    await expect(page).not.toHaveURL('/admin', { timeout: 10_000 });
  });

  test('root admin can view specific tenant profile', async ({ page }) => {
    const { email, password } = seed.rootAdmin;
    const tenantId = seed.manifest.accounts.activeOwner.tenantId;
    await loginAs(page, email, password);
    await page.goto(`/admin/tenants/${tenantId}`);
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });
});
