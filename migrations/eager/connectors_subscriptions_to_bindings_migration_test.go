//revive:disable:dot-imports
package eager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/migrations/eager"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// connectorSubtree returns wiki.connectors.<kind> from a frontmatter map,
// or nil if any segment is missing.
func connectorSubtreeFM(fm wikipage.FrontMatter, kind string) map[string]any {
	wiki := asMap(fm["wiki"])
	conns := asMap(wiki["connectors"])
	c, ok := conns[kind].(map[string]any)
	if !ok {
		return nil
	}
	return c
}

var _ = Describe("ConnectorsSubscriptionsToBindingsMigrationJob", func() {
	var (
		store *fakeReaderMutator
		job   *eager.ConnectorsSubscriptionsToBindingsMigrationJob
	)

	When("the page has a single Tasks subscription in legacy shape", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_alice": {
					"identifier": "profile_alice",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt",
								"email":         "alice@example.com",
								"subscriptions": []any{
									map[string]any{
										"page":              "shopping_lists",
										"list_name":         "groceries",
										"remote_list_id":    "tasklist-id-xyz",
										"remote_list_title": "Groceries",
										"state":             "active",
										"subscribed_at":     "2026-04-01T12:00:00Z",
										"last_synced_seq":   int64(1247),
										"item_id_map":       map[string]any{"uid-1": "task-1"},
										"item_etags":        map[string]any{"task-1": "etag-1"},
										"last_updated_min":  "2026-05-04T14:00:00Z",
										"push_failures":     map[string]any{"uid-1": int64(0)},
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_alice")
			Expect(job.Execute()).To(Succeed())
		})

		It("should remove the legacy subscriptions key", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			Expect(gt).NotTo(HaveKey("subscriptions"))
		})

		It("should write the new bindings key", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			Expect(gt).To(HaveKey("bindings"))
		})

		It("should preserve sibling connector fields (refresh_token, email)", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			Expect(gt["refresh_token"]).To(Equal("rt"))
			Expect(gt["email"]).To(Equal("alice@example.com"))
		})

		It("should produce one binding entry", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			bindings := asAnySlice(gt["bindings"])
			Expect(bindings).To(HaveLen(1))
		})

		It("should route engine-owned identity fields to the top level", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			bindings := asAnySlice(gt["bindings"])
			entry := asMap(bindings[0])
			Expect(entry["page"]).To(Equal("shopping_lists"))
			Expect(entry["list_name"]).To(Equal("groceries"))
		})

		It("should rename remote_list_id to remote_handle", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			bindings := asAnySlice(gt["bindings"])
			entry := asMap(bindings[0])
			Expect(entry["remote_handle"]).To(Equal("tasklist-id-xyz"))
			Expect(entry).NotTo(HaveKey("remote_list_id"))
		})

		It("should rename subscribed_at to bound_at", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			bindings := asAnySlice(gt["bindings"])
			entry := asMap(bindings[0])
			Expect(entry["bound_at"]).To(Equal("2026-04-01T12:00:00Z"))
			Expect(entry).NotTo(HaveKey("subscribed_at"))
		})

		It("should preserve engine-owned state and progress fields top-level", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			bindings := asAnySlice(gt["bindings"])
			entry := asMap(bindings[0])
			Expect(entry["state"]).To(Equal("active"))
			Expect(entry["last_synced_seq"]).To(Equal(int64(1247)))
			Expect(entry["remote_list_title"]).To(Equal("Groceries"))
		})

		It("should collapse adapter-specific fields into adapter_state", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			bindings := asAnySlice(gt["bindings"])
			entry := asMap(bindings[0])
			adapterState := asMap(entry["adapter_state"])
			Expect(adapterState).To(HaveKey("item_id_map"))
			Expect(adapterState).To(HaveKey("item_etags"))
			Expect(adapterState).To(HaveKey("last_updated_min"))
			Expect(adapterState).To(HaveKey("push_failures"))
		})

		It("should not leak engine-owned fields into adapter_state", func() {
			gt := connectorSubtreeFM(store.pages["profile_alice"], "google_tasks")
			bindings := asAnySlice(gt["bindings"])
			entry := asMap(bindings[0])
			adapterState := asMap(entry["adapter_state"])
			Expect(adapterState).NotTo(HaveKey("page"))
			Expect(adapterState).NotTo(HaveKey("list_name"))
			Expect(adapterState).NotTo(HaveKey("remote_list_id"))
			Expect(adapterState).NotTo(HaveKey("remote_list_title"))
			Expect(adapterState).NotTo(HaveKey("state"))
			Expect(adapterState).NotTo(HaveKey("subscribed_at"))
			Expect(adapterState).NotTo(HaveKey("last_synced_seq"))
		})
	})

	When("the page has bindings for both Keep and Tasks in legacy shape", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_bob": {
					"identifier": "profile_bob",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_keep": map[string]any{
								"subscriptions": []any{
									map[string]any{
										"page":           "list1",
										"list_name":      "default",
										"remote_list_id": "keep-note-1",
										"item_mapping":   map[string]any{"uid-1": "server-1"},
										"keep_cursor":    "cursor-token",
										"label_ids":      []any{"label-1"},
									},
								},
							},
							"google_tasks": map[string]any{
								"subscriptions": []any{
									map[string]any{
										"page":           "list2",
										"list_name":      "groceries",
										"remote_list_id": "tasklist-1",
										"item_id_map":    map[string]any{"uid-1": "task-1"},
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_bob")
			Expect(job.Execute()).To(Succeed())
		})

		It("should migrate the Keep subtree", func() {
			keep := connectorSubtreeFM(store.pages["profile_bob"], "google_keep")
			Expect(keep).NotTo(HaveKey("subscriptions"))
			Expect(keep).To(HaveKey("bindings"))
		})

		It("should migrate the Tasks subtree", func() {
			tasks := connectorSubtreeFM(store.pages["profile_bob"], "google_tasks")
			Expect(tasks).NotTo(HaveKey("subscriptions"))
			Expect(tasks).To(HaveKey("bindings"))
		})

		It("should route Keep-specific keys into Keep's adapter_state", func() {
			keep := connectorSubtreeFM(store.pages["profile_bob"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			adapterState := asMap(entry["adapter_state"])
			Expect(adapterState).To(HaveKey("item_mapping"))
			Expect(adapterState).To(HaveKey("keep_cursor"))
			Expect(adapterState).To(HaveKey("label_ids"))
		})

		It("should route Tasks-specific keys into Tasks's adapter_state", func() {
			tasks := connectorSubtreeFM(store.pages["profile_bob"], "google_tasks")
			bindings := asAnySlice(tasks["bindings"])
			entry := asMap(bindings[0])
			adapterState := asMap(entry["adapter_state"])
			Expect(adapterState).To(HaveKey("item_id_map"))
		})
	})

	When("the page has no connectors at all", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_clean": {
					"identifier": "profile_clean",
					"wiki": map[string]any{
						"system": true,
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_clean")
			Expect(job.Execute()).To(Succeed())
		})

		It("should not write the page (no-op)", func() {
			Expect(store.writeCount).To(Equal(0))
		})

		It("should leave the page unchanged", func() {
			Expect(store.pages["profile_clean"]["wiki"]).To(HaveKey("system"))
		})
	})

	When("the page is already in the new shape", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_done": {
					"identifier": "profile_done",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt",
								"bindings": []any{
									map[string]any{
										"page":          "shopping",
										"list_name":     "groceries",
										"remote_handle": "tasklist-1",
										"adapter_state": map[string]any{
											"item_id_map": map[string]any{"uid-1": "task-1"},
										},
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_done")
			Expect(job.Execute()).To(Succeed())
		})

		It("should not write the page (idempotent)", func() {
			Expect(store.writeCount).To(Equal(0))
		})

		It("should leave the bindings key intact", func() {
			gt := connectorSubtreeFM(store.pages["profile_done"], "google_tasks")
			Expect(gt).To(HaveKey("bindings"))
		})
	})

	When("the page has BOTH legacy subscriptions[] and new bindings[]", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_mixed": {
					"identifier": "profile_mixed",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"bindings": []any{
									map[string]any{
										"page":          "shopping",
										"list_name":     "new",
										"remote_handle": "new-handle",
									},
								},
								"subscriptions": []any{
									map[string]any{
										"page":           "shopping",
										"list_name":      "old",
										"remote_list_id": "old-handle",
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_mixed")
			Expect(job.Execute()).To(Succeed())
		})

		It("should write the page exactly once", func() {
			Expect(store.writeCount).To(Equal(1))
		})

		It("should drop the legacy subscriptions key", func() {
			gt := connectorSubtreeFM(store.pages["profile_mixed"], "google_tasks")
			Expect(gt).NotTo(HaveKey("subscriptions"))
		})

		It("should preserve the new-shape bindings unchanged (new shape wins)", func() {
			gt := connectorSubtreeFM(store.pages["profile_mixed"], "google_tasks")
			bindings := asAnySlice(gt["bindings"])
			Expect(bindings).To(HaveLen(1))
			entry := asMap(bindings[0])
			Expect(entry["list_name"]).To(Equal("new"))
			Expect(entry["remote_handle"]).To(Equal("new-handle"))
		})
	})

	When("the page has an empty subscriptions[] array", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_empty": {
					"identifier": "profile_empty",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt",
								"subscriptions": []any{},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_empty")
			Expect(job.Execute()).To(Succeed())
		})

		It("should remove the legacy subscriptions key", func() {
			gt := connectorSubtreeFM(store.pages["profile_empty"], "google_tasks")
			Expect(gt).NotTo(HaveKey("subscriptions"))
		})

		It("should not write a bindings key (empty array would be misleading)", func() {
			gt := connectorSubtreeFM(store.pages["profile_empty"], "google_tasks")
			Expect(gt).NotTo(HaveKey("bindings"))
		})

		It("should preserve other connector fields", func() {
			gt := connectorSubtreeFM(store.pages["profile_empty"], "google_tasks")
			Expect(gt["refresh_token"]).To(Equal("rt"))
		})
	})

	When("the migration is run a second time on already-migrated data", func() {
		var firstWriteCount int

		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_idempotent": {
					"identifier": "profile_idempotent",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"subscriptions": []any{
									map[string]any{
										"page":           "shopping",
										"list_name":      "groceries",
										"remote_list_id": "tasklist-1",
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_idempotent")

			// First run: rewrite legacy → new shape (one write).
			Expect(job.Execute()).To(Succeed())
			firstWriteCount = store.writeCount

			// Second run: should be a no-op.
			Expect(job.Execute()).To(Succeed())
		})

		It("should write exactly once on first run", func() {
			Expect(firstWriteCount).To(Equal(1))
		})

		It("should not write again on the second run", func() {
			Expect(store.writeCount).To(Equal(1))
		})
	})
})
