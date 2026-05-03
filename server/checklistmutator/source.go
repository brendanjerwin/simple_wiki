package checklistmutator

import "context"

// Source attributes a checklist mutation to its origin. Per ADR-0015,
// the per-checklist event log uses the source string to drive the
// engine's causal merge rule:
//
//   - user:<email>                    direct user edit (wiki UI / gRPC)
//   - connector:<kind>:apply          inbound apply from a connector
//   - connector:<kind>:push_recovery  412/conflict apply from a connector
//   - migration:<reason>              migration-job synthesized event
//   - system:<rule>                   wiki-internal rule (e.g., GC)
//
// The kind in connector sources matches the ConnectorKind enum
// (google_keep, google_tasks, icloud_reminders, …) so engine code
// can prefix-match `connector:<this>:` to identify its own writes.
type Source string

// UserSource attributes a mutation to a direct user edit.
// `email` is the principal's login name (typically a verified
// email address).
func UserSource(email string) Source {
	return Source("user:" + email)
}

// ConnectorSource attributes a mutation to a connector. `kind`
// matches the ConnectorKind enum (google_keep, google_tasks, …);
// `op` is the verb (apply, push_recovery, …).
func ConnectorSource(kind, op string) Source {
	return Source("connector:" + kind + ":" + op)
}

// MigrationSource attributes a mutation to a migration job.
// `reason` describes the specific migration (initial_baseline,
// fingerprint_to_log, …).
func MigrationSource(reason string) Source {
	return Source("migration:" + reason)
}

// SystemSource is the source for wiki-internal mutations that aren't
// user, connector, or migration driven (e.g., a future GC rule that
// drops orphaned items).
const SystemSource Source = "system:wiki"

// String returns the source as a plain string. Useful in log
// messages and in the persisted ChecklistEvent.Src field.
func (s Source) String() string { return string(s) }

// sourceCtxKey is the context-value key for an explicit Source override.
// Connector packages set it at the top of their inbound apply pass via
// WithSource so the mutator's *ForSync helpers can attribute their
// event-log entries with the right connector kind without changing
// the public mutator API.
type sourceCtxKey struct{}

// WithSource returns a derived context that carries source as the
// explicit attribution for any subsequent *ForSync call. Caller MUST
// thread the returned context through to the mutator call.
//
// Example (connector inbound apply pass):
//
//	ctx = checklistmutator.WithSource(ctx,
//	    checklistmutator.ConnectorSource("google_tasks", "apply"))
//	if err := m.UpdateItemForSync(ctx, ...); err != nil { ... }
func WithSource(ctx context.Context, source Source) context.Context {
	return context.WithValue(ctx, sourceCtxKey{}, source)
}

// sourceFromContext returns the explicit Source override (if any).
// The bool indicates presence: true means the caller set an override;
// false means the mutator should derive a default (typically
// UserSource(identity.LoginName())).
func sourceFromContext(ctx context.Context) (Source, bool) {
	if ctx == nil {
		return "", false
	}
	s, ok := ctx.Value(sourceCtxKey{}).(Source)
	return s, ok
}
