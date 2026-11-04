import React, { useState } from 'react';
import { createWorkspace } from '../services/api';
import { Workspace } from '../types';
import { X, Building2, Plus } from 'lucide-react';

interface WorkspaceModalProps {
  isOpen: boolean;
  onClose: () => void;
  onWorkspaceCreated: (ws: Workspace) => void;
}

export const WorkspaceModal: React.FC<WorkspaceModalProps> = ({
  isOpen,
  onClose,
  onWorkspaceCreated,
}) => {
  const [id, setId] = useState('');
  const [name, setName] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id.trim()) return;

    setIsSubmitting(true);
    setError(null);

    try {
      const cleanId = id.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '_');
      const ws = await createWorkspace(cleanId, name.trim() || cleanId);
      onWorkspaceCreated(ws);
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to create workspace');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-surface border border-border w-full max-w-md rounded-2xl shadow-2xl p-6 space-y-5">
        <div className="flex items-center justify-between border-b border-border pb-4">
          <div className="flex items-center gap-2.5">
            <div className="p-2 bg-primary-950/60 border border-primary-800/40 rounded-lg text-primary-400">
              <Building2 className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-bold text-white text-base">Create New Workspace</h3>
              <p className="text-xs text-slate-400 mt-0.5">Isolate API keys, quotas and audit trails</p>
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

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
              Workspace Identifier (Slug)
            </label>
            <input
              type="text"
              required
              placeholder="e.g. ws_production, ws_staging, client_acme"
              value={id}
              onChange={(e) => setId(e.target.value)}
              className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white font-mono placeholder-slate-600 focus:outline-none focus:border-primary-500"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
              Human-Readable Name
            </label>
            <input
              type="text"
              placeholder="e.g. Production Cluster Europe"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full px-3.5 py-2 bg-background border border-border rounded-lg text-sm text-white placeholder-slate-600 focus:outline-none focus:border-primary-500"
            />
          </div>

          <div className="pt-2 border-t border-border flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-xs font-medium text-slate-400 hover:text-white bg-surface-hover rounded-lg"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting || !id.trim()}
              className="px-5 py-2 text-xs font-semibold text-white bg-primary-600 hover:bg-primary-500 rounded-lg transition-colors flex items-center gap-1.5 disabled:opacity-50"
            >
              <Plus className="w-4 h-4" />
              {isSubmitting ? 'Creating...' : 'Create Workspace'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
