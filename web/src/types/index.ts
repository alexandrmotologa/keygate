export interface APIKey {
  id: string;
  prefix: string;
  name: string;
  ownerId: string;
  workspaceId: string;
  tier: 'free' | 'pro' | 'enterprise' | 'custom' | string;
  rateLimit: number;
  rateDuration: number;
  burst: number;
  maxUses: number;
  scopes: string[];
  ipAllowlist: string[];
  metadata: Record<string, string>;
  expiresAt?: string;
  revokedAt?: string;
  enabled: boolean;
  totalRequests: number;
  lastUsedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Workspace {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
}

export interface AuditLog {
  id: number;
  workspaceId: string;
  actor: string;
  action: string;
  targetId: string;
  details: Record<string, string>;
  timestamp: string;
}

export interface CreatedKeyResult {
  keyId: string;
  key: string;
  prefix: string;
  name: string;
  workspaceId: string;
  ownerId: string;
  tier: string;
  rateLimit: number;
  rateDuration: number;
  burst: number;
  maxUses: number;
  scopes: string[];
  ipAllowlist: string[];
  expiresAt?: string;
  createdAt: string;
}

export interface RotatedKeyResult {
  success: boolean;
  oldKeyId: string;
  newKeyId: string;
  newKey: string;
  gracePeriodHours: number;
  oldKeyExpiresAt: string;
}

export interface HourlyBucketData {
  timestamp: string;
  totalRequests: number;
  successRequests: number;
  rateLimitedRequests: number;
  unauthorizedRequests: number;
  avgLatencyMs: number;
}

export interface AnalyticsSummary {
  totalKeys: number;
  activeKeys: number;
  revokedKeys: number;
  totalRequests: number;
  successRequests: number;
  rateLimitedRequests: number;
  unauthorizedRequests: number;
  avgLatencyMs: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
  p99LatencyMs: number;
  requestsByTier: Record<string, number>;
  hourlyBuckets: HourlyBucketData[];
}

export interface UsageEvent {
  id: number;
  keyId: string;
  workspaceId: string;
  statusCode: number;
  latencyMs: number;
  clientIp: string;
  path: string;
  method: string;
  timestamp: string;
}

export interface VerifyKeyResult {
  valid: boolean;
  code: string;
  message?: string;
  keyId?: string;
  ownerId?: string;
  workspaceId?: string;
  name?: string;
  tier?: string;
  maxUses?: number;
  scopes?: string[];
  rateLimit?: {
    tier: string;
    limit: number;
    remaining: number;
    reset: number;
    retryAfter?: number;
  };
  cached?: boolean;
  durationUs?: number;
}
