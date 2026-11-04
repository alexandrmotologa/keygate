import React, { useState } from 'react';
import { CreatedKeyResult } from '../types';
import { Check, Copy, AlertCircle, KeyRound, Terminal } from 'lucide-react';

interface SecretKeyModalProps {
  keyResult: CreatedKeyResult | null;
  onClose: () => void;
}

export const SecretKeyModal: React.FC<SecretKeyModalProps> = ({ keyResult, onClose }) => {
  const [copiedKey, setCopiedKey] = useState(false);
  const [copiedCurl, setCopiedCurl] = useState(false);

  if (!keyResult) return null;

  const handleCopyKey = () => {
    navigator.clipboard.writeText(keyResult.key);
    setCopiedKey(true);
    setTimeout(() => setCopiedKey(false), 2000);
  };

  const curlExample = `curl -X POST http://localhost:8000/v1/keys.verifyKey \\
  -H "Content-Type: application/json" \\
  -d '{"key": "${keyResult.key}"}'`;

  const handleCopyCurl = () => {
    navigator.clipboard.writeText(curlExample);
    setCopiedCurl(true);
    setTimeout(() => setCopiedCurl(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-surface border border-border w-full max-w-xl rounded-2xl shadow-2xl overflow-hidden p-6 space-y-5">
        {/* Header */}
        <div className="flex items-center gap-3">
          <div className="p-2.5 bg-emerald-950/60 border border-emerald-800/40 rounded-xl text-emerald-400">
            <KeyRound className="w-6 h-6" />
          </div>
          <div>
            <h3 className="font-bold text-white text-lg">API Key Created Successfully</h3>
            <p className="text-xs text-slate-400 mt-0.5">Assigned ID: {keyResult.keyId}</p>
          </div>
        </div>

        {/* Warning Callout */}
        <div className="flex items-start gap-3 p-3.5 bg-amber-950/30 border border-amber-800/40 rounded-xl text-xs text-amber-200">
          <AlertCircle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
          <div>
            <span className="font-semibold block">Store this secret key securely</span>
            KeyGate stores only an irreversible salted SHA-256 hash in SQLite. For security reasons, this plaintext
            secret cannot be recovered or displayed again once this modal is closed.
          </div>
        </div>

        {/* Plaintext Key Display */}
        <div>
          <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
            Your Plaintext API Key
          </label>
          <div className="flex items-center justify-between p-3 bg-background border border-border rounded-xl font-mono text-sm text-emerald-400 break-all select-all">
            <span className="mr-2">{keyResult.key}</span>
            <button
              onClick={handleCopyKey}
              className="p-1.5 text-slate-400 hover:text-white bg-surface rounded-lg border border-border shrink-0 transition-colors"
              title="Copy Key"
            >
              {copiedKey ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
        </div>

        {/* Ready-to-use cURL Snippet */}
        <div>
          <div className="flex items-center justify-between mb-1.5">
            <label className="text-xs font-semibold uppercase tracking-wider text-slate-300 flex items-center gap-1.5">
              <Terminal className="w-3.5 h-3.5 text-slate-400" />
              Quick Test Command
            </label>
            <button
              onClick={handleCopyCurl}
              className="text-xs text-primary-400 hover:text-primary-300 transition-colors flex items-center gap-1"
            >
              {copiedCurl ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
              {copiedCurl ? 'Copied' : 'Copy cURL'}
            </button>
          </div>
          <pre className="p-3 bg-background border border-border rounded-xl font-mono text-xs text-slate-300 overflow-x-auto">
            {curlExample}
          </pre>
        </div>

        {/* Footer */}
        <div className="pt-2 flex justify-end">
          <button
            onClick={onClose}
            className="px-5 py-2 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-500 rounded-lg shadow-sm transition-colors"
          >
            I Have Saved My Secret Key
          </button>
        </div>
      </div>
    </div>
  );
};
