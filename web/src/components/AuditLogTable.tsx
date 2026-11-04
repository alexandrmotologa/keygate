import React, { useState, useEffect } from 'react';
import { AuditLog } from '../types';
import { fetchAuditLogs } from '../services/api';
import { ShieldAlert, RefreshCw, Clock } from 'lucide-react';

interface AuditLogTableProps {
  workspaceId: string;
}

export const AuditLogTable: React.FC<AuditLogTableProps> = ({ workspaceId }) => {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const loadLogs = async () => {
    setIsLoading(true);
    try {
      const data = await fetchAuditLogs(workspaceId);
      setLogs(data.logs || []);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadLogs();
  }, [workspaceId]);

  const getActionBadge = (action: string) => {
    switch (action) {
      case 'key.created':
        return 'bg-emerald-950/70 text-emerald-400 border-emerald-800/40';
      case 'key.revoked':
        return 'bg-rose-950/70 text-rose-400 border-rose-800/40';
      case 'key.rotated':
        return 'bg-amber-950/70 text-amber-400 border-amber-800/40';
      case 'key.updated':
        return 'bg-indigo-950/70 text-indigo-400 border-indigo-800/40';
      case 'workspace.created':
        return 'bg-purple-950/70 text-purple-400 border-purple-800/40';
      default:
        return 'bg-slate-800 text-slate-300 border-slate-700';
    }
  };

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between p-4 bg-surface border border-border rounded-xl">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-950/60 border border-indigo-800/40 rounded-lg text-indigo-400">
            <ShieldAlert className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-base font-semibold text-white">Security Audit Trail</h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Immutable ledger of administrative mutations in workspace {workspaceId}
            </p>
          </div>
        </div>

        <button
          onClick={loadLogs}
          className="p-2 text-slate-400 hover:text-white bg-background border border-border rounded-lg transition-colors"
          title="Refresh Logs"
        >
          <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Table */}
      <div className="bg-surface border border-border rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs font-mono">
            <thead className="bg-background/60 border-b border-border text-[11px] text-slate-400 uppercase tracking-wider font-semibold font-sans">
              <tr>
                <th className="py-3 px-4">Timestamp</th>
                <th className="py-3 px-4">Action</th>
                <th className="py-3 px-4">Actor (IP / User)</th>
                <th className="py-3 px-4">Target ID</th>
                <th className="py-3 px-4">Mutation Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {isLoading ? (
                <tr>
                  <td colSpan={5} className="py-12 text-center text-slate-500 font-sans text-xs">
                    Loading audit trail from database...
                  </td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td colSpan={5} className="py-12 text-center text-slate-500 font-sans text-xs">
                    No administrative audit events recorded yet.
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr key={log.id} className="hover:bg-surface-hover/50 transition-colors">
                    <td className="py-3 px-4 text-slate-400 flex items-center gap-1.5 whitespace-nowrap">
                      <Clock className="w-3.5 h-3.5 text-slate-500" />
                      <span>{new Date(log.timestamp).toLocaleString()}</span>
                    </td>
                    <td className="py-3 px-4">
                      <span className={`px-2 py-0.5 rounded text-[10px] font-bold border ${getActionBadge(log.action)}`}>
                        {log.action}
                      </span>
                    </td>
                    <td className="py-3 px-4 text-slate-300">{log.actor}</td>
                    <td className="py-3 px-4 text-primary-400">{log.targetId}</td>
                    <td className="py-3 px-4 text-slate-400">
                      {Object.entries(log.details || {}).map(([k, v]) => (
                        <span key={k} className="mr-3">
                          <span className="text-slate-500">{k}:</span> <span className="text-slate-300">{v}</span>
                        </span>
                      ))}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
