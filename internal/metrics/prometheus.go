package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/alexandrmotologa/keygate/internal/storage"
)

// Exporter collects and formats metrics according to Prometheus exposition standard.
type Exporter struct {
	db          *storage.DB
	startTime   time.Time
	reqTotal    int64
	cacheHits   int64
	cacheMisses int64
	rateLimited int64
}

// NewExporter initializes the Prometheus exporter.
func NewExporter(db *storage.DB) *Exporter {
	return &Exporter{
		db:        db,
		startTime: time.Now(),
	}
}

// IncRequests increments total request count.
func (e *Exporter) IncRequests() {
	atomic.AddInt64(&e.reqTotal, 1)
}

// IncCacheHit increments in-memory cache hit count.
func (e *Exporter) IncCacheHit() {
	atomic.AddInt64(&e.cacheHits, 1)
}

// IncCacheMiss increments SQLite lookup count.
func (e *Exporter) IncCacheMiss() {
	atomic.AddInt64(&e.cacheMisses, 1)
}

// IncRateLimited increments 429 response count.
func (e *Exporter) IncRateLimited() {
	atomic.AddInt64(&e.rateLimited, 1)
}

// ServeHTTP writes metrics in standard Prometheus text format.
func (e *Exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	summary, _ := e.db.GetAnalyticsSummary(ctx, "")
	uptime := time.Since(e.startTime).Seconds()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	fmt.Fprintf(w, "# HELP keygate_uptime_seconds Total runtime in seconds\n")
	fmt.Fprintf(w, "# TYPE keygate_uptime_seconds gauge\n")
	fmt.Fprintf(w, "keygate_uptime_seconds %.2f\n\n", uptime)

	fmt.Fprintf(w, "# HELP keygate_requests_total Total API requests processed\n")
	fmt.Fprintf(w, "# TYPE keygate_requests_total counter\n")
	if summary != nil {
		fmt.Fprintf(w, "keygate_requests_total{status=\"200\"} %d\n", summary.SuccessRequests)
		fmt.Fprintf(w, "keygate_requests_total{status=\"429\"} %d\n", summary.RateLimitedRequests)
		fmt.Fprintf(w, "keygate_requests_total{status=\"401\"} %d\n", summary.UnauthorizedRequests)
	} else {
		fmt.Fprintf(w, "keygate_requests_total %d\n", atomic.LoadInt64(&e.reqTotal))
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP keygate_active_keys Number of currently active API keys\n")
	fmt.Fprintf(w, "# TYPE keygate_active_keys gauge\n")
	if summary != nil {
		fmt.Fprintf(w, "keygate_active_keys %d\n", summary.ActiveKeys)
		fmt.Fprintf(w, "keygate_revoked_keys %d\n", summary.RevokedKeys)
		fmt.Fprintf(w, "keygate_total_keys %d\n", summary.TotalKeys)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP keygate_cache_events_total In-memory Ristretto cache events\n")
	fmt.Fprintf(w, "# TYPE keygate_cache_events_total counter\n")
	fmt.Fprintf(w, "keygate_cache_events_total{result=\"hit\"} %d\n", atomic.LoadInt64(&e.cacheHits))
	fmt.Fprintf(w, "keygate_cache_events_total{result=\"miss\"} %d\n", atomic.LoadInt64(&e.cacheMisses))
	fmt.Fprintf(w, "\n")

	if summary != nil {
		fmt.Fprintf(w, "# HELP keygate_verification_latency_ms Verification latency percentiles\n")
		fmt.Fprintf(w, "# TYPE keygate_verification_latency_ms gauge\n")
		fmt.Fprintf(w, "keygate_verification_latency_ms{quantile=\"0.5\"} %.4f\n", summary.P50LatencyMs)
		fmt.Fprintf(w, "keygate_verification_latency_ms{quantile=\"0.95\"} %.4f\n", summary.P95LatencyMs)
		fmt.Fprintf(w, "keygate_verification_latency_ms{quantile=\"0.99\"} %.4f\n", summary.P99LatencyMs)
		fmt.Fprintf(w, "keygate_verification_latency_ms{quantile=\"avg\"} %.4f\n", summary.AvgLatencyMs)
	}
}
