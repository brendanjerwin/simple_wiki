//revive:disable:dot-imports
//revive:disable:add-constant
package sync_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// newReadyLeaseTable returns a LeaseTable already in the ready state
// for tests. Production wiring blocks Subscribe until the boot rebuild
// signals ready; tests skip the rebuild and signal immediately.
func newReadyLeaseTable() *connectors.LeaseTable {
	lt := connectors.NewLeaseTable()
	lt.SignalReady()
	return lt
}

// profileA is a stable profile ID used across the coverage tests.
const profileA = wikipage.PageIdentifier("alice-profile")

// baseConnectedState builds a ConnectorState with email + token but no
// bindings. Tests that need bindings append them before calling
// SaveState.
func baseConnectedState() keepsync.ConnectorState {
	return keepsync.ConnectorState{
		Email:       "alice@example.com",
		MasterToken: "test-master-token",
	}
}

// saveTo is a convenience to SaveState via a SubscriptionStore without
// needing a local variable for the store.
func saveTo(store *fakeStore, profileID wikipage.PageIdentifier, state keepsync.ConnectorState) {
	bs := keepsync.NewSubscriptionStore(store)
	Expect(bs.SaveState(profileID, state)).To(Succeed())
}

// newCoverageConnector builds a Connector wired with fakeAuth and a
// caller-supplied KeepClient. Both checklist reader and mutator are
// wired to a fakeChecklist by default; callers can swap them with
// SetChecklistReader / SetChecklistMutator.
func newCoverageConnector(store *fakeStore, kc keepsync.KeepClient) (*keepsync.Connector, *fakeChecklist) {
	c, nerr := keepsync.NewConnector(keepsync.NewSubscriptionStore(store), newReadyLeaseTable(), nil, fakeClock{})
	Expect(nerr).ToNot(HaveOccurred())
	c.SetAuthBuilder(func(_ string) keepsync.AuthExchanger { return fakeAuth{} })
	c.SetClientBuilder(func(_ string) keepsync.KeepClient { return kc })
	chk := &fakeChecklist{}
	c.SetChecklistReader(chk)
	c.SetChecklistMutator(chk)
	c.SetSyncSuppressor(fakeSuppressor{})
	return c, chk
}

var _ = Describe("Connector.GetState", func() {
	var (
		ctx     context.Context
		store   *fakeStore
		profile = profileA
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = newFakeStore()
	})

	When("the connector is configured", func() {
		var (
			state keepsync.ConnectorState
			err   error
		)

		BeforeEach(func() {
			saved := baseConnectedState()
			saveTo(store, profile, saved)
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			state, err = c.GetState(ctx, profile)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the email", func() {
			Expect(state.Email).To(Equal("alice@example.com"))
		})

		It("should return IsConfigured() == true", func() {
			Expect(state.IsConfigured()).To(BeTrue())
		})
	})

	When("no state has been stored (profile is new)", func() {
		var (
			state keepsync.ConnectorState
			err   error
		)

		BeforeEach(func() {
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			state, err = c.GetState(ctx, profile)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return IsConfigured() == false (zero state)", func() {
			Expect(state.IsConfigured()).To(BeFalse())
		})
	})
})

var _ = Describe("Connector.Disconnect", func() {
	var (
		ctx     context.Context
		store   *fakeStore
		profile = profileA
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = newFakeStore()
		saveTo(store, profile, baseConnectedState())
	})

	When("the user has a configured connector", func() {
		var (
			returned keepsync.ConnectorState
			err      error
		)

		BeforeEach(func() {
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			returned, err = c.Disconnect(ctx, profile)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should clear the master token", func() {
			Expect(returned.MasterToken).To(BeEmpty())
		})

		It("should preserve the email", func() {
			Expect(returned.Email).To(Equal("alice@example.com"))
		})

		It("should persist the cleared state so GetState reflects it", func() {
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			st, _ := c.GetState(ctx, profile)
			Expect(st.MasterToken).To(BeEmpty())
		})
	})
})

var _ = Describe("Connector.ListNotes", func() {
	var (
		ctx     context.Context
		store   *fakeStore
		profile = profileA
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = newFakeStore()
		saveTo(store, profile, baseConnectedState())
	})

	When("Keep returns a mix of live lists and trashed lists", func() {
		var (
			notes []keepsync.KeepNoteSummary
			err   error
		)

		BeforeEach(func() {
			kc := &fakeKeepClient{}
			kc.pullState = gateway.ChangesResponse{
				ToVersion: "v-1",
				Nodes: []gateway.Node{
					{
						Type:     gateway.NodeTypeList,
						ID:       "client-list-1",
						ServerID: "srv-list-1",
						Title:    "Groceries",
						Timestamps: gateway.Timestamps{
							Created: tNow.Add(-24 * time.Hour),
							Updated: tNow,
						},
					},
					{
						Type:     gateway.NodeTypeList,
						ID:       "client-list-2",
						ServerID: "srv-list-2",
						Title:    "Old List",
						Timestamps: gateway.Timestamps{
							Created: tNow.Add(-48 * time.Hour),
							Updated: tNow.Add(-48 * time.Hour),
							Trashed: tNow.Add(-1 * time.Hour), // trashed
						},
					},
					{
						Type:     gateway.NodeTypeListItem,
						ID:       "client-item-1",
						ServerID: "srv-item-1",
						ParentID: "srv-list-1",
						Text:     "Milk",
						Timestamps: gateway.Timestamps{
							Created: tNow.Add(-24 * time.Hour),
							Updated: tNow,
						},
					},
				},
			}
			c, _ := newCoverageConnector(store, kc)
			notes, err = c.ListNotes(ctx, profile)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return only the live list", func() {
			Expect(notes).To(HaveLen(1))
			Expect(notes[0].KeepNoteID).To(Equal("srv-list-1"))
		})

		It("should set the title from the LIST node", func() {
			Expect(notes[0].Title).To(Equal("Groceries"))
		})

		It("should count the live item under the list", func() {
			Expect(notes[0].ItemCount).To(Equal(1))
		})
	})

	When("the connector is not configured for this profile", func() {
		var err error

		BeforeEach(func() {
			// Store has no state for this profile.
			emptyStore := newFakeStore()
			c, _ := newCoverageConnector(emptyStore, &fakeKeepClient{})
			_, err = c.ListNotes(ctx, "unknown-profile")
		})

		It("should return ErrConnectorNotConfigured", func() {
			Expect(err).To(MatchError(keepsync.ErrConnectorNotConfigured))
		})
	})
})

var _ = Describe("Connector.Bind", func() {
	var (
		ctx     context.Context
		store   *fakeStore
		profile = profileA
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = newFakeStore()
		saveTo(store, profile, baseConnectedState())
	})

	When("binding a new list with an empty keepNoteID (create-new-note path)", func() {
		var (
			binding keepsync.Subscription
			err     error
		)

		BeforeEach(func() {
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			binding, err = c.Bind(ctx, profile, "shopping", "groceries", "", nil)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should record the page and list name", func() {
			Expect(binding.Page).To(Equal("shopping"))
			Expect(binding.ListName).To(Equal("groceries"))
		})

		It("should leave KeepNoteID empty (sync creates it on first tick)", func() {
			Expect(binding.KeepNoteID).To(BeEmpty())
		})

		It("should mark the binding as born-migrated", func() {
			Expect(binding.MigratedFingerprints).To(BeTrue())
		})
	})

	When("binding to an existing Keep note", func() {
		var (
			binding keepsync.Subscription
			err     error
		)

		BeforeEach(func() {
			kc := &fakeKeepClient{}
			kc.pullState = gateway.ChangesResponse{
				ToVersion: "v-1",
				Nodes: []gateway.Node{
					{
						Type:     gateway.NodeTypeList,
						ID:       "client-list-1",
						ServerID: "srv-existing",
						Timestamps: gateway.Timestamps{
							Created: tNow,
							Updated: tNow,
						},
					},
				},
			}
			c, _ := newCoverageConnector(store, kc)
			binding, err = c.Bind(ctx, profile, "shopping", "groceries", "srv-existing", nil)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should record the Keep note ID", func() {
			Expect(binding.KeepNoteID).To(Equal("srv-existing"))
		})
	})
})

var _ = Describe("Connector.SyncToKeep error paths", func() {
	var (
		ctx     context.Context
		store   *fakeStore
		profile = profileA
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = newFakeStore()
	})

	When("the ChecklistReader is not configured", func() {
		var err error

		BeforeEach(func() {
			saveTo(store, profile, baseConnectedState())
			c, nerr := keepsync.NewConnector(keepsync.NewSubscriptionStore(store), newReadyLeaseTable(), nil, fakeClock{})
	Expect(nerr).ToNot(HaveOccurred())
			c.SetAuthBuilder(func(_ string) keepsync.AuthExchanger { return fakeAuth{} })
			c.SetClientBuilder(func(_ string) keepsync.KeepClient { return &fakeKeepClient{} })
			// Intentionally NOT calling SetChecklistReader — leaves it nil.
			err = c.SyncToKeep(ctx, profile, "shopping", "groceries")
		})

		It("should return ErrChecklistReaderUnavailable", func() {
			Expect(err).To(MatchError(keepsync.ErrChecklistReaderUnavailable))
		})
	})

	When("no binding exists for the given page and list", func() {
		var err error

		BeforeEach(func() {
			saveTo(store, profile, baseConnectedState())
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			// Profile has no bindings, so FindSubscription returns not-found.
			err = c.SyncToKeep(ctx, profile, "missing-page", "missing-list")
		})

		It("should return nil (no binding → no-op, not an error)", func() {
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("the binding is not yet migrated", func() {
		var err error

		BeforeEach(func() {
			state := baseConnectedState()
			state.Subscriptions = []keepsync.Subscription{{
				Page:                 "shopping",
				ListName:             "groceries",
				KeepNoteID:           "srv-note",
				MigratedFingerprints: false, // un-migrated
				SubscribedAt:              tNow,
			}}
			saveTo(store, profile, state)
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			err = c.SyncToKeep(ctx, profile, "shopping", "groceries")
		})

		It("should return nil (skip until migration is complete)", func() {
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("Connector.MigrateSubscriptionFingerprints early-exit paths", func() {
	var (
		ctx     context.Context
		store   *fakeStore
		profile = profileA
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = newFakeStore()
	})

	When("the binding is already migrated", func() {
		var err error

		BeforeEach(func() {
			state := baseConnectedState()
			state.Subscriptions = []keepsync.Subscription{{
				Page:                 "p",
				ListName:             "list",
				KeepNoteID:           "srv-note",
				MigratedFingerprints: true, // already done
				SubscribedAt:              tNow,
			}}
			saveTo(store, profile, state)
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			err = c.MigrateSubscriptionFingerprints(ctx, profile, "p", "list")
		})

		It("should return nil (idempotent no-op)", func() {
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("the binding does not exist", func() {
		var err error

		BeforeEach(func() {
			// Profile has no bindings.
			saveTo(store, profile, baseConnectedState())
			c, _ := newCoverageConnector(store, &fakeKeepClient{})
			err = c.MigrateSubscriptionFingerprints(ctx, profile, "missing-page", "missing-list")
		})

		It("should return nil (binding removed — succeed silently)", func() {
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("the ChecklistReader is not configured", func() {
		var err error

		BeforeEach(func() {
			saveTo(store, profile, baseConnectedState())
			c, nerr := keepsync.NewConnector(keepsync.NewSubscriptionStore(store), newReadyLeaseTable(), nil, fakeClock{})
	Expect(nerr).ToNot(HaveOccurred())
			c.SetAuthBuilder(func(_ string) keepsync.AuthExchanger { return fakeAuth{} })
			c.SetClientBuilder(func(_ string) keepsync.KeepClient { return &fakeKeepClient{} })
			// No SetChecklistReader — leaves it nil.
			err = c.MigrateSubscriptionFingerprints(ctx, profile, "p", "list")
		})

		It("should return ErrChecklistReaderUnavailable", func() {
			Expect(err).To(MatchError(keepsync.ErrChecklistReaderUnavailable))
		})
	})
})
