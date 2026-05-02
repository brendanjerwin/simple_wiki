package connectors

// Event names used by structured logs and metrics. There is NO event
// bus here — these are just stable string constants so log queries and
// dashboards can pivot on the exact label.
//
// Adding a new event is deliberate: dashboards and alerts may key on
// the constant value, so renaming an existing one is a dashboard-
// visible change.
const (
	// EventSubscriptionEstablished fires when a subscription is
	// successfully created (Subscribe call completed end-to-end:
	// profile written, lease taken).
	EventSubscriptionEstablished = "subscription_established"

	// EventSubscriptionPaused fires when a subscription transitions
	// to the paused state (typically auth_failed → invalid_grant
	// after retry-once, or operator action).
	EventSubscriptionPaused = "subscription_paused"

	// EventSubscriptionResumed fires when a paused subscription
	// resumes (reconnect completes, OAuth refresh succeeds, or
	// operator clears the paused flag).
	EventSubscriptionResumed = "subscription_resumed"

	// EventSubscriptionRevoked fires when a subscription is removed
	// (Unsubscribe call completed: profile updated, lease released).
	EventSubscriptionRevoked = "subscription_revoked"

	// EventRemoteItemArrived fires when an inbound apply observed
	// a new item from the remote backend that wasn't previously in
	// the wiki side. Used to track "Google created a fresh task
	// after delete-recreate" id-churn observability.
	EventRemoteItemArrived = "remote_item_arrived"

	// EventLocalItemPushed fires when an outbound sync successfully
	// pushed a wiki-side change to the remote backend. Used to track
	// per-connector throughput and round-trip latency.
	EventLocalItemPushed = "local_item_pushed"

	// EventForceFullResyncTriggered fires when a connector's
	// truncation-recovery path or an explicit ForceFullResync call
	// schedules a one-shot full re-fetch.
	EventForceFullResyncTriggered = "force_full_resync_triggered"
)
