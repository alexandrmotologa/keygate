import React, { useState, useEffect } from 'react';
import { APIKey, UsageEvent } from '../types';
import { fetchKeyDetails, updateKey } from '../services/api';
import { X, KeyRound, Save, RefreshCw, Check, Globe, Tag, Layers, Clock, Ban } from 'lucide-react';

interface KeyDetailDrawerProps {
  keyId: string | null;
  onClose: () => void;
  onKeyUpdated: () => void;
  onRotateKey: (key: APIKey) => void;
  onRevokeKey: (keyId: string, name: string) => void;
}

export const KeyDetailDrawer: React.FC<KeyDetailDrawerProps> = ({
  keyId,
  onClose,
  onKeyUpdated,
  onRotateKey,
  onRevokeKey,
}) => {
  const [key, setKey] = useState<APIKey | null>(null);
  const [history, setHistory] = useState<UsageEvent[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [savedSuccess, setSavedSuccess] = useState(false);

  // Editable fields
  const [name, setName] = useState('');
  const [tier, setTier] = useState('free');
  const [rateLimit, setRateLimit] = useState(60);
  const [rateDuration, setRateDuration] = useState(60);
  const [burst, setBurst] = useState(10);
  const [maxUses, setMaxUses] = useState(0);
  const [enabled, setEnabled] = useState(true);
  const [scopesStr, setScopesStr] = useState('');
  const [ipStr, setIpStr] = useState('');

  useEffect(() => {
    if (!keyId) {
      setKey(null);
      return;
    }

    const load = async () => {
      setIsLoading(true);
      try {
        const data = await fetchKeyDetails(keyId);
        setKey(data.key);
        setHistory(data.history || []);

        setName(data.key.name);
        setTier(data.key.tier);
        setRateLimit(data.key.rateLimit);
        setRateDuration(data.key.rateDuration);
        setBurst(data.key.burst);
        setMaxUses(data.key.maxUses || 0);
        setEnabled(data.key.enabled);
        setScopesStr((data.key.scopes || []).join(', '));
        setIpStr((data.key.ipAllowlist || []).join(', '));
      } finally {
        setIsLoading(false);
      }
    };

    load();
  }, [keyId]);

  if (!keyId) return null;

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!key) return;

    setIsSaving(true);
    try {
      const scopes = scopesStr
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);

      const ipAllowlist = ipStr
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);

      await updateKey(key.id, {
        name,
        tier,
        rateLimit,
        rateDuration,
        burst,
        maxUses,
        enabled,
        scopes,
        ipAllowlist,
      });

      setSavedSuccess(true);
      setTimeout(() => setSavedSuccess(false), 2000);
      onKeyUpdated();
    } finally {
      setIsSaving(false);
    }
  };

  const isRevoked = Boolean(key?.revokedAt);
  const quotaUsedPct =
    key && key.maxUses > 0 ? Math.min(100, (key.totalRequests / key.maxUses) * 100) : 0;

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/60 backdrop-blur-xs animate-in fade-in duration-150">
      <div className="bg-surface border-l border-border w-full max-w-xl h-full shadow-2xl flex flex-col overflow-hidden animate-in slide-in-from-right duration-200">
        {/* Header */}
        <div className="p-5 border-b border-border flex items-center justify-between bg-background/50">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-primary-950/60 border border-primary-800/40 rounded-lg text-primary-400">
              <KeyRound className="w-5 h-5" />
            </div>
            <div>
              <h2 className="font-bold text-white text-base truncate max-w-[320px]">
                {key?.name || 'Key Details'}
              </h2>
              <span className="text-xs font-mono text-slate-500 block">{keyId}</span>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-surface-hover transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        {isLoading ? (
          <div className="flex-1 flex items-center justify-center text-slate-400 text-sm">
            Loading key configuration...
          </div>
        ) : !key ? (
          <div className="flex-1 flex items-center justify-center text-slate-500 text-sm">
            Key not found.
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto p-6 space-y-6">
            {/* Quick Status Bar */}
            <div className="flex items-center justify-between p-3.5 bg-background border border-border rounded-xl text-xs">
              <div className="flex items-center gap-2">
                <span
                  className={`w-2.5 h-2.5 rounded-full ${
                    isRevoked
                      ? 'bg-rose-500'
                      : enabled
                      ? 'bg-emerald-400'
                      : 'bg-amber-400'
                  }`}
                />
                <span className="font-semibold text-white">
                  {isRevoked ? 'REVOKED' : enabled ? 'ACTIVE' : 'DISABLED'}
                </span>
              </div>

              {!isRevoked && (
                <label className="flex items-center gap-2 cursor-pointer select-none">
                  <span className="text-slate-400 text-[11px]">Allow Traffic</span>
                  <input
                    type="checkbox"
                    checked={enabled}
                    onChange={(e) => setEnabled(e.target.checked)}
                    className="rounded bg-surface border-border text-primary-600 focus:ring-0"
                  />
                </label>
              )}
            </div>

            {/* Quota Progress Bar (if MaxUses configured) */}
            {key.maxUses > 0 && (
              <div className="p-4 bg-background border border-border rounded-xl space-y-2">
                <div className="flex justify-between text-xs">
                  <span className="text-slate-400 font-medium">Quota Consumption (Max Uses)</span>
                  <span className="font-mono text-white font-bold">
                    {key.totalRequests} / {key.maxUses}
                  </span>
                </div>
                <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
                  <div
                    className={`h-full transition-all ${
                      quotaUsedPct >= 100 ? 'bg-rose-500' : quotaUsedPct > 80 ? 'bg-amber-400' : 'bg-primary-500'
                    }`}
                    style={{ width: `${quotaUsedPct}%` }}
                  />
                </div>
                <span className="text-[11px] text-slate-500 block">
                  {key.totalRequests >= key.maxUses
                    ? 'Quota exhausted: this key is blocked from further requests.'
                    : `${key.maxUses - key.totalRequests} calls remaining before auto-block.`}
                </span>
              </div>
            )}

            {/* Live Editable Form */}
            <form onSubmit={handleSave} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
                  Key Name
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-3 py-2 bg-background border border-border rounded-lg text-sm text-white focus:outline-none focus:border-primary-500"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5 flex items-center gap-1.5">
                    <Layers className="w-3.5 h-3.5 text-primary-400" />
                    Tier
                  </label>
                  <select
                    value={tier}
                    onChange={(e) => setTier(e.target.value)}
                    className="w-full px-3 py-2 bg-background border border-border rounded-lg text-sm text-white focus:outline-none focus:border-primary-500"
                  >
                    <option value="free">Free (10/min)</option>
                    <option value="pro">Pro (500/min)</option>
                    <option value="enterprise">Enterprise (10k/min)</option>
                    <option value="custom">Custom Quota</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
                    Max Uses (0 = unlimited)
                  </label>
                  <input
                    type="number"
                    min="0"
                    value={maxUses}
                    onChange={(e) => setMaxUses(parseInt(e.target.value) || 0)}
                    className="w-full px-3 py-2 bg-background border border-border rounded-lg text-sm text-white font-mono focus:outline-none focus:border-primary-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="block text-[11px] text-slate-400 mb-1">Limit (Req)</label>
                  <input
                    type="number"
                    min="1"
                    value={rateLimit}
                    onChange={(e) => setRateLimit(parseInt(e.target.value) || 60)}
                    className="w-full px-2.5 py-1.5 bg-background border border-border rounded-lg text-xs text-white font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[11px] text-slate-400 mb-1">Window (Sec)</label>
                  <input
                    type="number"
                    min="1"
                    value={rateDuration}
                    onChange={(e) => setRateDuration(parseInt(e.target.value) || 60)}
                    className="w-full px-2.5 py-1.5 bg-background border border-border rounded-lg text-xs text-white font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[11px] text-slate-400 mb-1">Burst</label>
                  <input
                    type="number"
                    min="0"
                    value={burst}
                    onChange={(e) => setBurst(parseInt(e.target.value) || 0)}
                    className="w-full px-2.5 py-1.5 bg-background border border-border rounded-lg text-xs text-white font-mono"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5 flex items-center gap-1.5">
                  <Tag className="w-3.5 h-3.5 text-slate-400" />
                  Permission Scopes
                </label>
                <input
                  type="text"
                  value={scopesStr}
                  onChange={(e) => setScopesStr(e.target.value)}
                  className="w-full px-3 py-2 bg-background border border-border rounded-lg text-xs text-white font-mono focus:outline-none focus:border-primary-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5 flex items-center gap-1.5">
                  <Globe className="w-3.5 h-3.5 text-slate-400" />
                  IP Allowlist (Comma-separated)
                </label>
                <input
                  type="text"
                  value={ipStr}
                  onChange={(e) => setIpStr(e.target.value)}
                  className="w-full px-3 py-2 bg-background border border-border rounded-lg text-xs text-white font-mono focus:outline-none focus:border-primary-500"
                />
              </div>

              <div className="flex items-center justify-between pt-2">
                <button
                  type="submit"
                  disabled={isSaving}
                  className="px-4 py-2 text-xs font-semibold text-white bg-primary-600 hover:bg-primary-500 rounded-lg transition-colors flex items-center gap-2"
                >
                  {savedSuccess ? <Check className="w-4 h-4 text-emerald-300" /> : <Save className="w-4 h-4" />}
                  {savedSuccess ? 'Saved' : isSaving ? 'Saving...' : 'Save Configuration'}
                </button>

                <div className="flex items-center gap-2">
                  {!isRevoked && (
                    <button
                      type="button"
                      onClick={() => onRotateKey(key)}
                      className="px-3 py-2 text-xs font-medium text-amber-300 hover:bg-amber-950/40 border border-amber-800/40 rounded-lg transition-colors flex items-center gap-1.5"
                    >
                      <RefreshCw className="w-3.5 h-3.5" />
                      Rotate Key
                    </button>
                  )}
                  {!isRevoked && (
                    <button
                      type="button"
                      onClick={() => onRevokeKey(key.id, key.name)}
                      className="px-3 py-2 text-xs font-medium text-rose-400 hover:bg-rose-950/40 border border-rose-900/40 rounded-lg transition-colors flex items-center gap-1.5"
                    >
                      <Ban className="w-3.5 h-3.5" />
                      Revoke
                    </button>
                  )}
                </div>
              </div>
            </form>

            {/* Recent Request Telemetry for this key */}
            <div className="pt-4 border-t border-border">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-300 mb-3 flex items-center gap-1.5">
                <Clock className="w-3.5 h-3.5 text-slate-400" />
                Recent Requests for this Key ({history.length})
              </h3>

              {history.length === 0 ? (
                <div className="p-6 text-center text-xs text-slate-500 bg-background border border-border rounded-lg">
                  No requests recorded for this key yet.
                </div>
              ) : (
                <div className="space-y-1.5 max-h-48 overflow-y-auto">
                  {history.map((h, i) => (
                    <div
                      key={i}
                      className="flex items-center justify-between p-2 bg-background border border-border rounded text-[11px] font-mono"
                    >
                      <div className="flex items-center gap-2">
                        <span
                          className={`px-1.5 py-0.2 rounded font-bold ${
                            h.statusCode >= 200 && h.statusCode < 300
                              ? 'bg-emerald-950 text-emerald-400'
                              : 'bg-rose-950 text-rose-400'
                          }`}
                        >
                          {h.statusCode}
                        </span>
                        <span className="text-slate-300 font-bold">{h.method}</span>
                        <span className="text-slate-400 truncate max-w-[150px]">{h.path}</span>
                      </div>
                      <div className="flex items-center gap-2 text-slate-400">
                        <span className="text-emerald-400">{h.latencyMs.toFixed(2)}ms</span>
                        <span>{new Date(h.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
