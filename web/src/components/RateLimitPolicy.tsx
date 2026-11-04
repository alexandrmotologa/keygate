import React from 'react';
import { Layers, Zap } from 'lucide-react';

export const RateLimitPolicy: React.FC = () => {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="p-6 bg-surface border border-border rounded-xl">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-950/60 border border-indigo-800/40 rounded-lg text-indigo-400">
            <Layers className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-white tracking-tight">Rate Limiting Policies & Algorithms</h1>
            <p className="text-sm text-slate-400 mt-1">
              Multi-tier sliding-window token-bucket throttling with IETF standard response headers.
            </p>
          </div>
        </div>
      </div>

      {/* Tier Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
        {/* Free */}
        <div className="p-6 bg-surface border border-border rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <span className="px-2.5 py-1 rounded text-xs font-bold uppercase tracking-wider bg-slate-800 text-slate-300 border border-slate-700">
              Free Tier
            </span>
            <span className="text-xs font-mono text-slate-400">Default</span>
          </div>
          <div>
            <div className="text-2xl font-bold text-white font-mono">10 <span className="text-sm text-slate-400 font-normal">req / min</span></div>
            <p className="text-xs text-slate-400 mt-1">Light developer evaluation and exploratory prototyping.</p>
          </div>
          <div className="pt-3 border-t border-border space-y-2 text-xs text-slate-300">
            <div className="flex justify-between">
              <span className="text-slate-400">Sliding Window:</span>
              <span className="font-mono">60 seconds</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Burst Tolerance:</span>
              <span className="font-mono">2 requests</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Header Support:</span>
              <span className="text-emerald-400">IETF Draft</span>
            </div>
          </div>
        </div>

        {/* Pro */}
        <div className="p-6 bg-surface border border-primary-500/40 rounded-xl space-y-4 relative overflow-hidden">
          <div className="absolute top-0 right-0 bg-primary-600 text-white text-[10px] uppercase font-bold tracking-wider px-3 py-0.5 rounded-bl">
            Recommended
          </div>
          <div className="flex items-center justify-between">
            <span className="px-2.5 py-1 rounded text-xs font-bold uppercase tracking-wider bg-primary-950/70 text-primary-300 border border-primary-800/40">
              Pro Tier
            </span>
            <span className="text-xs font-mono text-primary-400">Production</span>
          </div>
          <div>
            <div className="text-2xl font-bold text-white font-mono">500 <span className="text-sm text-slate-400 font-normal">req / min</span></div>
            <p className="text-xs text-slate-400 mt-1">Production SaaS APIs, mobile backends, and agentic workflows.</p>
          </div>
          <div className="pt-3 border-t border-border space-y-2 text-xs text-slate-300">
            <div className="flex justify-between">
              <span className="text-slate-400">Sliding Window:</span>
              <span className="font-mono">60 seconds</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Burst Tolerance:</span>
              <span className="font-mono">25 requests</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Header Support:</span>
              <span className="text-emerald-400">IETF Draft</span>
            </div>
          </div>
        </div>

        {/* Enterprise */}
        <div className="p-6 bg-surface border border-border rounded-xl space-y-4">
          <div className="flex items-center justify-between">
            <span className="px-2.5 py-1 rounded text-xs font-bold uppercase tracking-wider bg-emerald-950/70 text-emerald-300 border border-emerald-800/40">
              Enterprise Tier
            </span>
            <span className="text-xs font-mono text-emerald-400">High Volume</span>
          </div>
          <div>
            <div className="text-2xl font-bold text-white font-mono">10,000 <span className="text-sm text-slate-400 font-normal">req / min</span></div>
            <p className="text-xs text-slate-400 mt-1">Large-scale distributed systems and enterprise API pipelines.</p>
          </div>
          <div className="pt-3 border-t border-border space-y-2 text-xs text-slate-300">
            <div className="flex justify-between">
              <span className="text-slate-400">Sliding Window:</span>
              <span className="font-mono">60 seconds</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Burst Tolerance:</span>
              <span className="font-mono">500 requests</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Header Support:</span>
              <span className="text-emerald-400">IETF Draft</span>
            </div>
          </div>
        </div>
      </div>

      {/* Algorithm Deep Dive */}
      <div className="p-6 bg-surface border border-border rounded-xl space-y-4">
        <h2 className="text-base font-semibold text-white flex items-center gap-2">
          <Zap className="w-4 h-4 text-amber-400" />
          Sliding Window Algorithm Breakdown
        </h2>
        <p className="text-xs text-slate-400 leading-relaxed">
          Traditional fixed-window algorithms create boundary vulnerability: clients can execute double their quota
          by sending bursts right before and right after the window rollover. KeyGate prevents this by smoothing request density across adjacent time windows:
        </p>

        <div className="p-4 bg-background border border-border rounded-xl font-mono text-xs text-slate-300 space-y-2">
          <div className="text-primary-400 font-semibold">// Sliding-Window Interpolation Formula:</div>
          <div>Weight = 1.0 - (ElapsedInCurrentWindow / WindowSize)</div>
          <div>EstimatedCount = Floor(PreviousWindowCount * Weight) + CurrentWindowCount</div>
          <div className="text-slate-500">// If EstimatedCount + 1 &gt; Limit, reject with 429 and Retry-After</div>
        </div>
      </div>
    </div>
  );
};
