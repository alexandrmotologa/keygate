import { APIKey, CreatedKeyResult, AnalyticsSummary, VerifyKeyResult } from '../types';

const BASE_URL = '';

export async function fetchAnalyticsSummary(workspaceId = 'ws_default'): Promise<AnalyticsSummary> {
  const res = await fetch(`${BASE_URL}/v1/analytics.summary?workspaceId=${encodeURIComponent(workspaceId)}`);
  if (!res.ok) {
    throw new Error(`Failed to fetch analytics: ${res.statusText}`);
  }
  return res.json();
}

export async function fetchKeys(opts: {
  workspaceId?: string;
  status?: string;
  search?: string;
  limit?: number;
  offset?: number;
} = {}): Promise<{ keys: APIKey[]; total: number }> {
  const params = new URLSearchParams();
  if (opts.workspaceId) params.set('workspaceId', opts.workspaceId);
  if (opts.status) params.set('status', opts.status);
  if (opts.search) params.set('search', opts.search);
  if (opts.limit) params.set('limit', opts.limit.toString());
  if (opts.offset) params.set('offset', opts.offset.toString());

  const res = await fetch(`${BASE_URL}/v1/keys.listKeys?${params.toString()}`);
  if (!res.ok) {
    throw new Error(`Failed to list keys: ${res.statusText}`);
  }
  const data = await res.json();
  return {
    keys: data.keys || [],
    total: data.total || 0,
  };
}

export interface CreateKeyPayload {
  name: string;
  workspaceId?: string;
  prefix?: string;
  ownerId?: string;
  tier?: string;
  rateLimit?: number;
  rateDuration?: number;
  burst?: number;
  scopes?: string[];
  ipAllowlist?: string[];
  expiresAt?: string;
  metadata?: Record<string, string>;
}

export async function createKey(payload: CreateKeyPayload): Promise<CreatedKeyResult> {
  const res = await fetch(`${BASE_URL}/v1/keys.createKey`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(err.message || 'Failed to create key');
  }
  return res.json();
}

export async function revokeKey(keyId: string): Promise<{ success: boolean; keyId: string; revokedAt: string }> {
  const res = await fetch(`${BASE_URL}/v1/keys.revokeKey`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keyId }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(err.message || 'Failed to revoke key');
  }
  return res.json();
}

export async function verifyKey(payload: {
  key: string;
  workspaceId?: string;
  requiredScopes?: string[];
}): Promise<VerifyKeyResult> {
  const res = await fetch(`${BASE_URL}/v1/keys.verifyKey`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return res.json();
}
