import React, { useState } from 'react';
import { APIKey } from '../types';
import { Search, KeyRound, Copy, Check, Ban, ShieldCheck, Globe, Clock, AlertTriangle } from 'lucide-react';

interface KeyManagementTableProps {
  keys: APIKey[];
  isLoading: boolean;
  onRevokeKey: (keyId: string) => Promise<void>;
  onOpenCreateModal: () => void;
  onRefresh: () => void;
}

export const KeyManagementTable: React.FC<KeyManagementTableProps> = ({
  keys,
  isLoading,
  onRevokeKey,
  onOpenCreateModal,
  onRefresh,
}) => {
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'revoked' | 'expired'>('all');
  const [copiedKeyId, setCopiedKeyId] = useState<string | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKeyId(id);
    setTimeout(() => setCopiedKeyId(null), 2000);
  };

  const handleRevoke = async (id: string, name: string) => {
    if (confirm(`Are you sure you want to revoke API key "${name}"? This action takes effect immediately and cannot be undone.`)) {
      setRevokingId(id);
      try {
        await onRevokeKey(id);
        onRefresh();
      } finally {
        setRevokingId(null);
      }
    }
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
    const isExpired = Boolean(k.expiresAt && new Date(k.expiresAt) < now);

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

        <div className="flex items-center gap-2">
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

          <button
            onClick={onOpenCreateModal}
            className="px-4 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-500 rounded-lg transition-colors flex items-center gap-2"
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
                <th className="py-3 px-4">Scopes</th>
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
                  const isExpired = Boolean(key.expiresAt && new Date(key.expiresAt) < now);
                  const isActive = !isRevoked && !isExpired && key.enabled;

                  const tierColor =
                    key.tier === 'enterprise'
                      ? 'bg-emerald-950/70 text-emerald-300 border-emerald-800/40'
                      : key.tier === 'pro'
                      ? 'bg-indigo-950/70 text-indigo-300 border-indigo-800/40'
                      : 'bg-slate-800 text-slate-300 border-slate-700';

                  return (
                    <tr key={key.id} className="hover:bg-surface-hover/50 transition-colors">
                      {/* Name & ID */}
                      <td className="py-3.5 px-4">
                        <div className="font-medium text-white">{key.name}</div>
                        <div className="text-xs font-mono text-slate-500 flex items-center gap-1.5 mt-0.5">
                          <span>{key.id}</span>
                          <button
                            onClick={() => handleCopy(key.id, key.id)}
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
                        <div className="text-[11px] text-slate-500 mt-1 font-mono">
                          {key.totalRequests.toLocaleString()} requests executed
                        </div>
                      </td>

                      {/* Scopes */}
                      <td className="py-3.5 px-4">
                        <div className="flex flex-wrap gap-1 max-w-[200px]">
                          {key.scopes && key.scopes.length > 0 ? (
                            key.scopes.map((s, idx) => (
                              <span
                                key={idx}
                                className="px-1.5 py-0.5 bg-background border border-border rounded text-[10px] font-mono text-slate-300"
                              >
                                {s}
                              </span>
                            ))
                          ) : (
                            <span className="text-xs text-slate-500 italic">all (wildcard)</span>
                          )}
                        </div>
                      </td>

                      {/* Security (IP & Expiry) */}
                      <td className="py-3.5 px-4 text-xs">
                        <div className="flex items-center gap-1.5 text-slate-400">
                          <Globe className="w-3.5 h-3.5 text-slate-500" />
                          <span>
                            {key.ipAllowlist && key.ipAllowlist.length > 0
                              ? `${key.ipAllowlist.length} IP restricted`
                              : 'Any IP allowed'}
                          </span>
                        </div>
                        <div className="flex items-center gap-1.5 text-slate-500 mt-1">
                          <Clock className="w-3.5 h-3.5" />
                          <span>
                            {key.expiresAt
                              ? `Expires ${new Date(key.expiresAt).toLocaleDateString()}`
                              : 'No expiration'}
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
                            Expired
                          </span>
                        )}
                      </td>

                      {/* Actions */}
                      <td className="py-3.5 px-4 text-right">
                        {isActive && (
                          <button
                            onClick={() => handleRevoke(key.id, key.name)}
                            disabled={revokingId === key.id}
                            className="px-2.5 py-1 text-xs font-medium text-rose-400 hover:text-rose-300 hover:bg-rose-950/40 border border-rose-900/40 rounded transition-colors disabled:opacity-50"
                          >
                            {revokingId === key.id ? 'Revoking...' : 'Revoke'}
                          </button>
                        )}
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
