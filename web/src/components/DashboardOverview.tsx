import React from 'react';
import { AnalyticsSummary, UsageEvent } from '../types';
import { KeyRound, Activity, Zap, ShieldAlert, CheckCircle2 } from 'lucide-react';

interface DashboardOverviewProps {
  analytics: AnalyticsSummary | null;
  liveEvents: UsageEvent[];
  isConnected: boolean;
  onOpenCreateModal: () => void;
  onNavigateToTester: () => void;
}

export const DashboardOverview: React.FC<DashboardOverviewProps> = ({
  analytics,
  liveEvents,
  isConnected,
  onOpenCreateModal,
  onNavigateToTester,
}) => {
  const totalKeys = analytics?.totalKeys ?? 0;
  const activeKeys = analytics?.activeKeys ?? 0;
  const totalRequests = analytics?.totalRequests ?? 0;
  const successRequests = analytics?.successRequests ?? 0;
  const rateLimited = analytics?.rateLimitedRequests ?? 0;
  const unauthorized = analytics?.unauthorizedRequests ?? 0;
  const p99 = analytics?.p99LatencyMs ?? 0;

  const successRate = totalRequests > 0 ? ((successRequests / totalRequests) * 100).toFixed(1) : '100.0';

  const buckets = analytics?.hourlyBuckets ?? [];
  const maxBucketTotal = Math.max(...buckets.map((b) => b.totalRequests), 10);

  return (
    <div className="space-y-6">
      {/* Top Banner & Quick Actions */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 p-6 bg-surface border border-border rounded-xl">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-bold text-white tracking-tight">System Control Plane</h1>
            <span
              className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${
                isConnected
                  ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/40'
                  : 'bg-amber-950/60 text-amber-400 border border-amber-800/40'
              }`}
            >
              <span className={`w-1.5 h-1.5 rounded-full ${isConnected ? 'bg-emerald-400 animate-pulse' : 'bg-amber-400'}`} />
              {isConnected ? 'Telemetry Stream Active' : 'Connecting Stream...'}
            </span>
          </div>
          <p className="text-sm text-slate-400 mt-1">
            Sub-millisecond API key validation engine and sliding-window rate limiting gateway.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={onNavigateToTester}
            className="px-4 py-2 text-sm font-medium text-slate-200 bg-surface-hover hover:bg-slate-800 border border-border rounded-lg transition-colors flex items-center gap-2"
          >
            <Zap className="w-4 h-4 text-amber-400" />
            API Playground
          </button>
          <button
            onClick={onOpenCreateModal}
            className="px-4 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-500 rounded-lg shadow-sm shadow-primary-900/40 transition-colors flex items-center gap-2"
          >
            <KeyRound className="w-4 h-4" />
            Create API Key
          </button>
        </div>
      </div>

      {/* KPI Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Card 1 */}
        <div className="p-5 bg-surface border border-border rounded-xl">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Active Keys</span>
            <div className="p-2 bg-primary-950/50 rounded-lg border border-primary-800/30">
              <KeyRound className="w-4 h-4 text-primary-400" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white tracking-tight">{activeKeys}</span>
            <span className="text-xs text-slate-400">/ {totalKeys} total</span>
          </div>
          <div className="mt-2 text-xs text-emerald-400 flex items-center gap-1 font-mono">
            <CheckCircle2 className="w-3.5 h-3.5" />
            {(totalKeys > 0 ? (activeKeys / totalKeys) * 100 : 100).toFixed(0)}% healthy status
          </div>
        </div>

        {/* Card 2 */}
        <div className="p-5 bg-surface border border-border rounded-xl">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Total Throughput</span>
            <div className="p-2 bg-indigo-950/50 rounded-lg border border-indigo-800/30">
              <Activity className="w-4 h-4 text-indigo-400" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white tracking-tight">{totalRequests.toLocaleString()}</span>
            <span className="text-xs text-slate-400">requests</span>
          </div>
          <div className="mt-2 text-xs text-slate-400 font-mono">
            {successRate}% 200 OK success rate
          </div>
        </div>

        {/* Card 3 */}
        <div className="p-5 bg-surface border border-border rounded-xl">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">P99 Latency</span>
            <div className="p-2 bg-emerald-950/50 rounded-lg border border-emerald-800/30">
              <Zap className="w-4 h-4 text-emerald-400" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white font-mono tracking-tight">
              {p99 > 0 ? p99.toFixed(2) : '< 0.50'} <span className="text-base font-normal text-slate-400">ms</span>
            </span>
          </div>
          <div className="mt-2 text-xs text-emerald-400 font-medium">
            In-Memory Ristretto Cache
          </div>
        </div>

        {/* Card 4 */}
        <div className="p-5 bg-surface border border-border rounded-xl">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">Throttled & Rejected</span>
            <div className="p-2 bg-rose-950/50 rounded-lg border border-rose-800/30">
              <ShieldAlert className="w-4 h-4 text-rose-400" />
            </div>
          </div>
          <div className="mt-3 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white tracking-tight">
              {(rateLimited + unauthorized).toLocaleString()}
            </span>
            <span className="text-xs text-slate-400">blocked</span>
          </div>
          <div className="mt-2 text-xs text-slate-400 font-mono flex gap-2">
            <span className="text-amber-400">{rateLimited} 429</span>
            <span>•</span>
            <span className="text-rose-400">{unauthorized} 401/403</span>
          </div>
        </div>
      </div>

      {/* 24-Hour Request Volume Chart */}
      <div className="p-6 bg-surface border border-border rounded-xl">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-base font-semibold text-white">24-Hour Verification Volume</h2>
            <p className="text-xs text-slate-400 mt-0.5">Aggregate request density grouped by hour</p>
          </div>
          <div className="flex items-center gap-4 text-xs font-medium">
            <span className="flex items-center gap-1.5 text-slate-300">
              <span className="w-2.5 h-2.5 bg-primary-500 rounded-sm" /> Success (2xx)
            </span>
            <span className="flex items-center gap-1.5 text-slate-300">
              <span className="w-2.5 h-2.5 bg-amber-500 rounded-sm" /> Rate Limited (429)
            </span>
            <span className="flex items-center gap-1.5 text-slate-300">
              <span className="w-2.5 h-2.5 bg-rose-500 rounded-sm" /> Rejected (401/403)
            </span>
          </div>
        </div>

        {buckets.length === 0 ? (
          <div className="h-44 flex flex-col items-center justify-center border border-dashed border-border rounded-lg text-slate-500 text-sm">
            <Activity className="w-6 h-6 mb-2 text-slate-600" />
            No traffic recorded in the current 24-hour window yet. Send requests via the proxy or API Playground to populate.
          </div>
        ) : (
          <div className="h-44 flex items-end gap-2 pt-6 pb-2 px-2 border-b border-border">
            {buckets.map((b, i) => {
              const totalHeight = Math.max(4, (b.totalRequests / maxBucketTotal) * 140);
              const successRatio = b.totalRequests > 0 ? b.successRequests / b.totalRequests : 1;
              const rateLimitRatio = b.totalRequests > 0 ? b.rateLimitedRequests / b.totalRequests : 0;
              const rejectRatio = b.totalRequests > 0 ? b.unauthorizedRequests / b.totalRequests : 0;

              const hourLabel = new Date(b.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

              return (
                <div key={i} className="flex-1 flex flex-col items-center group relative h-full justify-end">
                  {/* Tooltip */}
                  <div className="absolute -top-12 hidden group-hover:flex flex-col items-center z-20 pointer-events-none">
                    <div className="bg-slate-900 border border-slate-700 text-slate-200 text-xs px-2 py-1 rounded shadow-lg whitespace-nowrap font-mono">
                      {hourLabel}: {b.totalRequests} reqs (avg {b.avgLatencyMs.toFixed(2)}ms)
                    </div>
                  </div>

                  {/* Stacked bar */}
                  <div className="w-full max-w-[28px] rounded-t overflow-hidden flex flex-col-reverse" style={{ height: `${totalHeight}px` }}>
                    <div style={{ height: `${successRatio * 100}%` }} className="bg-primary-500 w-full" />
                    <div style={{ height: `${rateLimitRatio * 100}%` }} className="bg-amber-500 w-full" />
                    <div style={{ height: `${rejectRatio * 100}%` }} className="bg-rose-500 w-full" />
                  </div>
                  <span className="text-[10px] text-slate-500 mt-2 font-mono">{hourLabel}</span>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Split Section: Tiers & Recent Live Stream */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Tier Distribution */}
        <div className="p-6 bg-surface border border-border rounded-xl lg:col-span-1">
          <h2 className="text-base font-semibold text-white">Tier Distribution</h2>
          <p className="text-xs text-slate-400 mt-0.5">Quota enforcement breakdown</p>

          <div className="mt-5 space-y-4">
            {[
              { name: 'Enterprise', tier: 'enterprise', limit: '10,000 / min', color: 'bg-emerald-500', text: 'text-emerald-400' },
              { name: 'Pro Tier', tier: 'pro', limit: '500 / min', color: 'bg-indigo-500', text: 'text-indigo-400' },
              { name: 'Free Tier', tier: 'free', limit: '10 / min', color: 'bg-slate-400', text: 'text-slate-400' },
            ].map((t) => {
              const count = analytics?.requestsByTier?.[t.tier] ?? 0;
              const pct = totalRequests > 0 ? ((count / totalRequests) * 100).toFixed(0) : '0';

              return (
                <div key={t.tier} className="p-3 bg-background border border-border rounded-lg">
                  <div className="flex items-center justify-between text-xs font-medium">
                    <span className="text-white font-medium">{t.name}</span>
                    <span className="font-mono text-slate-400">{t.limit}</span>
                  </div>
                  <div className="w-full bg-slate-800 h-1.5 rounded-full mt-2 overflow-hidden">
                    <div className={`${t.color} h-full rounded-full`} style={{ width: `${pct}%` }} />
                  </div>
                  <div className="flex justify-between items-center text-[11px] text-slate-400 mt-1 font-mono">
                    <span>{count.toLocaleString()} requests</span>
                    <span>{pct}%</span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Live SSE Mini-Feed */}
        <div className="p-6 bg-surface border border-border rounded-xl lg:col-span-2">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-base font-semibold text-white">Live Request Stream</h2>
              <p className="text-xs text-slate-400 mt-0.5">Sub-5ms event feed delivered via Server-Sent Events</p>
            </div>
            <span className="text-xs font-mono text-slate-400">{liveEvents.length} events in memory</span>
          </div>

          <div className="space-y-2 max-h-[260px] overflow-y-auto pr-1">
            {liveEvents.length === 0 ? (
              <div className="p-8 text-center border border-dashed border-border rounded-lg text-slate-500 text-xs">
                Awaiting incoming requests. Stream will update automatically when traffic hits port 8000.
              </div>
            ) : (
              liveEvents.slice(0, 7).map((ev, idx) => {
                const is2xx = ev.statusCode >= 200 && ev.statusCode < 300;
                const is429 = ev.statusCode === 429;

                return (
                  <div
                    key={idx}
                    className="flex items-center justify-between p-2.5 bg-background border border-border rounded-lg text-xs font-mono hover:border-border-light transition-colors"
                  >
                    <div className="flex items-center gap-3">
                      <span
                        className={`px-1.5 py-0.5 rounded text-[10px] font-bold ${
                          is2xx
                            ? 'bg-emerald-950 text-emerald-400 border border-emerald-800/40'
                            : is429
                            ? 'bg-amber-950 text-amber-400 border border-amber-800/40'
                            : 'bg-rose-950 text-rose-400 border border-rose-800/40'
                        }`}
                      >
                        {ev.statusCode}
                      </span>
                      <span className="font-semibold text-slate-300">{ev.method}</span>
                      <span className="text-slate-400 truncate max-w-[200px]">{ev.path}</span>
                    </div>
                    <div className="flex items-center gap-4 text-slate-400">
                      <span className="text-emerald-400 font-medium">{ev.latencyMs.toFixed(2)} ms</span>
                      <span className="text-slate-500 text-[11px]">
                        {new Date(ev.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                      </span>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
