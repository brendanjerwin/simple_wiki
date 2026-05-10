package engine

import (
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// Dead-letter retry is the engine's response to per-item push failures
// the adapter has classified as ErrorClassRetryable. Per MATRIX.md
// row 7 (and Keep's existing implementation that becomes engine
// policy under strictest-behavior-wins), the engine tracks per-item:
//
//   - PushFailureCount: incremented on every Retryable failure;
//     reset to zero on the first Success.
//   - NextAttemptAt: wall-clock floor for the next retry; computed
//     via exponential backoff from PushFailureCount.
//
// Items with PushFailureCount >= deadLetterThreshold (10, matching
// Keep's existing default) are dead-lettered: the engine emits a
// distinct log event and SKIPS the outbound push for that item until
// the failure count is cleared by user intervention (typically a
// wiki-side edit that re-resets the item to a known state, or an
// admin RPC).
//
// The state lives in the binding's AdapterState under the well-known
// `push_failures` subtree the engine reads/writes directly. Per-uid
// records are map[string]any with keys "count" (int) and
// "next_attempt_at" (RFC3339 string). The adapter codec round-trips
// the subtree opaquely. Defensive decoding handles TOML's tendency to
// surface integers as int64/float64 after a save/load round-trip.
//
// Tasks (which previously had no dead-letter mechanism) gains this
// behavior on collapse — every adapter inherits it via the engine.

// deadLetterThreshold is the per-item consecutive-push-failure count
// at which the engine stops attempting to push the item. After 10
// failures the item is dead-lettered: skipped on subsequent ticks
// until the user clears it (via a wiki-side re-edit that resets the
// failure count, or an admin RPC). Inbound apply still operates on
// dead-lettered items; only the outbound push is gated. Mirrors the
// legacy Keep connector's existing default — the strictest-behavior-
// wins promotion makes this engine policy.
const deadLetterThreshold = 10

// pushFailureBackoffBase and pushFailureBackoffMax bound the
// exponential per-item retry schedule. After the n-th consecutive
// failure, the engine waits min(60s * 2^(n-1), 1h) before the next
// push attempt for that item. The shouldSkipPush gate consults
// NextAttemptAt; only failures populate it. Mirrors the legacy Keep
// connector's pushFailureBackoffBaseSeconds /
// pushFailureBackoffMaxSeconds.
const (
	pushFailureBackoffBase = 60 * time.Second
	pushFailureBackoffMax  = time.Hour
)

// adapterStatePushFailuresKey is the well-known AdapterState subtree
// key the engine reads/writes for per-uid dead-letter bookkeeping.
// Shape: map[string]map[string]any keyed by uid, with fields
// "count" (int-coercible) and "next_attempt_at" (RFC3339 string).
// Documented in ADR-0015's "AdapterState schema" section alongside
// item_id_map.
const adapterStatePushFailuresKey = "push_failures"

// pushFailureRecord is the in-memory shape of a per-uid failure
// bookkeeping entry. Encoded into AdapterState as map[string]any so
// the adapter codec round-trips it opaquely.
type pushFailureRecord struct {
	count         int
	nextAttemptAt time.Time
}

// recordPushFailure is invoked from reconcile.go's outbound push
// path when an adapter primitive returns an error classified as
// ErrorClassRetryable. It increments the per-uid failure count,
// computes the next-attempt timestamp via exponential backoff, and
// writes the updated record back into binding.AdapterState under
// the well-known push_failures subtree. The caller persists via
// SaveBinding at the end of the reconcile tick.
//
// Returns the updated binding (caller-owned mutation pattern: the
// engine never mutates AdapterState in place because Binding is
// passed by value through the engine's hot path).
//
// Items whose post-increment count meets or exceeds
// deadLetterThreshold emit a distinct `dead_letter_threshold_breached`
// log event so operators have a stable grep target. The bookkeeping
// is still updated so subsequent ticks see the dead-letter state.
func (e *Engine) recordPushFailure(
	binding connectors.Binding,
	uid string,
	op string,
	pushErr error,
) connectors.Binding {
	failures := readPushFailures(binding.AdapterState)
	prev := failures[uid]
	count := prev.count + 1
	nextAttempt := e.clock.Now().Add(pushFailureBackoff(count))
	failures[uid] = pushFailureRecord{count: count, nextAttemptAt: nextAttempt}

	updated := writePushFailures(binding, failures)

	if count >= deadLetterThreshold {
		e.logger.Warn("connectors/engine: dead_letter_threshold_breached kind=%s profile=%s page=%s list=%s uid=%s op=%s count=%d next_attempt_at=%s err=%v",
			e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName,
			uid, op, count, nextAttempt.Format(time.RFC3339), pushErr)
	} else {
		e.logger.Info("connectors/engine: push_failure_recorded kind=%s profile=%s page=%s list=%s uid=%s op=%s count=%d next_attempt_at=%s err=%v",
			e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName,
			uid, op, count, nextAttempt.Format(time.RFC3339), pushErr)
	}
	return updated
}

// recordPushSuccess clears the dead-letter bookkeeping for a uid
// after a successful outbound primitive (Insert/Patch/Delete). The
// failure count and next-attempt timestamp are removed; other uids'
// records and other AdapterState subtrees are preserved. Returns
// the updated binding.
//
// No-op when no failure record exists for uid (the steady-state for
// healthy items). The returned binding is still caller-owned so
// reconcile.go can chain the result into the next iteration.
//
// Receiver is unused: the success path needs no clock or logger. The
// method stays on Engine for symmetry with recordPushFailure /
// shouldSkipPush so reconcile.go's outbound loop reads cleanly.
func (*Engine) recordPushSuccess(
	binding connectors.Binding,
	uid string,
) connectors.Binding {
	failures := readPushFailures(binding.AdapterState)
	if _, hasRecord := failures[uid]; !hasRecord {
		return binding
	}
	delete(failures, uid)
	return writePushFailures(binding, failures)
}

// shouldSkipPush gates the outbound push for a uid based on its
// dead-letter state. Callers consult this BEFORE invoking
// Adapter.{Insert,Patch,Delete}Remote. Returns:
//
//   - (false, "") if no failure record exists, or the backoff
//     window has elapsed and count is below threshold.
//   - (true, "dead_letter") if count >= deadLetterThreshold.
//   - (true, "backoff") if count < threshold but next_attempt_at
//     is still in the future.
//
// The reason is a stable string for log grepping and metric
// labeling.
func (e *Engine) shouldSkipPush(
	binding connectors.Binding,
	uid string,
) (bool, string) {
	failures := readPushFailures(binding.AdapterState)
	rec, has := failures[uid]
	if !has {
		return false, ""
	}
	if rec.count >= deadLetterThreshold {
		return true, "dead_letter"
	}
	if !rec.nextAttemptAt.IsZero() && e.clock.Now().Before(rec.nextAttemptAt) {
		return true, "backoff"
	}
	return false, ""
}

// pushFailureBackoff returns the wait duration after the n-th
// consecutive per-item push failure: min(60s * 2^(n-1), 1h).
// n is the post-increment failure count (so n=1 returns 60s, n=2
// returns 120s, ..., capped at 1h). For n < 1 (defensive guard),
// returns zero — callers shouldn't invoke this with n=0, but the
// guard avoids a negative shift.
func pushFailureBackoff(n int) time.Duration {
	if n < 1 {
		return 0
	}
	d := pushFailureBackoffBase
	for i := 1; i < n; i++ {
		d *= 2
		if d >= pushFailureBackoffMax {
			return pushFailureBackoffMax
		}
	}
	if d > pushFailureBackoffMax {
		return pushFailureBackoffMax
	}
	return d
}

// readPushFailures pulls the well-known push_failures subtree out of
// an AdapterState. Returns a fresh, non-nil map even when the
// AdapterState has no entry, so the caller can mutate without
// nil-checks.
//
// Defensive decoding: TOML round-trips can surface "count" as int,
// int64, or float64 (depending on the encoder), and may surface the
// per-uid map as either map[string]any or a typed shape. We accept
// every reasonable variant; unknown shapes are silently dropped
// (matches Tasks's existing tolerant approach to legacy state).
func readPushFailures(state connectors.AdapterState) map[string]pushFailureRecord {
	out := map[string]pushFailureRecord{}
	if state == nil {
		return out
	}
	raw, ok := state[adapterStatePushFailuresKey]
	if !ok || raw == nil {
		return out
	}
	asMap, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	for uid, entryAny := range asMap {
		entry, ok := entryAny.(map[string]any)
		if !ok {
			continue
		}
		out[uid] = decodePushFailureRecord(entry)
	}
	return out
}

// decodePushFailureRecord pulls count/next_attempt_at out of a
// per-uid map[string]any, accepting the shapes TOML / JSON / Go
// literal initializers produce. Unknown fields are ignored.
func decodePushFailureRecord(entry map[string]any) pushFailureRecord {
	rec := pushFailureRecord{}
	if c, ok := entry["count"]; ok {
		rec.count = coerceInt(c)
	}
	if nv, ok := entry["next_attempt_at"]; ok {
		rec.nextAttemptAt = coerceTime(nv)
	}
	return rec
}

// coerceInt accepts int, int32, int64, float64 (TOML's number shape)
// and returns the integer value. Returns 0 for any other shape — the
// caller treats a zero count as "no record," which is the safe
// default when the persisted data is malformed.
func coerceInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// coerceTime accepts string (RFC3339) and time.Time values — TOML
// can decode either depending on whether the encoder wrote a quoted
// string or an offset-datetime literal. Returns the zero time for
// any other shape.
func coerceTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		if t == "" {
			return time.Time{}
		}
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return time.Time{}
		}
		return parsed
	default:
		return time.Time{}
	}
}

// writePushFailures writes the supplied uid → record map back into
// the binding's AdapterState under the push_failures subtree.
// Returns the updated binding (caller stores). Records are encoded
// as map[string]any so the adapter codec round-trips them
// transparently. An empty map is preserved (rather than removing
// the key) so RebuildAdapterState / EncodeAdapterState don't have
// to treat presence/absence as different states.
func writePushFailures(binding connectors.Binding, failures map[string]pushFailureRecord) connectors.Binding {
	if binding.AdapterState == nil {
		binding.AdapterState = connectors.AdapterState{}
	}
	encoded := make(map[string]any, len(failures))
	for uid, rec := range failures {
		entry := map[string]any{
			"count": rec.count,
		}
		if !rec.nextAttemptAt.IsZero() {
			entry["next_attempt_at"] = rec.nextAttemptAt.UTC().Format(time.RFC3339)
		} else {
			entry["next_attempt_at"] = ""
		}
		encoded[uid] = entry
	}
	binding.AdapterState[adapterStatePushFailuresKey] = encoded
	return binding
}
