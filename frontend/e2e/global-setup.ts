import { execSync } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';

const ROOT_DIR = path.resolve(process.cwd(), '..');
const MANIFEST_PATH = path.join(ROOT_DIR, 'seed-manifest.json');

/**
 * Runs before the Playwright suite. Seeds the database (unless SKIP_SEED=1).
 * The seed command mints a 24h JWT for every account and writes them into
 * seed-manifest.json — no login API calls needed during the test run.
 */
async function globalSetup() {
  if (process.env.SKIP_SEED === '1') {
    console.log('[seed] SKIP_SEED=1 — skipping seed, using existing manifest.');
    return;
  }

  const backendDir = path.join(ROOT_DIR, 'backend');
  console.log('[seed] Running seed command...');
  try {
    execSync(
      `go run ./cmd/saasquickstart seed --reset --output "${MANIFEST_PATH}"`,
      {
        cwd: backendDir,
        stdio: 'inherit',
        env: { ...process.env, LASTSAAS_ENV: process.env.LASTSAAS_ENV ?? 'dev' },
      }
    );
  } catch (err) {
    console.error('[seed] Seed command failed:', err);
    throw err;
  }

  if (!fs.existsSync(MANIFEST_PATH)) {
    throw new Error(`[seed] Manifest not found at ${MANIFEST_PATH} after seed run.`);
  }
  console.log(`[seed] Manifest ready at ${MANIFEST_PATH}`);
}

export default globalSetup;
