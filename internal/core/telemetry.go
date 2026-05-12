// Distributed MemOS - Core: Cognitive Ranking and Lifecycle
package core

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Telemetry tracks service-level counters and coarse latency.
type Telemetry struct {
	storeCount        atomic.Int64
	retrieveCount     atomic.Int64
	auditWriteCount   atomic.Int64
	auditReadCount    atomic.Int64
	authDeniedCount   atomic.Int64
	cacheHitCount     atomic.Int64
	cacheMissCount    atomic.Int64
	replicationLagNanos atomic.Int64
	replicationLagMaxNanos atomic.Int64
	storeLatencyNanos atomic.Int64
	retrieveLatencyNanos atomic.Int64
}

func NewTelemetry() *Telemetry {
	return &Telemetry{}
}

func (t *Telemetry) RecordStore(duration time.Duration) {
	t.storeCount.Add(1)
	t.storeLatencyNanos.Add(duration.Nanoseconds())
}

func (t *Telemetry) RecordRetrieve(duration time.Duration) {
	t.retrieveCount.Add(1)
	t.retrieveLatencyNanos.Add(duration.Nanoseconds())
}

func (t *Telemetry) RecordAuditWrite() {
	t.auditWriteCount.Add(1)
}

func (t *Telemetry) RecordAuditRead() {
	t.auditReadCount.Add(1)
}

func (t *Telemetry) RecordAuthDenied() {
	t.authDeniedCount.Add(1)
}

func (t *Telemetry) RecordCacheHit() {
	t.cacheHitCount.Add(1)
}

func (t *Telemetry) RecordCacheMiss() {
	t.cacheMissCount.Add(1)
}

func (t *Telemetry) RecordReplicationLag(duration time.Duration) {
	nanos := duration.Nanoseconds()
	t.replicationLagNanos.Add(nanos)
	for {
		current := t.replicationLagMaxNanos.Load()
		if nanos <= current {
			return
		}
		if t.replicationLagMaxNanos.CompareAndSwap(current, nanos) {
			return
		}
	}
}

// Handler returns a minimal Prometheus-compatible metrics payload.
func (t *Telemetry) Handler(w http.ResponseWriter, r *http.Request) {
	storeCount := t.storeCount.Load()
	retrieveCount := t.retrieveCount.Load()
	auditCount := t.auditWriteCount.Load()
	auditReadCount := t.auditReadCount.Load()
	authDeniedCount := t.authDeniedCount.Load()
	cacheHitCount := t.cacheHitCount.Load()
	cacheMissCount := t.cacheMissCount.Load()
	replicationLagNanos := t.replicationLagNanos.Load()
	replicationLagMaxNanos := t.replicationLagMaxNanos.Load()
	storeLatency := t.storeLatencyNanos.Load()
	retrieveLatency := t.retrieveLatencyNanos.Load()

	avgStoreMs := float64(0)
	if storeCount > 0 {
		avgStoreMs = float64(storeLatency) / float64(storeCount) / float64(time.Millisecond)
	}

	avgRetrieveMs := float64(0)
	if retrieveCount > 0 {
		avgRetrieveMs = float64(retrieveLatency) / float64(retrieveCount) / float64(time.Millisecond)
	}

	avgReplicationLagMs := float64(0)
	if storeCount > 0 {
		avgReplicationLagMs = float64(replicationLagNanos) / float64(storeCount) / float64(time.Millisecond)
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "# HELP memos_store_requests_total Total Store requests\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_store_requests_total counter\n")
	_, _ = fmt.Fprintf(w, "memos_store_requests_total %d\n", storeCount)
	_, _ = fmt.Fprintf(w, "# HELP memos_retrieve_requests_total Total Retrieve requests\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_retrieve_requests_total counter\n")
	_, _ = fmt.Fprintf(w, "memos_retrieve_requests_total %d\n", retrieveCount)
	_, _ = fmt.Fprintf(w, "# HELP memos_audit_writes_total Total audit log writes\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_audit_writes_total counter\n")
	_, _ = fmt.Fprintf(w, "memos_audit_writes_total %d\n", auditCount)
	_, _ = fmt.Fprintf(w, "# HELP memos_audit_reads_total Total audit log reads\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_audit_reads_total counter\n")
	_, _ = fmt.Fprintf(w, "memos_audit_reads_total %d\n", auditReadCount)
	_, _ = fmt.Fprintf(w, "# HELP memos_auth_denied_total Total denied authorization checks\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_auth_denied_total counter\n")
	_, _ = fmt.Fprintf(w, "memos_auth_denied_total %d\n", authDeniedCount)
	_, _ = fmt.Fprintf(w, "# HELP memos_cache_hits_total Total cache hits\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_cache_hits_total counter\n")
	_, _ = fmt.Fprintf(w, "memos_cache_hits_total %d\n", cacheHitCount)
	_, _ = fmt.Fprintf(w, "# HELP memos_cache_misses_total Total cache misses\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_cache_misses_total counter\n")
	_, _ = fmt.Fprintf(w, "memos_cache_misses_total %d\n", cacheMissCount)
	_, _ = fmt.Fprintf(w, "# HELP memos_replication_lag_ms_avg Average replication lag in milliseconds\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_replication_lag_ms_avg gauge\n")
	_, _ = fmt.Fprintf(w, "memos_replication_lag_ms_avg %.3f\n", avgReplicationLagMs)
	_, _ = fmt.Fprintf(w, "# HELP memos_replication_lag_ms_max Maximum replication lag in milliseconds\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_replication_lag_ms_max gauge\n")
	_, _ = fmt.Fprintf(w, "memos_replication_lag_ms_max %.3f\n", float64(replicationLagMaxNanos)/float64(time.Millisecond))
	_, _ = fmt.Fprintf(w, "# HELP memos_store_latency_ms_avg Average Store latency in milliseconds\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_store_latency_ms_avg gauge\n")
	_, _ = fmt.Fprintf(w, "memos_store_latency_ms_avg %.3f\n", avgStoreMs)
	_, _ = fmt.Fprintf(w, "# HELP memos_retrieve_latency_ms_avg Average Retrieve latency in milliseconds\n")
	_, _ = fmt.Fprintf(w, "# TYPE memos_retrieve_latency_ms_avg gauge\n")
	_, _ = fmt.Fprintf(w, "memos_retrieve_latency_ms_avg %.3f\n", avgRetrieveMs)
}