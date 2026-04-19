import * as fs from 'fs';
import * as path from 'path';

interface Account {
  email: string;
  password: string;
  userId: string;
  tenantId: string;
  apiKey?: string;
}

interface SeedManifest {
  seededAt: string;
  password: string;
  accounts: {
    rootAdmin: Account;
    freeOwner: Account;
    trialOwner: Account;
    activeOwner: Account;
    annualOwner: Account;
    lifetimeOwner: Account;
    pastDueOwner: Account;
    canceledOwner: Account;
    enterpriseOwner: Account;
    teamOwner: Account;
    teamAdmin: Account;
    teamMember: Account;
    aiFullOwner: Account;
    aiLowOwner: Account;
    aiEmptyOwner: Account;
    apiKeyOwner: Account;
  };
  plans: {
    free: string;
    pro: string;
    pro_annual: string;
    enterprise: string;
    ai_credits: string;
  };
  jobs: {
    pending: string;
    running: string;
    completed: string;
    failed: string;
    dead: string;
    cancelled: string;
  };
  tenants: Record<string, string>;
}

let _manifest: SeedManifest | null = null;

export function getSeedManifest(): SeedManifest {
  if (_manifest) return _manifest;

  const manifestPath = path.resolve(process.cwd(), '../seed-manifest.json');
  if (!fs.existsSync(manifestPath)) {
    throw new Error(
      `seed-manifest.json not found at ${manifestPath}.\n` +
      `Run: cd backend && go run ./cmd/saasquickstart seed`
    );
  }

  _manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf-8')) as SeedManifest;
  return _manifest;
}

/** Shorthand helpers — use these in tests instead of hardcoding. */
export const seed = {
  get manifest() { return getSeedManifest(); },

  /** Root-tenant admin — can access all /api/admin/* routes */
  get rootAdmin() { return getSeedManifest().accounts.rootAdmin; },

  /** Free plan owner — hits entitlement gates, no paid features */
  get freeOwner() { return getSeedManifest().accounts.freeOwner; },

  /** Trial user — active billing but trialUsedAt set */
  get trialOwner() { return getSeedManifest().accounts.trialOwner; },

  /** Active monthly subscription — full Pro access */
  get activeOwner() { return getSeedManifest().accounts.activeOwner; },

  /** Active annual subscription */
  get annualOwner() { return getSeedManifest().accounts.annualOwner; },

  /** Lifetime / billing waived — bypasses all billing checks */
  get lifetimeOwner() { return getSeedManifest().accounts.lifetimeOwner; },

  /** Past due — RequireActiveBilling returns 402 */
  get pastDueOwner() { return getSeedManifest().accounts.pastDueOwner; },

  /** Canceled — tests post-cancellation and win-back UI */
  get canceledOwner() { return getSeedManifest().accounts.canceledOwner; },

  /** Enterprise — billing waived, all entitlements enabled */
  get enterpriseOwner() { return getSeedManifest().accounts.enterpriseOwner; },

  /** Team tenant owner */
  get teamOwner() { return getSeedManifest().accounts.teamOwner; },

  /** Team tenant admin member */
  get teamAdmin() { return getSeedManifest().accounts.teamAdmin; },

  /** Team tenant regular user member */
  get teamMember() { return getSeedManifest().accounts.teamMember; },

  /** AI plan with 1500 credits — full access */
  get aiFullOwner() { return getSeedManifest().accounts.aiFullOwner; },

  /** AI plan with 18 credits — low-credit warning state */
  get aiLowOwner() { return getSeedManifest().accounts.aiLowOwner; },

  /** AI plan with 0 credits — blocked, shows purchase prompt */
  get aiEmptyOwner() { return getSeedManifest().accounts.aiEmptyOwner; },

  /** Tenant that authenticated via API key (not JWT) */
  get apiKeyOwner() { return getSeedManifest().accounts.apiKeyOwner; },

  plans: {
    get free() { return getSeedManifest().plans.free; },
    get pro() { return getSeedManifest().plans.pro; },
    get enterprise() { return getSeedManifest().plans.enterprise; },
  },

  jobs: {
    get pending() { return getSeedManifest().jobs.pending; },
    get completed() { return getSeedManifest().jobs.completed; },
    get dead() { return getSeedManifest().jobs.dead; },
  },
};
