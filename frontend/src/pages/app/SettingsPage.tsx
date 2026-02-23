import { useEffect, useState } from 'react';
import { Settings, User, KeyRound, CheckCircle, AlertCircle, CreditCard, Receipt, Download, ExternalLink, Shield, Smartphone, Monitor, Fingerprint, Trash2, Copy } from 'lucide-react';
import { useAuth } from '../../contexts/AuthContext';
import { authApi, billingApi, plansApi } from '../../api/client';
import type { FinancialTransaction, BillingStatus, ActiveSession, PasskeyCredential } from '../../types';
import LoadingSpinner from '../../components/LoadingSpinner';

function InvoiceModal({ tx, tenantName, onClose }: { tx: FinancialTransaction; tenantName: string; onClose: () => void }) {
  const [downloading, setDownloading] = useState(false);

  const handleDownload = async () => {
    setDownloading(true);
    try {
      const blob = await billingApi.getInvoicePDF(tx.id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `invoice-${tx.invoiceNumber}.pdf`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // Error handled by interceptor
    } finally {
      setDownloading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-lg mx-4 w-full" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-6">
          <h3 className="text-lg font-semibold text-white">Invoice</h3>
          <button onClick={onClose} className="text-dark-400 hover:text-white">&times;</button>
        </div>

        <div className="space-y-4 mb-6">
          <div className="flex justify-between">
            <span className="text-dark-400 text-sm">Invoice Number</span>
            <span className="text-white text-sm font-mono">{tx.invoiceNumber}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-dark-400 text-sm">Date</span>
            <span className="text-white text-sm">{new Date(tx.createdAt).toLocaleDateString()}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-dark-400 text-sm">Bill To</span>
            <span className="text-white text-sm">{tenantName}</span>
          </div>
          <hr className="border-dark-800" />
          <div className="flex justify-between">
            <span className="text-dark-300 text-sm">{tx.description}</span>
            <span className="text-white text-sm font-medium">${(tx.amountCents / 100).toFixed(2)}</span>
          </div>
          <hr className="border-dark-800" />
          <div className="flex justify-between">
            <span className="text-white font-semibold">Total</span>
            <span className="text-white font-semibold">${(tx.amountCents / 100).toFixed(2)} {tx.currency.toUpperCase()}</span>
          </div>
        </div>

        <button
          onClick={handleDownload}
          disabled={downloading}
          className="w-full py-2.5 text-sm font-medium bg-primary-500 text-white rounded-lg hover:bg-primary-600 disabled:opacity-60 transition-colors flex items-center justify-center gap-2"
        >
          {downloading ? <LoadingSpinner size="sm" /> : <><Download className="w-4 h-4" /> Download PDF</>}
        </button>
      </div>
    </div>
  );
}

function MFASetupModal({ onClose, onComplete }: { onClose: () => void; onComplete: () => void }) {
  const [step, setStep] = useState<'qr' | 'verify' | 'codes'>('qr');
  const [qrCodeUrl, setQrCodeUrl] = useState('');
  const [secret, setSecret] = useState('');
  const [code, setCode] = useState('');
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    authApi.mfaSetup()
      .then((data) => {
        setQrCodeUrl(data.qrCodeUrl);
        setSecret(data.secret);
      })
      .catch(() => setError('Failed to initialize MFA setup'));
  }, []);

  const handleVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const data = await authApi.mfaVerifySetup(code);
      setRecoveryCodes(data.recoveryCodes);
      setStep('codes');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setError(msg || 'Invalid code');
    } finally {
      setLoading(false);
    }
  };

  const copyRecoveryCodes = () => {
    navigator.clipboard.writeText(recoveryCodes.join('\n'));
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-dark-900 border border-dark-700 rounded-2xl p-6 max-w-md mx-4 w-full" onClick={e => e.stopPropagation()}>
        {step === 'qr' && (
          <>
            <h3 className="text-lg font-semibold text-white mb-4">Set Up Two-Factor Authentication</h3>
            <p className="text-sm text-dark-400 mb-4">Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.)</p>
            {qrCodeUrl ? (
              <div className="flex justify-center mb-4">
                <img src={qrCodeUrl} alt="QR Code" className="w-48 h-48 rounded-lg bg-white p-2" />
              </div>
            ) : (
              <div className="flex justify-center mb-4 py-8"><LoadingSpinner /></div>
            )}
            {secret && (
              <div className="mb-4">
                <p className="text-xs text-dark-400 mb-1">Or enter this key manually:</p>
                <code className="block text-xs bg-dark-800 text-dark-300 px-3 py-2 rounded-lg font-mono break-all">{secret}</code>
              </div>
            )}
            <button onClick={() => setStep('verify')} className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all text-sm">
              Next
            </button>
          </>
        )}

        {step === 'verify' && (
          <>
            <h3 className="text-lg font-semibold text-white mb-4">Verify Code</h3>
            <p className="text-sm text-dark-400 mb-4">Enter the 6-digit code from your authenticator app</p>
            {error && <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{error}</div>}
            <form onSubmit={handleVerify} className="space-y-4">
              <input
                type="text"
                required
                autoFocus
                autoComplete="one-time-code"
                inputMode="numeric"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white text-center text-lg tracking-widest focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                placeholder="000000"
                maxLength={6}
              />
              <button type="submit" disabled={loading} className="w-full py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-50 transition-all text-sm">
                {loading ? 'Verifying...' : 'Enable MFA'}
              </button>
            </form>
          </>
        )}

        {step === 'codes' && (
          <>
            <h3 className="text-lg font-semibold text-white mb-2">Recovery Codes</h3>
            <p className="text-sm text-dark-400 mb-4">Save these codes in a safe place. Each code can only be used once.</p>
            <div className="bg-dark-800 rounded-lg p-4 mb-4 grid grid-cols-2 gap-2">
              {recoveryCodes.map((c, i) => (
                <code key={i} className="text-sm text-dark-300 font-mono">{c}</code>
              ))}
            </div>
            <div className="flex gap-2">
              <button onClick={copyRecoveryCodes} className="flex-1 py-2.5 px-4 bg-dark-800 border border-dark-700 text-white font-medium rounded-lg hover:bg-dark-700 transition-all text-sm flex items-center justify-center gap-2">
                <Copy className="w-4 h-4" /> Copy
              </button>
              <button onClick={() => { onComplete(); onClose(); }} className="flex-1 py-2.5 px-4 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all text-sm">
                Done
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export default function SettingsPage() {
  const { user, refreshUser } = useAuth();
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [passwordSuccess, setPasswordSuccess] = useState('');
  const [changingPassword, setChangingPassword] = useState(false);
  const [tab, setTab] = useState<'profile' | 'security' | 'sessions' | 'billing'>('profile');

  // MFA state
  const [showMfaSetup, setShowMfaSetup] = useState(false);
  const [mfaDisableCode, setMfaDisableCode] = useState('');
  const [mfaDisableError, setMfaDisableError] = useState('');
  const [mfaDisabling, setMfaDisabling] = useState(false);
  const [showDisableMfa, setShowDisableMfa] = useState(false);

  // Sessions state
  const [sessions, setSessions] = useState<ActiveSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);

  // Passkeys state
  const [passkeys, setPasskeys] = useState<PasskeyCredential[]>([]);
  const [passkeysLoading, setPasskeysLoading] = useState(false);
  const [passkeyName, setPasskeyName] = useState('');
  const [addingPasskey, setAddingPasskey] = useState(false);
  const [passkeyError, setPasskeyError] = useState('');

  // Billing state
  const [billingStatus, setBillingStatus] = useState<BillingStatus>('none');
  const [billingInterval, setBillingInterval] = useState('');
  const [currentPeriodEnd, setCurrentPeriodEnd] = useState('');
  const [currentPlanName, setCurrentPlanName] = useState('');
  const [tenantName] = useState('');
  const [transactions, setTransactions] = useState<FinancialTransaction[]>([]);
  const [txTotal, setTxTotal] = useState(0);
  const [txPage, setTxPage] = useState(1);
  const [billingLoading, setBillingLoading] = useState(false);
  const [portalLoading, setPortalLoading] = useState(false);
  const [selectedTx, setSelectedTx] = useState<FinancialTransaction | null>(null);

  const loadSessions = () => {
    setSessionsLoading(true);
    authApi.listSessions()
      .then((data) => setSessions(data.sessions))
      .catch(() => {})
      .finally(() => setSessionsLoading(false));
  };

  const loadPasskeys = () => {
    setPasskeysLoading(true);
    authApi.listPasskeys()
      .then((data) => setPasskeys(data.passkeys || []))
      .catch(() => {})
      .finally(() => setPasskeysLoading(false));
  };

  const loadBillingData = () => {
    setBillingLoading(true);
    Promise.all([
      plansApi.list(),
      billingApi.listTransactions({ page: txPage, perPage: 10 }),
    ])
      .then(([planData, txData]) => {
        setBillingStatus(planData.billingStatus || 'none');
        setBillingInterval(planData.billingInterval || '');
        setCurrentPeriodEnd(planData.currentPeriodEnd || '');
        const plan = planData.plans.find(p => p.id === planData.currentPlanId);
        setCurrentPlanName(plan?.name || 'Free');
        setTransactions(txData.transactions);
        setTxTotal(txData.total);
      })
      .catch(() => {})
      .finally(() => setBillingLoading(false));
  };

  useEffect(() => {
    if (tab === 'billing') loadBillingData();
    if (tab === 'sessions') loadSessions();
    if (tab === 'security') loadPasskeys();
  }, [tab, txPage]);

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess('');
    setChangingPassword(true);
    try {
      await authApi.changePassword(currentPassword, newPassword);
      setPasswordSuccess('Password changed successfully');
      setCurrentPassword('');
      setNewPassword('');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setPasswordError(msg || 'Failed to change password');
    } finally {
      setChangingPassword(false);
    }
  };

  const handleResendVerification = async () => {
    if (!user?.email) return;
    try {
      await authApi.resendVerification(user.email);
      await refreshUser();
    } catch {
      // ignore
    }
  };

  const handleDisableMfa = async (e: React.FormEvent) => {
    e.preventDefault();
    setMfaDisableError('');
    setMfaDisabling(true);
    try {
      await authApi.mfaDisable(mfaDisableCode);
      await refreshUser();
      setShowDisableMfa(false);
      setMfaDisableCode('');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      setMfaDisableError(msg || 'Invalid code');
    } finally {
      setMfaDisabling(false);
    }
  };

  const handleRevokeSession = async (id: string) => {
    try {
      await authApi.revokeSession(id);
      setSessions(s => s.filter(session => session.id !== id));
    } catch {
      // ignore
    }
  };

  const handleRevokeAllSessions = async () => {
    try {
      await authApi.revokeAllSessions();
      loadSessions();
    } catch {
      // ignore
    }
  };

  const handleAddPasskey = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasskeyError('');
    setAddingPasskey(true);
    try {
      const options = await authApi.passkeyRegisterBegin();
      const credential = await navigator.credentials.create({ publicKey: options });
      if (!credential) throw new Error('No credential created');
      await authApi.passkeyRegisterFinish({ name: passkeyName || 'My Passkey', credential });
      setPasskeyName('');
      loadPasskeys();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error
        || (err as Error)?.message || 'Failed to add passkey';
      setPasskeyError(msg);
    } finally {
      setAddingPasskey(false);
    }
  };

  const handleDeletePasskey = async (id: string) => {
    try {
      await authApi.deletePasskey(id);
      setPasskeys(p => p.filter(pk => pk.id !== id));
    } catch {
      // ignore
    }
  };

  const handlePortal = async () => {
    setPortalLoading(true);
    try {
      const result = await billingApi.portal();
      window.location.href = result.portalUrl;
    } catch {
      // Error handled by interceptor
    } finally {
      setPortalLoading(false);
    }
  };

  const totalPages = Math.ceil(txTotal / 10);

  const tabs = [
    { key: 'profile' as const, label: 'Profile' },
    { key: 'security' as const, label: 'Security' },
    { key: 'sessions' as const, label: 'Sessions' },
    { key: 'billing' as const, label: 'Billing' },
  ];

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white flex items-center gap-3">
          <Settings className="w-7 h-7 text-primary-400" />
          Settings
        </h1>
        <p className="text-dark-400 mt-1">Manage your account</p>
      </div>

      {/* Tab Navigation */}
      <div className="flex gap-1 mb-6 bg-dark-900/50 border border-dark-800 rounded-xl p-1 max-w-md">
        {tabs.map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`flex-1 px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              tab === t.key ? 'bg-dark-700 text-white' : 'text-dark-400 hover:text-dark-300'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Profile Tab */}
      {tab === 'profile' && (
        <div className="space-y-6 max-w-2xl">
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
            <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
              <User className="w-5 h-5 text-dark-400" />
              Profile
            </h2>
            <div className="space-y-3">
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-dark-400">Name</span>
                <span className="text-sm text-white">{user?.displayName}</span>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-dark-800">
                <span className="text-sm text-dark-400">Email</span>
                <span className="text-sm text-white">{user?.email}</span>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-dark-800">
                <span className="text-sm text-dark-400">Email Verified</span>
                <div className="flex items-center gap-2">
                  {user?.emailVerified ? (
                    <span className="flex items-center gap-1 text-sm text-accent-emerald">
                      <CheckCircle className="w-4 h-4" /> Verified
                    </span>
                  ) : (
                    <div className="flex items-center gap-2">
                      <span className="flex items-center gap-1 text-sm text-amber-400">
                        <AlertCircle className="w-4 h-4" /> Not verified
                      </span>
                      <button
                        onClick={handleResendVerification}
                        className="text-xs text-primary-400 hover:text-primary-300 transition-colors"
                      >
                        Resend
                      </button>
                    </div>
                  )}
                </div>
              </div>
              <div className="flex items-center justify-between py-2 border-t border-dark-800">
                <span className="text-sm text-dark-400">Auth Methods</span>
                <div className="flex gap-2">
                  {user?.authMethods.map((method) => (
                    <span key={method} className="px-2 py-0.5 bg-dark-800 rounded text-xs text-dark-300 capitalize">
                      {method}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Change Password */}
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
            <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
              <KeyRound className="w-5 h-5 text-dark-400" />
              Change Password
            </h2>

            {passwordError && (
              <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{passwordError}</div>
            )}
            {passwordSuccess && (
              <div className="mb-4 bg-accent-emerald/10 border border-accent-emerald/20 rounded-lg p-3 text-sm text-accent-emerald">{passwordSuccess}</div>
            )}

            <form onSubmit={handleChangePassword} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1.5">Current Password</label>
                <input
                  type="password"
                  required
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-dark-300 mb-1.5">New Password</label>
                <input
                  type="password"
                  required
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                  placeholder="Min 10 chars, mixed case, number, special"
                />
              </div>
              <button
                type="submit"
                disabled={changingPassword}
                className="py-2.5 px-6 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 disabled:opacity-50 disabled:cursor-not-allowed transition-all text-sm"
              >
                {changingPassword ? 'Changing...' : 'Change Password'}
              </button>
            </form>
          </div>
        </div>
      )}

      {/* Security Tab */}
      {tab === 'security' && (
        <div className="space-y-6 max-w-2xl">
          {/* MFA Section */}
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
            <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
              <Shield className="w-5 h-5 text-dark-400" />
              Two-Factor Authentication
            </h2>

            {user?.totpEnabled ? (
              <div>
                <div className="flex items-center gap-2 mb-4">
                  <span className="flex items-center gap-1 text-sm text-accent-emerald">
                    <CheckCircle className="w-4 h-4" /> Enabled
                  </span>
                </div>

                {showDisableMfa ? (
                  <form onSubmit={handleDisableMfa} className="space-y-3">
                    {mfaDisableError && (
                      <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{mfaDisableError}</div>
                    )}
                    <div>
                      <label className="block text-sm font-medium text-dark-300 mb-1.5">Enter TOTP code or recovery code to disable</label>
                      <input
                        type="text"
                        required
                        autoFocus
                        value={mfaDisableCode}
                        onChange={(e) => setMfaDisableCode(e.target.value)}
                        className="w-full px-4 py-2.5 bg-dark-800 border border-dark-700 rounded-lg text-white focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors"
                        placeholder="000000"
                      />
                    </div>
                    <div className="flex gap-2">
                      <button type="submit" disabled={mfaDisabling} className="py-2 px-4 bg-red-500/20 text-red-400 border border-red-500/30 rounded-lg hover:bg-red-500/30 text-sm disabled:opacity-50 transition-all">
                        {mfaDisabling ? 'Disabling...' : 'Confirm Disable'}
                      </button>
                      <button type="button" onClick={() => { setShowDisableMfa(false); setMfaDisableCode(''); setMfaDisableError(''); }} className="py-2 px-4 bg-dark-800 text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-700 text-sm transition-all">
                        Cancel
                      </button>
                    </div>
                  </form>
                ) : (
                  <button
                    onClick={() => setShowDisableMfa(true)}
                    className="py-2 px-4 bg-dark-800 text-dark-300 border border-dark-700 rounded-lg hover:bg-dark-700 text-sm transition-all"
                  >
                    Disable MFA
                  </button>
                )}
              </div>
            ) : (
              <div>
                <p className="text-sm text-dark-400 mb-4">Add an extra layer of security to your account with a TOTP authenticator app.</p>
                <button
                  onClick={() => setShowMfaSetup(true)}
                  className="py-2.5 px-6 bg-gradient-to-r from-primary-600 to-primary-500 text-white font-medium rounded-lg hover:from-primary-500 hover:to-primary-400 transition-all text-sm"
                >
                  Enable Two-Factor Authentication
                </button>
              </div>
            )}
          </div>

          {/* Passkeys Section */}
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
            <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
              <Fingerprint className="w-5 h-5 text-dark-400" />
              Passkeys
            </h2>

            {passkeysLoading ? (
              <LoadingSpinner size="sm" className="py-4" />
            ) : (
              <>
                {passkeys.length > 0 && (
                  <div className="space-y-2 mb-4">
                    {passkeys.map(pk => (
                      <div key={pk.id} className="flex items-center justify-between py-2 px-3 bg-dark-800/50 rounded-lg">
                        <div>
                          <p className="text-sm text-white">{pk.name}</p>
                          <p className="text-xs text-dark-400">
                            Added {new Date(pk.createdAt).toLocaleDateString()}
                            {pk.lastUsedAt && ` · Last used ${new Date(pk.lastUsedAt).toLocaleDateString()}`}
                          </p>
                        </div>
                        <button onClick={() => handleDeletePasskey(pk.id)} className="text-dark-400 hover:text-red-400 transition-colors">
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}

                {passkeyError && (
                  <div className="mb-4 bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-sm text-red-400">{passkeyError}</div>
                )}

                <form onSubmit={handleAddPasskey} className="flex gap-2">
                  <input
                    type="text"
                    value={passkeyName}
                    onChange={(e) => setPasskeyName(e.target.value)}
                    className="flex-1 px-4 py-2 bg-dark-800 border border-dark-700 rounded-lg text-white placeholder-dark-500 focus:outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500 transition-colors text-sm"
                    placeholder="Passkey name (e.g., MacBook)"
                  />
                  <button type="submit" disabled={addingPasskey} className="py-2 px-4 bg-dark-800 border border-dark-700 text-white rounded-lg hover:bg-dark-700 text-sm disabled:opacity-50 transition-all">
                    {addingPasskey ? 'Adding...' : 'Add Passkey'}
                  </button>
                </form>
              </>
            )}
          </div>
        </div>
      )}

      {/* Sessions Tab */}
      {tab === 'sessions' && (
        <div className="space-y-6 max-w-2xl">
          <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-white flex items-center gap-2">
                <Monitor className="w-5 h-5 text-dark-400" />
                Active Sessions
              </h2>
              {sessions.length > 1 && (
                <button
                  onClick={handleRevokeAllSessions}
                  className="text-xs text-red-400 hover:text-red-300 transition-colors"
                >
                  Revoke all other sessions
                </button>
              )}
            </div>

            {sessionsLoading ? (
              <LoadingSpinner size="sm" className="py-8" />
            ) : sessions.length === 0 ? (
              <p className="text-sm text-dark-400">No active sessions found.</p>
            ) : (
              <div className="space-y-3">
                {sessions.map(session => (
                  <div key={session.id} className="flex items-center justify-between py-3 px-4 bg-dark-800/50 rounded-lg">
                    <div className="flex items-start gap-3">
                      <Smartphone className="w-5 h-5 text-dark-400 mt-0.5 shrink-0" />
                      <div>
                        <p className="text-sm text-white">
                          {session.deviceInfo || session.userAgent.slice(0, 50)}
                          {session.isCurrent && (
                            <span className="ml-2 px-2 py-0.5 bg-primary-500/20 text-primary-400 text-xs rounded">Current</span>
                          )}
                        </p>
                        <p className="text-xs text-dark-400">
                          {session.ipAddress} · Last active {new Date(session.lastActiveAt).toLocaleString()}
                        </p>
                      </div>
                    </div>
                    {!session.isCurrent && (
                      <button
                        onClick={() => handleRevokeSession(session.id)}
                        className="text-xs text-red-400 hover:text-red-300 transition-colors shrink-0"
                      >
                        Revoke
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Billing Tab */}
      {tab === 'billing' && (
        <div className="space-y-6 max-w-3xl">
          {billingLoading ? (
            <LoadingSpinner size="lg" className="py-20" />
          ) : (
            <>
              {/* Subscription Summary */}
              <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl p-6">
                <h2 className="text-lg font-semibold text-white flex items-center gap-2 mb-4">
                  <CreditCard className="w-5 h-5 text-dark-400" />
                  Subscription
                </h2>
                <div className="space-y-3">
                  <div className="flex items-center justify-between py-2">
                    <span className="text-sm text-dark-400">Plan</span>
                    <span className="text-sm text-white font-medium">{currentPlanName}</span>
                  </div>
                  <div className="flex items-center justify-between py-2 border-t border-dark-800">
                    <span className="text-sm text-dark-400">Status</span>
                    <span className={`text-sm font-medium ${
                      billingStatus === 'active' ? 'text-accent-emerald' :
                      billingStatus === 'past_due' ? 'text-red-400' :
                      billingStatus === 'canceled' ? 'text-yellow-400' :
                      'text-dark-400'
                    }`}>
                      {billingStatus === 'active' ? 'Active' :
                       billingStatus === 'past_due' ? 'Past Due' :
                       billingStatus === 'canceled' ? 'Canceled' :
                       'None'}
                    </span>
                  </div>
                  {billingInterval && (
                    <div className="flex items-center justify-between py-2 border-t border-dark-800">
                      <span className="text-sm text-dark-400">Billing Interval</span>
                      <span className="text-sm text-white capitalize">{billingInterval}ly</span>
                    </div>
                  )}
                  {currentPeriodEnd && (
                    <div className="flex items-center justify-between py-2 border-t border-dark-800">
                      <span className="text-sm text-dark-400">
                        {billingStatus === 'canceled' ? 'Benefits Until' : 'Next Billing'}
                      </span>
                      <span className="text-sm text-white">{new Date(currentPeriodEnd).toLocaleDateString()}</span>
                    </div>
                  )}
                </div>

                {billingStatus !== 'none' && (
                  <div className="mt-4 pt-4 border-t border-dark-800">
                    <button
                      onClick={handlePortal}
                      disabled={portalLoading}
                      className="inline-flex items-center gap-2 px-4 py-2 text-sm bg-dark-800 text-dark-300 border border-dark-700 rounded-lg hover:border-dark-600 hover:text-white transition-colors disabled:opacity-60"
                    >
                      {portalLoading ? <LoadingSpinner size="sm" /> : <><ExternalLink className="w-4 h-4" /> Update Payment Method</>}
                    </button>
                  </div>
                )}
              </div>

              {/* Transaction History */}
              <div className="bg-dark-900/50 backdrop-blur-sm border border-dark-800 rounded-2xl overflow-hidden">
                <div className="px-6 py-4 border-b border-dark-800">
                  <h2 className="text-lg font-semibold text-white flex items-center gap-2">
                    <Receipt className="w-5 h-5 text-dark-400" />
                    Transaction History
                  </h2>
                </div>

                {transactions.length === 0 ? (
                  <div className="text-center py-12 text-dark-400">No transactions yet</div>
                ) : (
                  <>
                    <div className="overflow-x-auto">
                      <table className="w-full">
                        <thead>
                          <tr className="border-b border-dark-800">
                            <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Date</th>
                            <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Description</th>
                            <th className="text-right px-6 py-3 text-xs font-medium text-dark-400 uppercase">Amount</th>
                            <th className="text-left px-6 py-3 text-xs font-medium text-dark-400 uppercase">Invoice</th>
                            <th className="px-6 py-3"></th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-dark-800/50">
                          {transactions.map(tx => (
                            <tr key={tx.id} className="hover:bg-dark-800/20">
                              <td className="px-6 py-3 text-sm text-dark-300 whitespace-nowrap">
                                {new Date(tx.createdAt).toLocaleDateString()}
                              </td>
                              <td className="px-6 py-3 text-sm text-white">{tx.description}</td>
                              <td className="px-6 py-3 text-sm text-white text-right font-mono">
                                ${(tx.amountCents / 100).toFixed(2)}
                              </td>
                              <td className="px-6 py-3 text-sm text-dark-400 font-mono">{tx.invoiceNumber}</td>
                              <td className="px-6 py-3">
                                <button
                                  onClick={() => setSelectedTx(tx)}
                                  className="text-xs text-primary-400 hover:text-primary-300 transition-colors"
                                >
                                  View
                                </button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>

                    {totalPages > 1 && (
                      <div className="px-6 py-3 border-t border-dark-800 flex items-center justify-between">
                        <span className="text-xs text-dark-400">{txTotal} total</span>
                        <div className="flex gap-2">
                          <button
                            onClick={() => setTxPage(p => Math.max(1, p - 1))}
                            disabled={txPage === 1}
                            className="px-3 py-1 text-xs bg-dark-800 text-dark-300 rounded disabled:opacity-40"
                          >
                            Prev
                          </button>
                          <span className="px-3 py-1 text-xs text-dark-400">{txPage} / {totalPages}</span>
                          <button
                            onClick={() => setTxPage(p => Math.min(totalPages, p + 1))}
                            disabled={txPage === totalPages}
                            className="px-3 py-1 text-xs bg-dark-800 text-dark-300 rounded disabled:opacity-40"
                          >
                            Next
                          </button>
                        </div>
                      </div>
                    )}
                  </>
                )}
              </div>
            </>
          )}

          {selectedTx && (
            <InvoiceModal tx={selectedTx} tenantName={tenantName || 'Your Organization'} onClose={() => setSelectedTx(null)} />
          )}
        </div>
      )}

      {/* MFA Setup Modal */}
      {showMfaSetup && (
        <MFASetupModal onClose={() => setShowMfaSetup(false)} onComplete={refreshUser} />
      )}
    </div>
  );
}
