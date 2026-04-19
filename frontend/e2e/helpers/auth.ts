import { type Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const MANIFEST_PATH = path.resolve(process.cwd(), '../seed-manifest.json');

type Manifest = {
  accounts: Record<string, { email: string; tenantId: string; accessToken?: string; refreshToken?: string }>;
};

let _manifest: Manifest | null = null;

function getManifest(): Manifest {
  if (_manifest) return _manifest;
  if (!fs.existsSync(MANIFEST_PATH)) {
    throw new Error(`seed-manifest.json not found. Run: npx playwright test (without SKIP_SEED=1)`);
  }
  _manifest = JSON.parse(fs.readFileSync(MANIFEST_PATH, 'utf-8')) as Manifest;
  return _manifest;
}

/**
 * Logs in by injecting pre-minted JWT tokens from seed-manifest.json into localStorage.
 * The seed command generates 24h tokens for every account — no login API calls
 * here, so parallel workers never trip the rate limiter.
 *
 * `email` must match the email of one of the seeded accounts.
 * `_password` is unused (kept for API symmetry with UI-based login helpers).
 */
export async function loginAs(page: Page, email: string, _password: string): Promise<void> {
  const manifest = getManifest();
  const entry = Object.values(manifest.accounts).find(a => a.email === email);
  if (!entry) throw new Error(`No seeded account with email: ${email}`);
  if (!entry.accessToken) throw new Error(`No token in manifest for ${email}. Re-run seed (without SKIP_SEED=1).`);

  await page.goto('/login');
  await page.waitForLoadState('domcontentloaded');

  await page.evaluate(({ accessToken, refreshToken, tenantId }) => {
    localStorage.setItem('saasquickstart_access_token', accessToken);
    localStorage.setItem('saasquickstart_refresh_token', refreshToken);
    localStorage.setItem('saasquickstart_active_tenant', tenantId);
  }, { accessToken: entry.accessToken, refreshToken: entry.refreshToken ?? '', tenantId: entry.tenantId });

  await page.goto('/dashboard');
  await page.waitForURL(/\/(dashboard|onboarding|last)/, { timeout: 15_000 });
}

export async function logout(page: Page): Promise<void> {
  await page.evaluate(() => {
    localStorage.clear();
    sessionStorage.clear();
  });
}
