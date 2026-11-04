import React, { useState } from 'react';
import { verifyKey } from '../services/api';
import { VerifyKeyResult } from '../types';
import { CodeGenerator } from './CodeGenerator';
import { Zap, Play, CheckCircle2, XCircle, Clock, ShieldCheck } from 'lucide-react';

export const ApiTester: React.FC = () => {
  const [apiKeyInput, setApiKeyInput] = useState('');
  const [workspaceId, setWorkspaceId] = useState('ws_default');
  const [requiredScopes, setRequiredScopes] = useState('');
  const [isTesting, setIsTesting] = useState(false);
  const [result, setResult] = useState<VerifyKeyResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleTestVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!apiKeyInput.trim()) return;

    setIsTesting(true);
    setError(null);
    setResult(null);

    try {
      const scopes = requiredScopes
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);

      const res = await verifyKey({
        key: apiKeyInput.trim(),
        workspaceId: workspaceId.trim() || undefined,
        requiredScopes: scopes.length > 0 ? scopes : undefined,
      });

      setResult(res);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Verification network error');
      }
    } finally {
      setIsTesting(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Intro Header */}
      <div className="p-6 bg-surface border border-border rounded-xl">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-amber-950/60 border border-amber-800/40 rounded-lg text-amber-400">
            <Zap className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-white tracking-tight">API Verification Playground</h1>
            <p className="text-sm text-slate-400 mt-1">
              Verify cryptographic keys and simulate gateway authorization directly in your browser.
            </p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Test Form */}
        <div className="p-6 bg-surface border border-border rounded-xl space-y-5">
          <h2 className="text-base font-semibold text-white flex items-center gap-2">
            <span>Direct Verification Request</span>
            <span className="text-xs font-mono font-normal text-slate-400 px-2 py-0.5 bg-background rounded border border-border">
              POST /v1/keys.verifyKey
            </span>
          </h2>

          <form onSubmit={handleTestVerify} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
                Plaintext API Key
              </label>
              <input
                type="text"
                required
                placeholder="sk_live_ws_default_..."
                value={apiKeyInput}
                onChange={(e) => setApiKeyInput(e.target.value)}
                className="w-full px-3.5 py-2.5 bg-background border border-border rounded-lg text-sm text-white font-mono placeholder-slate-600 focus:outline-none focus:border-primary-500"
              />
            </div>

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
                  Required Scopes (Optional)
                </label>
                <input
                  type="text"
                  placeholder="e.g. read:users"
                  value={requiredScopes}
                  onChange={(e) => setRequiredScopes(e.target.value)}
                  className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white font-mono placeholder-slate-600 focus:outline-none focus:border-primary-500"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={isTesting || !apiKeyInput.trim()}
              className="w-full py-2.5 px-4 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-500 rounded-lg shadow-sm shadow-primary-900/40 transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
            >
              <Play className="w-4 h-4 fill-current" />
              {isTesting ? 'Verifying Key...' : 'Execute Key Verification'}
            </button>
          </form>

          {/* Proxy Info Banner */}
          <div className="p-4 bg-background border border-border rounded-lg space-y-2 text-xs">
            <span className="font-semibold text-slate-300 flex items-center gap-1.5">
              <ShieldCheck className="w-4 h-4 text-emerald-400" />
              Inline Reverse Proxy Testing
            </span>
            <p className="text-slate-400 leading-relaxed">
              To test the inline transparent proxy, pass this key as an HTTP header to port 8000:
            </p>
            <div className="font-mono text-[11px] p-2 bg-surface rounded border border-border text-slate-300 overflow-x-auto">
              curl -H &quot;Authorization: Bearer {apiKeyInput || 'sk_live_...'}&quot; http://localhost:8000/
            </div>
          </div>
        </div>

        {/* Verification Result Display */}
        <div className="p-6 bg-surface border border-border rounded-xl space-y-4 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-semibold text-white">Verification Response</h2>
              {result && (
                <div className="flex items-center gap-2 text-xs font-mono">
                  <span className="flex items-center gap-1 text-slate-400">
                    <Clock className="w-3.5 h-3.5 text-primary-400" />
                    {result.durationUs ? `${result.durationUs} µs` : '< 1 ms'}
                  </span>
                  <span
                    className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                      result.cached
                        ? 'bg-emerald-950 text-emerald-400 border border-emerald-800/40'
                        : 'bg-indigo-950 text-indigo-400 border border-indigo-800/40'
                    }`}
                  >
                    {result.cached ? 'CACHE HIT' : 'CACHE MISS (SQL)'}
                  </span>
                </div>
              )}
            </div>

            {error ? (
              <div className="p-4 bg-rose-950/40 border border-rose-800/50 rounded-lg text-sm text-rose-300">
                {error}
              </div>
            ) : !result ? (
              <div className="h-64 flex flex-col items-center justify-center border border-dashed border-border rounded-lg text-slate-500 text-xs text-center p-6">
                <Zap className="w-8 h-8 text-slate-600 mb-2" />
                Provide a key on the left and click &quot;Execute Key Verification&quot; to inspect sub-millisecond validation metrics and headers.
              </div>
            ) : (
              <div className="space-y-4">
                {/* Status Callout */}
                <div
                  className={`p-4 rounded-xl border flex items-center justify-between ${
                    result.valid
                      ? 'bg-emerald-950/40 border-emerald-800/50 text-emerald-300'
                      : 'bg-rose-950/40 border-rose-800/50 text-rose-300'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    {result.valid ? (
                      <CheckCircle2 className="w-6 h-6 text-emerald-400 shrink-0" />
                    ) : (
                      <XCircle className="w-6 h-6 text-rose-400 shrink-0" />
                    )}
                    <div>
                      <div className="font-bold text-sm tracking-wide">
                        {result.valid ? 'KEY VALID & AUTHORIZED' : `REJECTED: ${result.code}`}
                      </div>
                      <div className="text-xs text-slate-300 mt-0.5">
                        {result.message || (result.valid ? 'Key is active with valid quota.' : 'Key verification failed.')}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Quota & Rate Limit Headers */}
                {result.rateLimit && (
                  <div className="grid grid-cols-3 gap-3 p-3 bg-background border border-border rounded-lg text-xs font-mono">
                    <div>
                      <span className="text-slate-500 block text-[10px] uppercase">RateLimit-Limit</span>
                      <span className="text-white font-bold">{result.rateLimit.limit} req</span>
                    </div>
                    <div>
                      <span className="text-slate-500 block text-[10px] uppercase">RateLimit-Remaining</span>
                      <span className="text-emerald-400 font-bold">{result.rateLimit.remaining} req</span>
                    </div>
                    <div>
                      <span className="text-slate-500 block text-[10px] uppercase">RateLimit-Reset</span>
                      <span className="text-white font-bold">{result.rateLimit.reset} s</span>
                    </div>
                  </div>
                )}

                {/* Full JSON Dump */}
                <div>
                  <span className="text-xs font-semibold uppercase tracking-wider text-slate-400 block mb-1">
                    Raw JSON Output
                  </span>
                  <pre className="p-3 bg-background border border-border rounded-lg font-mono text-xs text-slate-300 overflow-x-auto max-h-48">
                    {JSON.stringify(result, null, 2)}
                  </pre>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Multi-Language Code Snippet Generator */}
      <div className="space-y-3">
        <h2 className="text-base font-semibold text-white">Client Code Snippets</h2>
        <CodeGenerator apiKey={apiKeyInput || 'sk_live_ws_default_sampleKey123'} />
      </div>
    </div>
  );
};
