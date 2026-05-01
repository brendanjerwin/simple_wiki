//revive:disable:dot-imports
//revive:disable:add-constant
//revive:disable:redundant-import-alias
package v1_test

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/keep/bridge"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// keepConnectorClock satisfies bridge.Clock for tests. The Connector
// only consults this for sync operations; the dead-letter handlers
// don't use it, so any fixed time works.
type keepConnectorClock struct{}

func (keepConnectorClock) Now() time.Time {
	return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
}

// keepConnectorTestEmail is the test user's login. Must round-trip
// through ProfileIdentifierFor to derive the binding-store key.
const keepConnectorTestEmail = "alice@example.com"

// keepDeadLetterThreshold mirrors the production constant in
// connector.go. Defined locally so the test reads as data, not
// magic numbers. If the production threshold changes, update here too.
const keepDeadLetterThreshold = 10

// withCallerIdentity wraps a context with a real-user Tailscale identity
// so the requireRealUser handler gate accepts the request.
func withCallerIdentity(ctx context.Context, login string) context.Context {
	return tailscale.ContextWithIdentity(ctx, tailscale.NewIdentity(login, "Alice", "node-1"))
}

var _ = Describe("KeepConnectorService dead-letter handlers", func() {
	var (
		ctx       context.Context
		server    *v1.Server
		mock      *MockPageReaderMutator
		profileID wikipage.PageIdentifier
	)

	const (
		page     = "Groceries"
		listName = "weekly"
	)

	BeforeEach(func() {
		var err error
		profileID, err = wikipage.ProfileIdentifierFor(keepConnectorTestEmail)
		Expect(err).ToNot(HaveOccurred())

		mock = &MockPageReaderMutator{}
		// Seed the profile page with a connector and one binding so
		// FindBinding succeeds. ItemIDMap shape mirrors what the
		// frontmatter codec produces.
		mock.WrittenFrontmatterByID = map[string]map[string]any{
			string(profileID): {
				"identifier": string(profileID),
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_keep": map[string]any{
							"email":        keepConnectorTestEmail,
							"master_token": "tok",
							"bindings": []any{
								map[string]any{
									"page":                  page,
									"list_name":             listName,
									"keep_note_id":          "srv-list-1",
									"migrated_fingerprints": true,
									"item_id_map": map[string]any{
										"uid-below": map[string]any{
											"server_id":               "srv-A",
											"last_observed_wiki_text": "below threshold",
											"push_failure_count":      keepDeadLetterThreshold - 1,
											"last_failure_code":       "TRANSIENT",
										},
										"uid-at": map[string]any{
											"server_id":               "srv-B",
											"last_observed_wiki_text": "at threshold",
											"push_failure_count":      keepDeadLetterThreshold,
											"last_failure_code":       "REJECTED",
											"synced_text":             "synced",
											"synced_checked":          true,
										},
										"uid-above": map[string]any{
											"server_id":               "srv-C",
											"last_observed_wiki_text": "above threshold",
											"push_failure_count":      keepDeadLetterThreshold + 5,
											"last_failure_code":       "FATAL",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		store := bridge.NewBindingStore(mock)
		connector := bridge.NewConnector(store, http.DefaultClient, keepConnectorClock{})

		server = mustNewServer(mock, nil, nil).WithKeepConnector(connector)
		ctx = withCallerIdentity(context.Background(), keepConnectorTestEmail)
	})

	Describe("ListDeadLetters", func() {
		Describe("when called for a binding with mixed PushFailureCount values", func() {
			var (
				resp *apiv1.ListDeadLettersResponse
				err  error
			)

			BeforeEach(func() {
				resp, err = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
					Page:     page,
					ListName: listName,
				})
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("list_dead_letters_returns_only_items_at_or_above_threshold", func() {
				uids := make([]string, 0, len(resp.GetItems()))
				for _, it := range resp.GetItems() {
					uids = append(uids, it.GetItemUid())
				}
				Expect(uids).To(ConsistOf("uid-at", "uid-above"))
			})

			It("should expose the failure code on each returned item", func() {
				byUID := map[string]*apiv1.DeadLetterItem{}
				for _, it := range resp.GetItems() {
					byUID[it.GetItemUid()] = it
				}
				Expect(byUID["uid-at"].GetLastFailureCode()).To(Equal("REJECTED"))
				Expect(byUID["uid-above"].GetLastFailureCode()).To(Equal("FATAL"))
			})

			It("should expose the failure count on each returned item", func() {
				byUID := map[string]*apiv1.DeadLetterItem{}
				for _, it := range resp.GetItems() {
					byUID[it.GetItemUid()] = it
				}
				Expect(byUID["uid-at"].GetPushFailureCount()).To(Equal(int32(keepDeadLetterThreshold)))
				Expect(byUID["uid-above"].GetPushFailureCount()).To(Equal(int32(keepDeadLetterThreshold + 5)))
			})

			It("should expose the LastObservedWikiText as the row label", func() {
				byUID := map[string]*apiv1.DeadLetterItem{}
				for _, it := range resp.GetItems() {
					byUID[it.GetItemUid()] = it
				}
				Expect(byUID["uid-at"].GetText()).To(Equal("at threshold"))
				Expect(byUID["uid-above"].GetText()).To(Equal("above threshold"))
			})
		})

		Describe("when called without an authenticated user identity", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ListDeadLetters(context.Background(), &apiv1.ListDeadLettersRequest{
					Page:     page,
					ListName: listName,
				})
			})

			It("should return PermissionDenied", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.PermissionDenied, "real user identity"))
			})
		})

		Describe("when called with empty page or list_name", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
					Page:     "",
					ListName: listName,
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("ClearDeadLetter", func() {
		Describe("when called for a dead-lettered item", func() {
			var (
				err              error
				postClearListing *apiv1.ListDeadLettersResponse
				postFM           map[string]any
			)

			BeforeEach(func() {
				_, err = server.ClearDeadLetter(ctx, &apiv1.ClearDeadLetterRequest{
					Page:     page,
					ListName: listName,
					ItemUid:  "uid-at",
				})

				// Verify by re-listing.
				if err == nil {
					postClearListing, _ = server.ListDeadLetters(ctx, &apiv1.ListDeadLettersRequest{
						Page:     page,
						ListName: listName,
					})
				}
				// Inspect the persisted frontmatter to verify
				// synced_fp survived the clear.
				postFM = mock.WrittenFrontmatterByID[string(profileID)]
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("clear_dead_letter_resets_count_and_code_but_preserves_synced_fp — uid-at no longer appears in the dead-letter list", func() {
				uids := make([]string, 0)
				for _, it := range postClearListing.GetItems() {
					uids = append(uids, it.GetItemUid())
				}
				Expect(uids).ToNot(ContainElement("uid-at"))
				Expect(uids).To(ContainElement("uid-above"))
			})

			It("clear_dead_letter_resets_count_and_code_but_preserves_synced_fp — synced_text is preserved on the item", func() {
				Expect(postFM).ToNot(BeNil())
				idMap := navigateMap(postFM, "wiki", "connectors", "google_keep", "bindings")
				bindings, ok := idMap.([]any)
				Expect(ok).To(BeTrue(), "bindings should be a list, got %T", idMap)
				Expect(bindings).To(HaveLen(1))
				binding, ok := bindings[0].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := binding["item_id_map"].(map[string]any)
				Expect(ok).To(BeTrue())
				atItem, ok := items["uid-at"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(atItem["synced_text"]).To(Equal("synced"))
				Expect(atItem["synced_checked"]).To(Equal(true))
				// push_failure_count should have been zeroed; codec
				// elides zero-valued fields, so it should be absent.
				_, hasFailureCount := atItem["push_failure_count"]
				Expect(hasFailureCount).To(BeFalse(), "push_failure_count should have been cleared (and codec elides zeros)")
				_, hasFailureCode := atItem["last_failure_code"]
				Expect(hasFailureCode).To(BeFalse(), "last_failure_code should have been cleared (and codec elides empties)")
			})
		})

		Describe("when called for an unknown item_uid", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ClearDeadLetter(ctx, &apiv1.ClearDeadLetterRequest{
					Page:     page,
					ListName: listName,
					ItemUid:  "uid-bogus",
				})
			})

			It("clear_dead_letter_returns_not_found_for_unknown_item_uid", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "dead_letter_item_not_found"))
			})
		})

		Describe("when called for an unknown binding", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ClearDeadLetter(ctx, &apiv1.ClearDeadLetterRequest{
					Page:     "ghost-page",
					ListName: "ghost-list",
					ItemUid:  "uid-x",
				})
			})

			It("should return NotFound binding_not_found", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "binding_not_found"))
			})
		})

		Describe("when called with empty arguments", func() {
			var err error

			BeforeEach(func() {
				_, err = server.ClearDeadLetter(ctx, &apiv1.ClearDeadLetterRequest{
					Page:     page,
					ListName: listName,
					ItemUid:  "",
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "item_uid"))
			})
		})
	})
})

// navigateMap walks a chain of map[string]any keys and returns the
// terminal value, or nil if any link is missing or wrong-typed.
func navigateMap(start any, keys ...string) any {
	cur := start
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}
