import React, { useState } from 'react';
import { CreateKeyPayload } from '../services/api';
import { X, KeyRound, Shield, Tag, Globe, Calendar, Layers } from 'lucide-react';

interface KeyCreationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (payload: CreateKeyPayload) => Promise<void>;
}

export const KeyCreationModal: React.FC<KeyCreationModalProps> = ({ isOpen, onClose, onSubmit }) => {
  const [name, setName] = useState('');
  const [prefix, setPrefix] = useState('sk_live');
  const [workspaceId, setWorkspaceId] = useState('ws_default');
  const [ownerId, setOwnerId] = useState('usr_developer');
  const [tier, setTier] = useState<'free' | 'pro' | 'enterprise' | 'custom'>('pro');
  const [customRateLimit, setCustomRateLimit] = useState(500);
  const [customRateDuration, setCustomRateDuration] = useState(60);
  const [customBurst, setCustomBurst] = useState(25);
  const [scopesInput, setScopesInput] = useState('read:users, write:keys');
  const [ipAllowlistInput, setIpAllowlistInput] = useState('');
  const [maxUses, setMaxUses] = useState(0);
  const [expiryOption, setExpiryOption] = useState<'never' | '30d' | '90d' | '1y'>('90d');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      // Calculate expiration date
      let expiresAt: string | undefined;
      const now = new Date();
      if (expiryOption === '30d') {
        expiresAt = new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000).toISOString();
      } else if (expiryOption === '90d') {
        expiresAt = new Date(now.getTime() + 90 * 24 * 60 * 60 * 1000).toISOString();
      } else if (expiryOption === '1y') {
        expiresAt = new Date(now.getTime() + 365 * 24 * 60 * 60 * 1000).toISOString();
      }

      const scopes = scopesInput
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);

      const ipAllowlist = ipAllowlistInput
        .split(/[\n,]/)
        .map((ip) => ip.trim())
        .filter(Boolean);

      const payload: CreateKeyPayload = {
        name: name || 'API Key',
        prefix,
        workspaceId,
        ownerId,
        tier,
        maxUses,
        scopes,
        ipAllowlist,
        expiresAt,
      };

      if (tier === 'custom') {
        payload.rateLimit = customRateLimit;
        payload.rateDuration = customRateDuration;
        payload.burst = customBurst;
      }

      await onSubmit(payload);
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to create key');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-surface border border-border w-full max-w-xl rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Modal Header */}
        <div className="flex items-center justify-between p-5 border-b border-border">
          <div className="flex items-center gap-2.5">
            <div className="p-2 bg-primary-950/60 border border-primary-800/40 rounded-lg text-primary-400">
              <KeyRound className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-semibold text-white text-base">Issue New API Key</h3>
              <p className="text-xs text-slate-400 mt-0.5">Generates a cryptographically random prefix-encoded key</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-surface-hover transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Modal Body */}
        <form onSubmit={handleSubmit} className="p-6 space-y-5 overflow-y-auto flex-1">
          {error && (
            <div className="p-3 bg-rose-950/50 border border-rose-800/50 rounded-lg text-xs text-rose-300">
              {error}
            </div>
          )}

          {/* Key Name & Prefix */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
                Key Identifier / Name
              </label>
              <input
                type="text"
                required
                placeholder="e.g. Production Mobile Backend"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white placeholder-slate-500 focus:outline-none focus:border-primary-500"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
                Key Prefix
              </label>
              <input
                type="text"
                required
                value={prefix}
                onChange={(e) => setPrefix(e.target.value)}
                className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white font-mono focus:outline-none focus:border-primary-500"
              />
            </div>
          </div>

          {/* Workspace & Owner */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
                Workspace ID
              </label>
              <input
                type="text"
                value={workspaceId}
                onChange={(e) => setWorkspaceId(e.target.value)}
                className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white font-mono focus:outline-none focus:border-primary-500"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
                Owner ID / Tenant
              </label>
              <input
                type="text"
                value={ownerId}
                onChange={(e) => setOwnerId(e.target.value)}
                className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white font-mono focus:outline-none focus:border-primary-500"
              />
            </div>
          </div>

          {/* Rate Limiting Tier */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-2 flex items-center gap-1.5">
              <Layers className="w-3.5 h-3.5 text-primary-400" />
              Rate Limiting Policy Tier
            </label>
            <div className="grid grid-cols-4 gap-2">
              {[
                { id: 'free', name: 'Free', quota: '10/m' },
                { id: 'pro', name: 'Pro', quota: '500/m' },
                { id: 'enterprise', name: 'Enterprise', quota: '10k/m' },
                { id: 'custom', name: 'Custom', quota: 'Configurable' },
              ].map((t) => (
                <button
                  type="button"
                  key={t.id}
                  onClick={() => setTier(t.id as any)}
                  className={`p-2.5 rounded-lg border text-left transition-all ${
                    tier === t.id
                      ? 'bg-primary-950/40 border-primary-500 text-white'
                      : 'bg-background border-border text-slate-400 hover:border-border-light'
                  }`}
                >
                  <div className="font-semibold text-xs text-white">{t.name}</div>
                  <div className="text-[10px] font-mono text-slate-400 mt-0.5">{t.quota}</div>
                </button>
              ))}
            </div>

            {tier === 'custom' && (
              <div className="grid grid-cols-3 gap-3 mt-3 p-3 bg-background border border-border rounded-lg">
                <div>
                  <label className="block text-[11px] text-slate-400 mb-1">Limit (Req)</label>
                  <input
                    type="number"
                    min="1"
                    value={customRateLimit}
                    onChange={(e) => setCustomRateLimit(parseInt(e.target.value) || 100)}
                    className="w-full px-2 py-1 text-xs bg-surface border border-border rounded text-white font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[11px] text-slate-400 mb-1">Window (Sec)</label>
                  <input
                    type="number"
                    min="1"
                    value={customRateDuration}
                    onChange={(e) => setCustomRateDuration(parseInt(e.target.value) || 60)}
                    className="w-full px-2 py-1 text-xs bg-surface border border-border rounded text-white font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[11px] text-slate-400 mb-1">Burst</label>
                  <input
                    type="number"
                    min="0"
                    value={customBurst}
                    onChange={(e) => setCustomBurst(parseInt(e.target.value) || 0)}
                    className="w-full px-2 py-1 text-xs bg-surface border border-border rounded text-white font-mono"
                  />
                </div>
              </div>
            )}
          </div>

          {/* Max Uses Quota Cap */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5 flex items-center justify-between">
              <span>Lifetime Request Quota (Max Uses)</span>
              <span className="text-slate-500 font-normal lowercase">0 = unlimited</span>
            </label>
            <input
              type="number"
              min="0"
              placeholder="0 (Unlimited)"
              value={maxUses || ''}
              onChange={(e) => setMaxUses(parseInt(e.target.value) || 0)}
              className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white font-mono placeholder-slate-600 focus:outline-none focus:border-primary-500"
            />
            <span className="text-[11px] text-slate-500 mt-1 block">
              Ideal for ephemeral AI agents or temporary access. Key is auto-blocked once quota is exhausted.
            </span>
          </div>

          {/* Scopes */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5 flex items-center gap-1.5">
              <Tag className="w-3.5 h-3.5 text-slate-400" />
              Permission Scopes (Comma-separated)
            </label>
            <input
              type="text"
              placeholder="e.g. read:users, write:payments, analytics:view"
              value={scopesInput}
              onChange={(e) => setScopesInput(e.target.value)}
              className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white placeholder-slate-500 font-mono focus:outline-none focus:border-primary-500"
            />
          </div>

          {/* IP Allowlist */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5 flex items-center gap-1.5">
              <Globe className="w-3.5 h-3.5 text-slate-400" />
              CIDR / IP Allowlist (Optional)
            </label>
            <textarea
              rows={2}
              placeholder="e.g. 192.168.1.0/24, 10.0.0.5 (Leave blank to permit any client IP)"
              value={ipAllowlistInput}
              onChange={(e) => setIpAllowlistInput(e.target.value)}
              className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white placeholder-slate-500 font-mono focus:outline-none focus:border-primary-500"
            />
          </div>

          {/* Expiration */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5 flex items-center gap-1.5">
              <Calendar className="w-3.5 h-3.5 text-slate-400" />
              Expiration Period
            </label>
            <div className="grid grid-cols-4 gap-2">
              {[
                { id: '30d', label: '30 Days' },
                { id: '90d', label: '90 Days' },
                { id: '1y', label: '1 Year' },
                { id: 'never', label: 'Never' },
              ].map((opt) => (
                <button
                  type="button"
                  key={opt.id}
                  onClick={() => setExpiryOption(opt.id as any)}
                  className={`py-2 px-3 text-xs font-medium rounded-lg border transition-all ${
                    expiryOption === opt.id
                      ? 'bg-surface-hover border-primary-500 text-white font-semibold'
                      : 'bg-background border-border text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          {/* Footer Submit */}
          <div className="pt-3 border-t border-border flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm font-medium text-slate-400 hover:text-slate-200 bg-surface-hover rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-5 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-500 rounded-lg shadow-sm shadow-primary-900/40 transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              <Shield className="w-4 h-4" />
              {isSubmitting ? 'Generating...' : 'Issue Key'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
