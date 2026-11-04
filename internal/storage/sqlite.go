package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQL database handle and manages concurrency.
type DB struct {
	*sql.DB
	path string
	mu   sync.RWMutex
}

// Open creates or connects to a SQLite database in WAL mode and runs migrations.
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	db := &DB{
		DB:   sqlDB,
		path: path,
	}

	if err := db.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to execute migrations: %w", err)
	}

	return db, nil
}

// migrate creates required tables and indexes if they do not exist.
func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			salt TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			key_hash TEXT UNIQUE NOT NULL,
			prefix TEXT NOT NULL,
			name TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			tier TEXT NOT NULL DEFAULT 'free',
			rate_limit INTEGER NOT NULL DEFAULT 60,
			rate_duration INTEGER NOT NULL DEFAULT 60,
			burst INTEGER NOT NULL DEFAULT 10,
			max_uses INTEGER NOT NULL DEFAULT 0,
			scopes_json TEXT NOT NULL DEFAULT '[]',
			ip_allowlist_json TEXT NOT NULL DEFAULT '[]',
			metadata_json TEXT NOT NULL DEFAULT '{}',
			expires_at DATETIME,
			revoked_at DATETIME,
			enabled INTEGER NOT NULL DEFAULT 1,
			total_requests INTEGER NOT NULL DEFAULT 0,
			last_used_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_workspace ON api_keys(workspace_id);`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_owner ON api_keys(owner_id);`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_created_at ON api_keys(created_at);`,

		`CREATE TABLE IF NOT EXISTS usage_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_id TEXT,
			workspace_id TEXT NOT NULL,
			status_code INTEGER NOT NULL,
			latency_ms REAL NOT NULL,
			client_ip TEXT,
			path TEXT,
			method TEXT,
			timestamp DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_ws_time ON usage_events(workspace_id, timestamp);`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_key_time ON usage_events(key_id, timestamp);`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_time ON usage_events(timestamp);`,

		`CREATE TABLE IF NOT EXISTS hourly_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workspace_id TEXT NOT NULL,
			hour_bucket DATETIME NOT NULL,
			total_requests INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			error_count INTEGER NOT NULL DEFAULT 0,
			rate_limited_count INTEGER NOT NULL DEFAULT 0,
			avg_latency_ms REAL NOT NULL DEFAULT 0,
			UNIQUE(workspace_id, hour_bucket)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_hourly_stats_ws_hour ON hourly_stats(workspace_id, hour_bucket);`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workspace_id TEXT NOT NULL,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			target_id TEXT NOT NULL,
			details_json TEXT NOT NULL DEFAULT '{}',
			timestamp DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_ws_time ON audit_logs(workspace_id, timestamp);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("migration query error: %w (query: %s)", err, query)
		}
	}

	// Idempotent column check for existing databases
	_, _ = db.Exec("ALTER TABLE api_keys ADD COLUMN max_uses INTEGER NOT NULL DEFAULT 0;")

	// Seed default workspace if empty
	return db.seedDefaultWorkspace()
}

func (db *DB) seedDefaultWorkspace() error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM workspaces WHERE id = 'ws_default'").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		saltBytes := make([]byte, 16)
		if _, err := rand.Read(saltBytes); err != nil {
			return err
		}
		salt := hex.EncodeToString(saltBytes)
		now := time.Now().UTC()
		_, err = db.Exec(
			"INSERT INTO workspaces (id, name, salt, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			"ws_default", "Default Workspace", salt, now, now,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateWorkspace provisions a new isolated workspace with a cryptographic salt.
func (db *DB) CreateWorkspace(ctx context.Context, id, name string) (*Workspace, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if name == "" {
		name = id
	}

	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return nil, err
	}
	salt := hex.EncodeToString(saltBytes)
	now := time.Now().UTC()

	query := "INSERT INTO workspaces (id, name, salt, created_at, updated_at) VALUES (?, ?, ?, ?, ?)"
	_, err := db.ExecContext(ctx, query, id, name, salt, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert workspace: %w", err)
	}

	return &Workspace{
		ID:        id,
		Name:      name,
		Salt:      salt,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ListWorkspaces retrieves all registered workspaces.
func (db *DB) ListWorkspaces(ctx context.Context) ([]*Workspace, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, name, created_at, updated_at FROM workspaces ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []*Workspace
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, &w)
	}
	return workspaces, nil
}

// GetWorkspaceSalt retrieves the salt for a given workspace.
func (db *DB) GetWorkspaceSalt(ctx context.Context, workspaceID string) (string, error) {
	var salt string
	err := db.QueryRowContext(ctx, "SELECT salt FROM workspaces WHERE id = ?", workspaceID).Scan(&salt)
	if err == sql.ErrNoRows {
		// Auto-create workspace if it doesn't exist
		saltBytes := make([]byte, 16)
		if _, randErr := rand.Read(saltBytes); randErr != nil {
			return "", randErr
		}
		salt = hex.EncodeToString(saltBytes)
		now := time.Now().UTC()
		_, insertErr := db.ExecContext(ctx,
			"INSERT INTO workspaces (id, name, salt, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			workspaceID, workspaceID, salt, now, now,
		)
		if insertErr != nil {
			return "", insertErr
		}
		return salt, nil
	}
	if err != nil {
		return "", err
	}
	return salt, nil
}

// CreateKey inserts a new API key record into SQLite.
func (db *DB) CreateKey(ctx context.Context, key *APIKey) error {
	scopesJSON, err := json.Marshal(key.Scopes)
	if err != nil {
		scopesJSON = []byte("[]")
	}
	ipJSON, err := json.Marshal(key.IPAllowlist)
	if err != nil {
		ipJSON = []byte("[]")
	}
	metaJSON, err := json.Marshal(key.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	enabledInt := 0
	if key.Enabled {
		enabledInt = 1
	}

	query := `INSERT INTO api_keys (
		id, key_hash, prefix, name, owner_id, workspace_id, tier,
		rate_limit, rate_duration, burst, max_uses, scopes_json, ip_allowlist_json,
		metadata_json, expires_at, revoked_at, enabled, total_requests,
		last_used_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = db.ExecContext(ctx, query,
		key.ID, key.KeyHash, key.Prefix, key.Name, key.OwnerID, key.WorkspaceID, key.Tier,
		key.RateLimit, key.RateDuration, key.Burst, key.MaxUses, string(scopesJSON), string(ipJSON),
		string(metaJSON), key.ExpiresAt, key.RevokedAt, enabledInt, key.TotalRequests,
		key.LastUsedAt, key.CreatedAt, key.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert key: %w", err)
	}
	return nil
}

// GetKeyByHash looks up an API key by its SHA-256 hash string.
func (db *DB) GetKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	query := `SELECT
		id, key_hash, prefix, name, owner_id, workspace_id, tier,
		rate_limit, rate_duration, burst, max_uses, scopes_json, ip_allowlist_json,
		metadata_json, expires_at, revoked_at, enabled, total_requests,
		last_used_at, created_at, updated_at
	FROM api_keys WHERE key_hash = ?`

	row := db.QueryRowContext(ctx, query, keyHash)
	return db.scanKey(row)
}

// GetKeyByID looks up an API key by its internal ID.
func (db *DB) GetKeyByID(ctx context.Context, keyID string) (*APIKey, error) {
	query := `SELECT
		id, key_hash, prefix, name, owner_id, workspace_id, tier,
		rate_limit, rate_duration, burst, max_uses, scopes_json, ip_allowlist_json,
		metadata_json, expires_at, revoked_at, enabled, total_requests,
		last_used_at, created_at, updated_at
	FROM api_keys WHERE id = ?`

	row := db.QueryRowContext(ctx, query, keyID)
	return db.scanKey(row)
}

// ListKeysOptions specifies filters for listing API keys.
type ListKeysOptions struct {
	WorkspaceID string
	Status      string // "active", "revoked", "expired", "all"
	Search      string
	Limit       int
	Offset      int
}

// ListKeys returns filtered and paginated API keys.
func (db *DB) ListKeys(ctx context.Context, opts ListKeysOptions) ([]*APIKey, int, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}

	whereClauses := []string{"1=1"}
	args := []interface{}{}

	if opts.WorkspaceID != "" {
		whereClauses = append(whereClauses, "workspace_id = ?")
		args = append(args, opts.WorkspaceID)
	}

	now := time.Now().UTC()
	switch opts.Status {
	case "active":
		whereClauses = append(whereClauses, "enabled = 1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?) AND (max_uses = 0 OR total_requests < max_uses)")
		args = append(args, now)
	case "revoked":
		whereClauses = append(whereClauses, "revoked_at IS NOT NULL")
	case "expired":
		whereClauses = append(whereClauses, "(expires_at IS NOT NULL AND expires_at <= ?) OR (max_uses > 0 AND total_requests >= max_uses)")
		args = append(args, now)
	}

	if opts.Search != "" {
		searchTerm := "%" + strings.ToLower(opts.Search) + "%"
		whereClauses = append(whereClauses, "(LOWER(name) LIKE ? OR LOWER(prefix) LIKE ? OR LOWER(owner_id) LIKE ? OR id LIKE ?)")
		args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM api_keys WHERE %s", whereSQL)
	var total int
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count keys: %w", err)
	}

	query := fmt.Sprintf(`SELECT
		id, key_hash, prefix, name, owner_id, workspace_id, tier,
		rate_limit, rate_duration, burst, max_uses, scopes_json, ip_allowlist_json,
		metadata_json, expires_at, revoked_at, enabled, total_requests,
		last_used_at, created_at, updated_at
	FROM api_keys WHERE %s ORDER BY created_at DESC LIMIT ? OFFSET ?`, whereSQL)

	queryArgs := append(args, opts.Limit, opts.Offset)
	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		k, err := db.scanKeyFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		keys = append(keys, k)
	}

	return keys, total, nil
}

// RevokeKey marks an API key as revoked.
func (db *DB) RevokeKey(ctx context.Context, keyID string) (*time.Time, error) {
	now := time.Now().UTC()
	query := `UPDATE api_keys SET revoked_at = ?, enabled = 0, updated_at = ? WHERE id = ? AND revoked_at IS NULL`
	res, err := db.ExecContext(ctx, query, now, now, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke key: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, sql.ErrNoRows
	}
	return &now, nil
}

// UpdateKeyOptions specifies mutable key properties.
type UpdateKeyOptions struct {
	Name         *string
	Tier         *string
	RateLimit    *int
	RateDuration *int
	Burst        *int
	MaxUses      *int
	Scopes       *[]string
	IPAllowlist  *[]string
	ExpiresAt    *time.Time
	Enabled      *bool
}

// UpdateKey updates key configuration.
func (db *DB) UpdateKey(ctx context.Context, keyID string, opts UpdateKeyOptions) error {
	setClauses := []string{"updated_at = ?"}
	now := time.Now().UTC()
	args := []interface{}{now}

	if opts.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *opts.Name)
	}
	if opts.Tier != nil {
		setClauses = append(setClauses, "tier = ?")
		args = append(args, *opts.Tier)
	}
	if opts.RateLimit != nil {
		setClauses = append(setClauses, "rate_limit = ?")
		args = append(args, *opts.RateLimit)
	}
	if opts.RateDuration != nil {
		setClauses = append(setClauses, "rate_duration = ?")
		args = append(args, *opts.RateDuration)
	}
	if opts.Burst != nil {
		setClauses = append(setClauses, "burst = ?")
		args = append(args, *opts.Burst)
	}
	if opts.MaxUses != nil {
		setClauses = append(setClauses, "max_uses = ?")
		args = append(args, *opts.MaxUses)
	}
	if opts.Scopes != nil {
		scopesJSON, _ := json.Marshal(*opts.Scopes)
		setClauses = append(setClauses, "scopes_json = ?")
		args = append(args, string(scopesJSON))
	}
	if opts.IPAllowlist != nil {
		ipJSON, _ := json.Marshal(*opts.IPAllowlist)
		setClauses = append(setClauses, "ip_allowlist_json = ?")
		args = append(args, string(ipJSON))
	}
	if opts.ExpiresAt != nil {
		setClauses = append(setClauses, "expires_at = ?")
		args = append(args, *opts.ExpiresAt)
	}
	if opts.Enabled != nil {
		enabledInt := 0
		if *opts.Enabled {
			enabledInt = 1
		}
		setClauses = append(setClauses, "enabled = ?")
		args = append(args, enabledInt)
	}

	args = append(args, keyID)
	query := fmt.Sprintf("UPDATE api_keys SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// IncrementKeyUsage records a successful or rejected request for an API key.
func (db *DB) IncrementKeyUsage(ctx context.Context, keyID string, lastUsed time.Time) error {
	query := `UPDATE api_keys SET total_requests = total_requests + 1, last_used_at = ? WHERE id = ?`
	_, err := db.ExecContext(ctx, query, lastUsed, keyID)
	return err
}

// RecordUsageEvent records an individual request telemetry event.
func (db *DB) RecordUsageEvent(ctx context.Context, event *UsageEvent) error {
	query := `INSERT INTO usage_events (
		key_id, workspace_id, status_code, latency_ms, client_ip, path, method, timestamp
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query,
		event.KeyID, event.WorkspaceID, event.StatusCode, event.LatencyMs,
		event.ClientIP, event.Path, event.Method, event.Timestamp,
	)
	return err
}

// GetKeyUsageHistory returns recent usage events specifically for a single key.
func (db *DB) GetKeyUsageHistory(ctx context.Context, keyID string, limit int) ([]*UsageEvent, error) {
	if limit <= 0 {
		limit = 30
	}
	query := `SELECT id, key_id, workspace_id, status_code, latency_ms, client_ip, path, method, timestamp
		FROM usage_events WHERE key_id = ? ORDER BY timestamp DESC LIMIT ?`
	rows, err := db.QueryContext(ctx, query, keyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*UsageEvent
	for rows.Next() {
		var ev UsageEvent
		if err := rows.Scan(&ev.ID, &ev.KeyID, &ev.WorkspaceID, &ev.StatusCode, &ev.LatencyMs, &ev.ClientIP, &ev.Path, &ev.Method, &ev.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, &ev)
	}
	return events, nil
}

// RecordAuditLog inserts an administrative action entry.
func (db *DB) RecordAuditLog(ctx context.Context, log *AuditLog) error {
	detailsJSON, err := json.Marshal(log.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	query := `INSERT INTO audit_logs (workspace_id, actor, action, target_id, details_json, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err = db.ExecContext(ctx, query, log.WorkspaceID, log.Actor, log.Action, log.TargetID, string(detailsJSON), log.Timestamp)
	return err
}

// ListAuditLogs returns filtered security audit trails.
func (db *DB) ListAuditLogs(ctx context.Context, workspaceID string, limit, offset int) ([]*AuditLog, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := "1=1"
	args := []interface{}{}
	if workspaceID != "" {
		whereClause = "workspace_id = ?"
		args = append(args, workspaceID)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs WHERE %s", whereClause)
	var total int
	_ = db.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	query := fmt.Sprintf(`SELECT id, workspace_id, actor, action, target_id, details_json, timestamp
		FROM audit_logs WHERE %s ORDER BY timestamp DESC LIMIT ? OFFSET ?`, whereClause)
	queryArgs := append(args, limit, offset)

	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		var l AuditLog
		var detJSON string
		if err := rows.Scan(&l.ID, &l.WorkspaceID, &l.Actor, &l.Action, &l.TargetID, &detJSON, &l.Timestamp); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal([]byte(detJSON), &l.Details)
		if l.Details == nil {
			l.Details = make(map[string]string)
		}
		logs = append(logs, &l)
	}
	return logs, total, nil
}

// GetAnalyticsSummary calculates high-level telemetry and percentiles.
func (db *DB) GetAnalyticsSummary(ctx context.Context, workspaceID string) (*AnalyticsSummary, error) {
	summary := &AnalyticsSummary{
		RequestsByTier: make(map[string]int64),
		HourlyBuckets:  make([]HourlyBucketData, 0),
	}

	keyQuery := `SELECT
		COUNT(*) as total,
		SUM(CASE WHEN enabled = 1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > datetime('now')) AND (max_uses = 0 OR total_requests < max_uses) THEN 1 ELSE 0 END) as active,
		SUM(CASE WHEN revoked_at IS NOT NULL THEN 1 ELSE 0 END) as revoked
	FROM api_keys`
	keyArgs := []interface{}{}
	if workspaceID != "" {
		keyQuery += " WHERE workspace_id = ?"
		keyArgs = append(keyArgs, workspaceID)
	}

	var totalKeys, activeKeys, revokedKeys sql.NullInt64
	err := db.QueryRowContext(ctx, keyQuery, keyArgs...).Scan(&totalKeys, &activeKeys, &revokedKeys)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to count keys: %w", err)
	}
	summary.TotalKeys = totalKeys.Int64
	summary.ActiveKeys = activeKeys.Int64
	summary.RevokedKeys = revokedKeys.Int64

	usageQuery := `SELECT
		COUNT(*) as total,
		SUM(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END) as success,
		SUM(CASE WHEN status_code = 429 THEN 1 ELSE 0 END) as rate_limited,
		SUM(CASE WHEN status_code = 401 OR status_code = 403 THEN 1 ELSE 0 END) as unauthorized,
		COALESCE(AVG(latency_ms), 0) as avg_latency
	FROM usage_events`
	usageArgs := []interface{}{}
	if workspaceID != "" {
		usageQuery += " WHERE workspace_id = ?"
		usageArgs = append(usageArgs, workspaceID)
	}

	var totalReqs, successReqs, rateLimitedReqs, unauthorizedReqs sql.NullInt64
	var avgLatency sql.NullFloat64
	err = db.QueryRowContext(ctx, usageQuery, usageArgs...).Scan(
		&totalReqs, &successReqs, &rateLimitedReqs, &unauthorizedReqs, &avgLatency,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to aggregate usage events: %w", err)
	}

	summary.TotalRequests = totalReqs.Int64
	summary.SuccessRequests = successReqs.Int64
	summary.RateLimitedRequests = rateLimitedReqs.Int64
	summary.UnauthorizedRequests = unauthorizedReqs.Int64
	summary.AvgLatencyMs = avgLatency.Float64

	latQuery := `SELECT latency_ms FROM usage_events`
	latArgs := []interface{}{}
	if workspaceID != "" {
		latQuery += " WHERE workspace_id = ?"
		latArgs = append(latArgs, workspaceID)
	}
	latQuery += " ORDER BY latency_ms ASC"

	latRows, err := db.QueryContext(ctx, latQuery, latArgs...)
	if err == nil {
		defer latRows.Close()
		var latencies []float64
		for latRows.Next() {
			var l float64
			if err := latRows.Scan(&l); err == nil {
				latencies = append(latencies, l)
			}
		}
		if len(latencies) > 0 {
			summary.P50LatencyMs = percentile(latencies, 50)
			summary.P95LatencyMs = percentile(latencies, 95)
			summary.P99LatencyMs = percentile(latencies, 99)
		}
	}

	tierQuery := `SELECT k.tier, COUNT(u.id)
		FROM usage_events u
		JOIN api_keys k ON u.key_id = k.id`
	tierArgs := []interface{}{}
	if workspaceID != "" {
		tierQuery += " WHERE u.workspace_id = ?"
		tierArgs = append(tierArgs, workspaceID)
	}
	tierQuery += " GROUP BY k.tier"

	tierRows, err := db.QueryContext(ctx, tierQuery, tierArgs...)
	if err == nil {
		defer tierRows.Close()
		for tierRows.Next() {
			var tier string
			var count int64
			if err := tierRows.Scan(&tier, &count); err == nil {
				summary.RequestsByTier[tier] = count
			}
		}
	}

	bucketQuery := `SELECT
		strftime('%Y-%m-%dT%H:00:00Z', timestamp) as hour_bucket,
		COUNT(*) as total,
		SUM(CASE WHEN status_code >= 200 AND status_code < 400 THEN 1 ELSE 0 END) as success,
		SUM(CASE WHEN status_code = 429 THEN 1 ELSE 0 END) as rate_limited,
		SUM(CASE WHEN status_code = 401 OR status_code = 403 THEN 1 ELSE 0 END) as unauthorized,
		COALESCE(AVG(latency_ms), 0) as avg_latency
	FROM usage_events
	WHERE timestamp >= datetime('now', '-24 hours')`
	bucketArgs := []interface{}{}
	if workspaceID != "" {
		bucketQuery += " AND workspace_id = ?"
		bucketArgs = append(bucketArgs, workspaceID)
	}
	bucketQuery += " GROUP BY hour_bucket ORDER BY hour_bucket ASC"

	bRows, err := db.QueryContext(ctx, bucketQuery, bucketArgs...)
	if err == nil {
		defer bRows.Close()
		for bRows.Next() {
			var b HourlyBucketData
			if err := bRows.Scan(&b.Timestamp, &b.TotalRequests, &b.SuccessRequests, &b.RateLimitedRequests, &b.UnauthorizedRequests, &b.AvgLatencyMs); err == nil {
				summary.HourlyBuckets = append(summary.HourlyBuckets, b)
			}
		}
	}

	return summary, nil
}

func percentile(sortedData []float64, pct float64) float64 {
	if len(sortedData) == 0 {
		return 0
	}
	if len(sortedData) == 1 {
		return sortedData[0]
	}
	if pct <= 0 {
		return sortedData[0]
	}
	if pct >= 100 {
		return sortedData[len(sortedData)-1]
	}

	rank := (pct / 100.0) * float64(len(sortedData)-1)
	lowIndex := int(rank)
	highIndex := lowIndex + 1
	if highIndex >= len(sortedData) {
		return sortedData[len(sortedData)-1]
	}
	weight := rank - float64(lowIndex)
	return sortedData[lowIndex]*(1.0-weight) + sortedData[highIndex]*weight
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func (db *DB) scanKey(row scannable) (*APIKey, error) {
	var k APIKey
	var scopesJSON, ipJSON, metaJSON string
	var enabledInt int

	err := row.Scan(
		&k.ID, &k.KeyHash, &k.Prefix, &k.Name, &k.OwnerID, &k.WorkspaceID, &k.Tier,
		&k.RateLimit, &k.RateDuration, &k.Burst, &k.MaxUses, &scopesJSON, &ipJSON,
		&metaJSON, &k.ExpiresAt, &k.RevokedAt, &enabledInt, &k.TotalRequests,
		&k.LastUsedAt, &k.CreatedAt, &k.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	k.Enabled = enabledInt == 1
	_ = json.Unmarshal([]byte(scopesJSON), &k.Scopes)
	_ = json.Unmarshal([]byte(ipJSON), &k.IPAllowlist)
	_ = json.Unmarshal([]byte(metaJSON), &k.Metadata)

	if k.Scopes == nil {
		k.Scopes = []string{}
	}
	if k.IPAllowlist == nil {
		k.IPAllowlist = []string{}
	}
	if k.Metadata == nil {
		k.Metadata = make(map[string]string)
	}

	return &k, nil
}

func (db *DB) scanKeyFromRows(rows *sql.Rows) (*APIKey, error) {
	return db.scanKey(rows)
}
