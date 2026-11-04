import React, { useState } from 'react';
import { UsageEvent } from '../types';
import { Activity, Trash2 } from 'lucide-react';

interface LiveRequestFeedProps {
  events: UsageEvent[];
  isConnected: boolean;
  onClear: () => void;
}

export const LiveRequestFeed: React.FC<LiveRequestFeedProps> = ({ events, isConnected, onClear }) => {
  const [statusFilter, setStatusFilter] = useState<'all' | '2xx' | '429' | '4xx'>('all');

  const filtered = events.filter((ev) => {
    if (statusFilter === '2xx') return ev.statusCode >= 200 && ev.statusCode < 300;
    if (statusFilter === '429') return ev.statusCode === 429;
    if (statusFilter === '4xx') return ev.statusCode >= 400 && ev.statusCode < 500 && ev.statusCode !== 429;
    return true;
  });

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-4 bg-surface border border-border rounded-xl">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-950/60 border border-indigo-800/40 rounded-lg text-indigo-400">
            <Activity className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-base font-semibold text-white">Live Telemetry Feed</h2>
              <span
                className={`w-2 h-2 rounded-full ${
                  isConnected ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400'
                }`}
              />
            </div>
            <p className="text-xs text-slate-400 mt-0.5">Real-time Server-Sent Events stream from port 8000</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Status filter */}
          <div className="flex bg-background border border-border rounded-lg p-0.5 text-xs font-medium">
            {(['all', '2xx', '429', '4xx'] as const).map((filter) => (
              <button
                key={filter}
                onClick={() => setStatusFilter(filter)}
                className={`px-3 py-1.5 rounded-md uppercase transition-colors ${
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
            onClick={onClear}
            className="p-2 text-slate-400 hover:text-rose-400 bg-background border border-border rounded-lg transition-colors"
            title="Clear Feed"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Feed Table */}
      <div className="bg-surface border border-border rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs font-mono">
            <thead className="bg-background/60 border-b border-border text-[11px] text-slate-400 uppercase tracking-wider font-semibold">
              <tr>
                <th className="py-3 px-4">Status</th>
                <th className="py-3 px-4">Method & Path</th>
                <th className="py-3 px-4">Key ID</th>
                <th className="py-3 px-4">Client IP</th>
                <th className="py-3 px-4">Latency</th>
                <th className="py-3 px-4 text-right">Timestamp</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-12 text-center text-slate-500 font-sans text-xs">
                    {events.length === 0
                      ? 'No events received yet. Start sending requests to /v1/keys.verifyKey or the reverse proxy.'
                      : 'No events match the selected status filter.'}
                  </td>
                </tr>
              ) : (
                filtered.map((ev, i) => {
                  const is2xx = ev.statusCode >= 200 && ev.statusCode < 300;
                  const is429 = ev.statusCode === 429;

                  return (
                    <tr key={i} className="hover:bg-surface-hover/50 transition-colors">
                      <td className="py-2.5 px-4">
                        <span
                          className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                            is2xx
                              ? 'bg-emerald-950 text-emerald-400 border border-emerald-800/40'
                              : is429
                              ? 'bg-amber-950 text-amber-400 border border-amber-800/40'
                              : 'bg-rose-950 text-rose-400 border border-rose-800/40'
                          }`}
                        >
                          {ev.statusCode}
                        </span>
                      </td>
                      <td className="py-2.5 px-4">
                        <span className="font-bold text-slate-200 mr-2">{ev.method}</span>
                        <span className="text-slate-400">{ev.path}</span>
                      </td>
                      <td className="py-2.5 px-4 text-slate-400">
                        {ev.keyId ? ev.keyId : <span className="text-slate-600 italic">unauthenticated</span>}
                      </td>
                      <td className="py-2.5 px-4 text-slate-400">{ev.clientIp || '127.0.0.1'}</td>
                      <td className="py-2.5 px-4">
                        <span className="text-emerald-400 font-medium">{ev.latencyMs.toFixed(2)} ms</span>
                      </td>
                      <td className="py-2.5 px-4 text-right text-slate-500">
                        {new Date(ev.timestamp).toLocaleTimeString([], {
                          hour: '2-digit',
                          minute: '2-digit',
                          second: '2-digit',
                        })}
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
