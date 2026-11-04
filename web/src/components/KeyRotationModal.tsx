import React, { useState } from 'react';
import { APIKey, RotatedKeyResult } from '../types';
import { rotateKey } from '../services/api';
import { X, RefreshCw, ShieldCheck, Copy, Check } from 'lucide-react';

interface KeyRotationModalProps {
  keyToRotate: APIKey | null;
  onClose: () => void;
  onRotated: () => void;
}

export const KeyRotationModal: React.FC<KeyRotationModalProps> = ({
  keyToRotate,
  onClose,
  onRotated,
}) => {
  const [graceHours, setGraceHours] = useState(24);
  const [isRotating, setIsRotating] = useState(false);
  const [result, setResult] = useState<RotatedKeyResult | null>(null);
  const [copiedKey, setCopiedKey] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!keyToRotate) return null;

  const handleRotate = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsRotating(true);
    setError(null);

    try {
      const res = await rotateKey(keyToRotate.id, graceHours);
      setResult(res);
      onRotated();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Key rotation failed');
      }
    } finally {
      setIsRotating(false);
    }
  };

  const handleCopy = () => {
    if (result) {
      navigator.clipboard.writeText(result.newKey);
      setCopiedKey(true);
      setTimeout(() => setCopiedKey(false), 2000);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-surface border border-border w-full max-w-lg rounded-2xl shadow-2xl overflow-hidden p-6 space-y-5">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border pb-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-amber-950/60 border border-amber-800/40 rounded-lg text-amber-400">
              <RefreshCw className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-bold text-white text-base">Zero-Downtime Key Rotation</h3>
              <p className="text-xs text-slate-400 mt-0.5">Rotating: {keyToRotate.name}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-surface-hover"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {error && (
          <div className="p-3 bg-rose-950/50 border border-rose-800/50 rounded-lg text-xs text-rose-300">
            {error}
          </div>
        )}

        {!result ? (
          <form onSubmit={handleRotate} className="space-y-4">
            <div className="p-3.5 bg-background border border-border rounded-xl text-xs text-slate-300 space-y-2">
              <span className="font-semibold text-white block">How Zero-Downtime Rotation Works</span>
              <p className="text-slate-400 leading-relaxed">
                KeyGate generates a new replacement key inheriting all permissions and quotas, while keeping the old
                key active throughout the grace window so client systems can deploy credentials without outages.
              </p>
            </div>

            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-2">
                Transition Grace Period (Overlap Window)
              </label>
              <div className="grid grid-cols-4 gap-2">
                {[
                  { hours: 1, label: '1 Hour' },
                  { hours: 6, label: '6 Hours' },
                  { hours: 24, label: '24 Hours' },
                  { hours: 168, label: '7 Days' },
                ].map((opt) => (
                  <button
                    type="button"
                    key={opt.hours}
                    onClick={() => setGraceHours(opt.hours)}
                    className={`py-2 px-3 text-xs font-medium rounded-lg border transition-all ${
                      graceHours === opt.hours
                        ? 'bg-surface-hover border-amber-500 text-white font-semibold'
                        : 'bg-background border-border text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>

            <div className="pt-3 border-t border-border flex justify-end gap-3">
              <button
                type="button"
                onClick={onClose}
                className="px-4 py-2 text-xs font-medium text-slate-400 hover:text-white bg-surface-hover rounded-lg"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={isRotating}
                className="px-5 py-2 text-xs font-semibold text-white bg-amber-600 hover:bg-amber-500 rounded-lg transition-colors flex items-center gap-1.5 shadow-sm"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                {isRotating ? 'Rotating Key...' : 'Issue Replacement Key'}
              </button>
            </div>
          </form>
        ) : (
          <div className="space-y-4">
            <div className="p-3.5 bg-emerald-950/40 border border-emerald-800/40 rounded-xl text-xs text-emerald-300 flex items-center gap-2.5">
              <ShieldCheck className="w-5 h-5 text-emerald-400 shrink-0" />
              <div>
                <span className="font-bold block">New Key Activated Successfully</span>
                Old key expires on: {new Date(result.oldKeyExpiresAt).toLocaleString()} ({result.gracePeriodHours}h grace period).
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
                New Plaintext Secret Key
              </label>
              <div className="flex items-center justify-between p-3 bg-background border border-border rounded-xl font-mono text-xs text-emerald-400 break-all select-all">
                <span className="mr-2">{result.newKey}</span>
                <button
                  onClick={handleCopy}
                  className="p-1.5 text-slate-400 hover:text-white bg-surface rounded-lg border border-border shrink-0"
                >
                  {copiedKey ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                </button>
              </div>
            </div>

            <div className="pt-2 flex justify-end">
              <button
                onClick={onClose}
                className="px-5 py-2 text-xs font-semibold text-white bg-primary-600 hover:bg-primary-500 rounded-lg"
              >
                Done
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
