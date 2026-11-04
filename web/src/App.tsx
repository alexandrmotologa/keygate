import React, { useState, useEffect, useCallback } from 'react';
import { APIKey, CreatedKeyResult, AnalyticsSummary, Workspace } from './types';
import { fetchAnalyticsSummary, fetchKeys, createKey, revokeKey, fetchWorkspaces, CreateKeyPayload } from './services/api';
import { useEventStream } from './hooks/useEventStream';
import { DashboardOverview } from './components/DashboardOverview';
import { KeyManagementTable } from './components/KeyManagementTable';
import { KeyDetailDrawer } from './components/KeyDetailDrawer';
import { KeyCreationModal } from './components/KeyCreationModal';
import { SecretKeyModal } from './components/SecretKeyModal';
import { KeyRotationModal } from './components/KeyRotationModal';
import { WorkspaceModal } from './components/WorkspaceModal';
import { AuditLogTable } from './components/AuditLogTable';
import { LiveRequestFeed } from './components/LiveRequestFeed';
import { ApiTester } from './components/ApiTester';
import { RateLimitPolicy } from './components/RateLimitPolicy';
import { Shield, KeyRound, Activity, Layers, Zap, RefreshCw, Building2, ShieldAlert, Plus } from 'lucide-react';

type Tab = 'overview' | 'keys' | 'telemetry' | 'audit' | 'ratelimits' | 'tester';

export const App: React.FC = () => {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [workspaces, setWorkspaces] = useState<Workspace[]>([
    { id: 'ws_default', name: 'Default Workspace', createdAt: '', updatedAt: '' },
  ]);
  const [selectedWorkspace, setSelectedWorkspace] = useState('ws_default');
  const [analytics, setAnalytics] = useState<AnalyticsSummary | null>(null);
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [isLoadingKeys, setIsLoadingKeys] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Modals & Drawers
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [createdResult, setCreatedResult] = useState<CreatedKeyResult | null>(null);
  const [selectedKeyId, setSelectedKeyId] = useState<string | null>(null);
  const [keyToRotate, setKeyToRotate] = useState<APIKey | null>(null);
  const [isWorkspaceModalOpen, setIsWorkspaceModalOpen] = useState(false);

  // Live SSE stream
  const { events: liveEvents, isConnected } = useEventStream(100);

  const loadWorkspaces = useCallback(async () => {
    try {
      const list = await fetchWorkspaces();
      if (list && list.length > 0) {
        setWorkspaces(list);
      }
    } catch {
      // Fallback
    }
  }, []);

  const loadData = useCallback(async () => {
    try {
      setIsRefreshing(true);
      const [summaryRes, keysRes] = await Promise.all([
        fetchAnalyticsSummary(selectedWorkspace).catch(() => null),
        fetchKeys({ workspaceId: selectedWorkspace, limit: 100 }).catch(() => ({ keys: [], total: 0 })),
      ]);

      if (summaryRes) setAnalytics(summaryRes);
      setKeys(keysRes.keys);
    } finally {
      setIsLoadingKeys(false);
      setIsRefreshing(false);
    }
  }, [selectedWorkspace]);

  useEffect(() => {
    loadWorkspaces();
  }, [loadWorkspaces]);

  useEffect(() => {
    loadData();
    const interval = setInterval(loadData, 10000);
    return () => clearInterval(interval);
  }, [loadData]);

  const handleCreateSubmit = async (payload: CreateKeyPayload) => {
    const res = await createKey({
      ...payload,
      workspaceId: selectedWorkspace,
    });
    setCreatedResult(res);
    await loadData();
  };

  const handleRevoke = async (keyId: string) => {
    await revokeKey(keyId);
    await loadData();
  };

  return (
    <div className="min-h-screen bg-background flex flex-col font-sans text-slate-100 antialiased">
      {/* Top Navigation Bar */}
      <header className="sticky top-0 z-40 bg-surface/90 backdrop-blur-md border-b border-border">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-4">
          {/* Logo & Workspace Selector */}
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-primary-600 to-indigo-500 flex items-center justify-center text-white shadow-md shadow-primary-900/30">
                <Shield className="w-5 h-5" />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-bold text-white text-base tracking-tight">KeyGate</span>
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold bg-primary-950/80 text-primary-400 border border-primary-800/40">
                    v1.1
                  </span>
                </div>
                <span className="text-[11px] text-slate-400 block -mt-0.5">Control Plane & Proxy</span>
              </div>
            </div>

            {/* Workspace Switcher */}
            <div className="flex items-center gap-1.5 pl-2 sm:pl-3 border-l border-border">
              <Building2 className="w-4 h-4 text-slate-500 hidden sm:inline" />
              <select
                value={selectedWorkspace}
                onChange={(e) => {
                  if (e.target.value === '__new__') {
                    setIsWorkspaceModalOpen(true);
                  } else {
                    setSelectedWorkspace(e.target.value);
                  }
                }}
                className="bg-background border border-border text-xs text-slate-200 rounded-lg px-2 py-1 sm:px-2.5 sm:py-1.5 font-medium max-w-[120px] sm:max-w-none truncate focus:outline-none focus:border-primary-500"
              >
                {workspaces.map((ws) => (
                  <option key={ws.id} value={ws.id}>
                    {ws.name}
                  </option>
                ))}
                <option value="__new__">+ New Workspace...</option>
              </select>
            </div>
          </div>

          {/* Navigation Tabs */}
          <nav className="hidden md:flex items-center gap-1 bg-background/80 p-1 rounded-xl border border-border">
            {[
              { id: 'overview', label: 'Overview', icon: Activity },
              { id: 'keys', label: 'API Keys', icon: KeyRound },
              { id: 'telemetry', label: 'Live Stream', icon: Activity },
              { id: 'audit', label: 'Audit Trail', icon: ShieldAlert },
              { id: 'ratelimits', label: 'Rate Limits', icon: Layers },
              { id: 'tester', label: 'Playground', icon: Zap },
            ].map((tab) => {
              const Icon = tab.icon;
              const isActive = activeTab === tab.id;
              return (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id as Tab)}
                  className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                    isActive
                      ? 'bg-surface text-white shadow-sm border border-border-light font-semibold'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-surface-hover/50'
                  }`}
                >
                  <Icon className={`w-3.5 h-3.5 ${isActive ? 'text-primary-400' : 'text-slate-400'}`} />
                  {tab.label}
                </button>
              );
            })}
          </nav>

          {/* Right Header Actions */}
          <div className="flex items-center gap-2.5">
            <button
              onClick={() => setIsWorkspaceModalOpen(true)}
              className="hidden sm:flex items-center gap-1.5 px-2.5 py-1.5 text-xs text-slate-300 bg-surface-hover hover:bg-slate-800 border border-border rounded-lg transition-colors"
              title="Add Workspace"
            >
              <Plus className="w-3.5 h-3.5" />
              <span className="hidden xl:inline">Workspace</span>
            </button>
            <button
              onClick={loadData}
              disabled={isRefreshing}
              className="p-2 text-slate-400 hover:text-white bg-surface-hover hover:bg-slate-800 border border-border rounded-lg transition-colors disabled:opacity-50"
              title="Refresh Data"
            >
              <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin text-primary-400' : ''}`} />
            </button>
            <button
              onClick={() => setIsCreateOpen(true)}
              className="px-3.5 py-1.5 text-xs font-semibold text-white bg-primary-600 hover:bg-primary-500 rounded-lg shadow-sm shadow-primary-900/40 transition-colors flex items-center gap-1.5"
            >
              <KeyRound className="w-3.5 h-3.5" />
              New Key
            </button>
          </div>
        </div>

        {/* Mobile Tab Scroller */}
        <div className="md:hidden flex overflow-x-auto border-t border-border px-4 py-2 gap-2 bg-background">
          {[
            { id: 'overview', label: 'Overview' },
            { id: 'keys', label: 'API Keys' },
            { id: 'telemetry', label: 'Live Stream' },
            { id: 'audit', label: 'Audit Trail' },
            { id: 'ratelimits', label: 'Rate Limits' },
            { id: 'tester', label: 'Playground' },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as Tab)}
              className={`px-3 py-1 text-xs whitespace-nowrap rounded-md ${
                activeTab === tab.id
                  ? 'bg-primary-600 text-white font-medium'
                  : 'bg-surface text-slate-400'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </header>

      {/* Main Content Area */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 flex-1 w-full">
        {activeTab === 'overview' && (
          <DashboardOverview
            analytics={analytics}
            liveEvents={liveEvents}
            isConnected={isConnected}
            onOpenCreateModal={() => setIsCreateOpen(true)}
            onNavigateToTester={() => setActiveTab('tester')}
          />
        )}

        {activeTab === 'keys' && (
          <KeyManagementTable
            keys={keys}
            isLoading={isLoadingKeys}
            workspaceId={selectedWorkspace}
            onRevokeKey={handleRevoke}
            onRotateKey={(k) => setKeyToRotate(k)}
            onSelectKey={(id) => setSelectedKeyId(id)}
            onOpenCreateModal={() => setIsCreateOpen(true)}
            onRefresh={loadData}
          />
        )}

        {activeTab === 'telemetry' && (
          <LiveRequestFeed
            events={liveEvents}
            isConnected={isConnected}
            onClear={() => {}}
          />
        )}

        {activeTab === 'audit' && (
          <AuditLogTable workspaceId={selectedWorkspace} />
        )}

        {activeTab === 'ratelimits' && <RateLimitPolicy />}

        {activeTab === 'tester' && <ApiTester />}
      </main>

      {/* Footer */}
      <footer className="border-t border-border bg-surface/50 py-4 text-center text-xs text-slate-500">
        KeyGate — High-Throughput API Key Gateway & Multi-Tier Rate Limiting Engine. Open-source self-hosted Unkey alternative.
      </footer>

      {/* Slide-Over Key Inspector Drawer */}
      <KeyDetailDrawer
        keyId={selectedKeyId}
        onClose={() => setSelectedKeyId(null)}
        onKeyUpdated={loadData}
        onRotateKey={(k) => {
          setSelectedKeyId(null);
          setKeyToRotate(k);
        }}
        onRevokeKey={(id) => {
          setSelectedKeyId(null);
          handleRevoke(id);
        }}
      />

      {/* Key Creation Modal */}
      <KeyCreationModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onSubmit={handleCreateSubmit}
      />

      {/* Secret Plaintext Reveal Modal */}
      <SecretKeyModal
        keyResult={createdResult}
        onClose={() => setCreatedResult(null)}
      />

      {/* Zero-Downtime Rotation Modal */}
      <KeyRotationModal
        keyToRotate={keyToRotate}
        onClose={() => setKeyToRotate(null)}
        onRotated={loadData}
      />

      {/* Create Workspace Modal */}
      <WorkspaceModal
        isOpen={isWorkspaceModalOpen}
        onClose={() => setIsWorkspaceModalOpen(false)}
        onWorkspaceCreated={(ws) => {
          setWorkspaces((prev) => [...prev, ws]);
          setSelectedWorkspace(ws.id);
        }}
      />
    </div>
  );
};
