import React, { useState } from 'react';
import { APIKey } from '../types';
import { Search, KeyRound, Copy, Check, Ban, ShieldCheck, Globe, Clock, AlertTriangle, Download, RefreshCw, SlidersHorizontal } from 'lucide-react';

interface KeyManagementTableProps {
  keys: APIKey[];
  isLoading: boolean;
  workspaceId: string;
  onRevokeKey: (keyId: string, name: string) => Promise<void>;
  onRotateKey: (key: APIKey) => void;
  onSelectKey: (keyId: string) => void;
  onOpenCreateModal: () => void;
  onRefresh: () => void;
}

export const KeyManagementTable: React.FC<KeyManagementTableProps> = ({
  keys,
  isLoading,
  workspaceId,
  onRevokeKey,
  onRotateKey,
  onSelectKey,
  onOpenCreateModal,
}) => {
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'revoked' | 'expired'>('all');
  const [copiedKeyId, setCopiedKeyId] = useState<string | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const handleCopy = (e: React.MouseEvent, text: string, id: string) => {
    e.stopPropagation();
    navigator.clipboard.writeText(text);
    setCopiedKeyId(id);
    setTimeout(() => setCopiedKeyId(null), 2000);
  };

  const handleRevoke = async (e: React.MouseEvent, id: string, name: string) => {
    e.stopPropagation();
    if (confirm(`Are you sure you want to revoke API key "${name}"? This action takes effect immediately.`)) {
      setRevokingId(id);
      try {
        await onRevokeKey(id, name);
      } finally {
        setRevokingId(null);
      }
    }
  };

  const handleRotate = (e: React.MouseEvent, key: APIKey) => {
    e.stopPropagation();
    onRotateKey(key);
  };

  const filteredKeys = keys.filter((k) => {
    const matchesSearch =
      k.name.toLowerCase().includes(search.toLowerCase()) ||
      k.id.toLowerCase().includes(search.toLowerCase()) ||
      k.prefix.toLowerCase().includes(search.toLowerCase()) ||
      k.ownerId.toLowerCase().includes(search.toLowerCase());

    if (!matchesSearch) return false;

    const now = new Date();
    const isRevoked = Boolean(k.revokedAt);
    const isExpired = Boolean(
      (k.expiresAt && new Date(k.expiresAt) < now) ||
      (k.maxUses > 0 && k.totalRequests >= k.maxUses)
    );

    if (statusFilter === 'active') return !isRevoked && !isExpired && k.enabled;
    if (statusFilter === 'revoked') return isRevoked;
    if (statusFilter === 'expired') return isExpired;
    return true;
  });

  return (
    <div className="space-y-4">
      {/* Search and Filters Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-4 bg-surface border border-border rounded-xl">
        <div className="relative flex-1 max-w-md">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder="Search keys by name, prefix, owner or ID..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full pl-9 pr-4 py-2 bg-background border border-border rounded-lg text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:border-primary-500 font-sans"
          />
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {/* Status filter tabs */}
          <div className="flex bg-background border border-border rounded-lg p-0.5 text-xs font-medium">
            {(['all', 'active', 'revoked', 'expired'] as const).map((filter) => (
              <button
                key={filter}
                onClick={() => setStatusFilter(filter)}
                className={`px-3 py-1.5 rounded-md capitalize transition-colors ${
                  statusFilter === filter
                    ? 'bg-surface-hover text-white font-semibold'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                {filter}
              </button>
            ))}
          </div>

          {/* Export Dropdown */}
          <div className="flex items-center gap-1.5">
            <a
              href={`/v1/keys.export?workspaceId=${encodeURIComponent(workspaceId)}&format=csv`}
              download="keygate-keys.csv"
              className="px-2.5 py-1.5 text-xs font-medium text-slate-300 hover:text-white bg-background hover:bg-slate-800 border border-border rounded-lg transition-colors flex items-center gap-1.5"
              title="Export as CSV"
            >
              <Download className="w-3.5 h-3.5" />
              CSV
            </a>
            <a
              href={`/v1/keys.export?workspaceId=${encodeURIComponent(workspaceId)}&format=json`}
              download="keygate-keys.json"
              className="px-2.5 py-1.5 text-xs font-medium text-slate-300 hover:text-white bg-background hover:bg-slate-800 border border-border rounded-lg transition-colors flex items-center gap-1.5"
              title="Export as JSON"
            >
              <Download className="w-3.5 h-3.5" />
              JSON
            </a>
          </div>

          <button
            onClick={onOpenCreateModal}
            className="px-4 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-500 rounded-lg transition-colors flex items-center gap-2 shadow-sm"
          >
            <KeyRound className="w-4 h-4" />
            New Key
          </button>
        </div>
      </div>

      {/* Table Container */}
      <div className="bg-surface border border-border rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="bg-background/60 border-b border-border text-xs text-slate-400 uppercase tracking-wider font-semibold">
              <tr>
                <th className="py-3 px-4">Key Name & ID</th>
                <th className="py-3 px-4">Prefix / Pattern</th>
                <th className="py-3 px-4">Tier & Quota</th>
                <th className="py-3 px-4">Usage & Cap</th>
                <th className="py-3 px-4">Security</th>
                <th className="py-3 px-4">Status</th>
                <th className="py-3 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {isLoading ? (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-slate-400 text-sm">
                    Loading cryptographic keys from storage...
                  </td>
                </tr>
              ) : filteredKeys.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-12 text-center">
                    <div className="flex flex-col items-center justify-center text-slate-400">
                      <KeyRound className="w-8 h-8 text-slate-600 mb-2" />
                      <p className="font-medium text-slate-300">No API keys found</p>
                      <p className="text-xs text-slate-500 mt-1">
                        {search || statusFilter !== 'all'
                          ? 'Try adjusting your search criteria or status filter.'
                          : 'Issue your first prefixed cryptographic API key to begin.'}
                      </p>
                      {!search && statusFilter === 'all' && (
                        <button
                          onClick={onOpenCreateModal}
                          className="mt-4 px-3.5 py-1.5 text-xs font-medium text-white bg-primary-600 hover:bg-primary-500 rounded-lg transition-colors"
                        >
                          Create Key
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ) : (
                filteredKeys.map((key) => {
                  const now = new Date();
                  const isRevoked = Boolean(key.revokedAt);
                  const isExpired = Boolean(
                    (key.expiresAt && new Date(key.expiresAt) < now) ||
                    (key.maxUses > 0 && key.totalRequests >= key.maxUses)
                  );
                  const isActive = !isRevoked && !isExpired && key.enabled;

                  const tierColor =
                    key.tier === 'enterprise'
                      ? 'bg-emerald-950/70 text-emerald-300 border-emerald-800/40'
                      : key.tier === 'pro'
                      ? 'bg-indigo-950/70 text-indigo-300 border-indigo-800/40'
                      : 'bg-slate-800 text-slate-300 border-slate-700';

                  const quotaPct = key.maxUses > 0 ? Math.min(100, (key.totalRequests / key.maxUses) * 100) : 0;

                  return (
                    <tr
                      key={key.id}
                      onClick={() => onSelectKey(key.id)}
                      className="hover:bg-surface-hover/70 transition-colors cursor-pointer group"
                    >
                      {/* Name & ID */}
                      <td className="py-3.5 px-4">
                        <div className="font-medium text-white group-hover:text-primary-400 transition-colors flex items-center gap-1.5">
                          {key.name}
                          <SlidersHorizontal className="w-3 h-3 text-slate-500 opacity-0 group-hover:opacity-100 transition-opacity" />
                        </div>
                        <div className="text-xs font-mono text-slate-500 flex items-center gap-1.5 mt-0.5">
                          <span>{key.id}</span>
                          <button
                            onClick={(e) => handleCopy(e, key.id, key.id)}
                            className="text-slate-500 hover:text-slate-300 transition-colors"
                            title="Copy Key ID"
                          >
                            {copiedKeyId === key.id ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                          </button>
                        </div>
                      </td>

                      {/* Prefix */}
                      <td className="py-3.5 px-4 font-mono text-xs">
                        <span className="text-primary-400 font-semibold">{key.prefix}</span>
                        <span className="text-slate-500">_{key.workspaceId}_••••</span>
                      </td>

                      {/* Tier & Quota */}
                      <td className="py-3.5 px-4">
                        <div className="flex items-center gap-2">
                          <span className={`px-2 py-0.5 rounded text-[11px] font-medium border uppercase tracking-wider ${tierColor}`}>
                            {key.tier}
                          </span>
                          <span className="text-xs font-mono text-slate-400">
                            {key.rateLimit} req/{key.rateDuration}s
                          </span>
                        </div>
                      </td>

                      {/* Usage & Max Uses Bar */}
                      <td className="py-3.5 px-4">
                        <div className="text-xs font-mono text-slate-300">
                          {key.totalRequests.toLocaleString()}{' '}
                          <span className="text-slate-500">
                            {key.maxUses > 0 ? `/ ${key.maxUses}` : 'calls'}
                          </span>
                        </div>
                        {key.maxUses > 0 ? (
                          <div className="w-24 bg-slate-800 h-1.5 rounded-full mt-1.5 overflow-hidden">
                            <div
                              className={`h-full ${quotaPct >= 100 ? 'bg-rose-500' : 'bg-primary-500'}`}
                              style={{ width: `${quotaPct}%` }}
                            />
                          </div>
                        ) : (
                          <span className="text-[10px] text-slate-500">Unlimited quota</span>
                        )}
                      </td>

                      {/* Security (IP & Expiry) */}
                      <td className="py-3.5 px-4 text-xs">
                        <div className="flex items-center gap-1.5 text-slate-400">
                          <Globe className="w-3.5 h-3.5 text-slate-500" />
                          <span>
                            {key.ipAllowlist && key.ipAllowlist.length > 0
                              ? `${key.ipAllowlist.length} IP restricted`
                              : 'Any IP'}
                          </span>
                        </div>
                        <div className="flex items-center gap-1.5 text-slate-500 mt-1">
                          <Clock className="w-3.5 h-3.5" />
                          <span>
                            {key.expiresAt
                              ? `Expires ${new Date(key.expiresAt).toLocaleDateString()}`
                              : 'No expiry'}
                          </span>
                        </div>
                      </td>

                      {/* Status */}
                      <td className="py-3.5 px-4">
                        {isActive ? (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-950/60 text-emerald-400 border border-emerald-800/40">
                            <ShieldCheck className="w-3 h-3" />
                            Active
                          </span>
                        ) : isRevoked ? (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-rose-950/60 text-rose-400 border border-rose-800/40">
                            <Ban className="w-3 h-3" />
                            Revoked
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-amber-950/60 text-amber-400 border border-amber-800/40">
                            <AlertTriangle className="w-3 h-3" />
                            {key.maxUses > 0 && key.totalRequests >= key.maxUses ? 'Quota Full' : 'Expired'}
                          </span>
                        )}
                      </td>

                      {/* Actions */}
                      <td className="py-3.5 px-4 text-right">
                        <div className="flex items-center justify-end gap-1.5">
                          {isActive && (
                            <>
                              <button
                                onClick={(e) => handleRotate(e, key)}
                                className="px-2 py-1 text-xs font-medium text-amber-400 hover:text-amber-300 hover:bg-amber-950/40 border border-amber-900/40 rounded transition-colors flex items-center gap-1"
                                title="Zero-Downtime Rotation"
                              >
                                <RefreshCw className="w-3 h-3" />
                                Rotate
                              </button>
                              <button
                                onClick={(e) => handleRevoke(e, key.id, key.name)}
                                disabled={revokingId === key.id}
                                className="px-2 py-1 text-xs font-medium text-rose-400 hover:text-rose-300 hover:bg-rose-950/40 border border-rose-900/40 rounded transition-colors disabled:opacity-50"
                              >
                                {revokingId === key.id ? '...' : 'Revoke'}
                              </button>
                            </>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
