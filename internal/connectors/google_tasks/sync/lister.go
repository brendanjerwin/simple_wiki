package sync

import (
	"sync"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// ActiveSubscriptions tracks every subscription the current process
// knows about so the SyncScheduler's tick can enumerate them without
// re-scanning the page store on each tick. Populated at bootstrap
// (one-time fan-out scan of profile_* pages) and updated in-process
// on Subscribe/Unsubscribe.
type ActiveSubscriptions struct {
	mu    sync.Mutex
	known map[connectors.SubscriptionKey]struct{}
}

// NewActiveSubscriptions constructs an empty tracker.
func NewActiveSubscriptions() *ActiveSubscriptions {
	return &ActiveSubscriptions{known: map[connectors.SubscriptionKey]struct{}{}}
}

// Add records a subscription as active.
func (a *ActiveSubscriptions) Add(k connectors.SubscriptionKey) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.known[k] = struct{}{}
}

// Remove forgets a subscription.
func (a *ActiveSubscriptions) Remove(k connectors.SubscriptionKey) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.known, k)
}

// Snapshot returns a copy of the currently-tracked set.
func (a *ActiveSubscriptions) Snapshot() []connectors.SubscriptionKey {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]connectors.SubscriptionKey, 0, len(a.known))
	for k := range a.known {
		out = append(out, k)
	}
	return out
}

// Lister adapts ActiveSubscriptions.Snapshot to the
// connectors.SubscriptionLister type for SyncScheduler.Register.
func (a *ActiveSubscriptions) Lister() connectors.SubscriptionLister {
	return func() []connectors.SubscriptionKey {
		return a.Snapshot()
	}
}

// SubscriptionsForProfile returns every (page, list_name) the given
// profile currently has subscribed (active or paused) on disk. Used
// by bootstrap to seed ActiveSubscriptions and by the LeaseTable
// rebuild fan-out scan.
//
// The connector's store is the authoritative source.
func (c *Connector) SubscriptionsForProfile(profileID wikipage.PageIdentifier) ([]connectors.SubscriptionKey, error) {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return nil, err
	}
	out := make([]connectors.SubscriptionKey, 0, len(state.Subscriptions))
	for _, sub := range state.Subscriptions {
		out = append(out, connectors.SubscriptionKey{
			ProfileID: string(profileID),
			Page:      sub.Page,
			ListName:  sub.ListName,
		})
	}
	return out, nil
}
