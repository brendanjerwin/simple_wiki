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

	When("a legacy Keep subscription carries item_id_map structured entries", func() {
		// Legacy Keep persisted ItemMapping under item_id_map[uid] = {
		//   server_id, base_version, client_id,
		//   synced_text, synced_checked, synced_sort_value,
		//   last_observed_wiki_*, push_failure_count, ...
		// }. The new KeepAdapter reads item_mapping[server_id] = {
		//   server_id, base_version, client_id
		// }. The migration must translate the shape AND drop the legacy
		// fingerprint baselines (per the Phase 7 plan).
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_carol": {
					"identifier": "profile_carol",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_keep": map[string]any{
								"subscriptions": []any{
									map[string]any{
										"page":           "shopping",
										"list_name":     "groceries",
										"remote_list_id": "keep-note-1",
										"keep_cursor":    "cursor-token",
										"item_id_map": map[string]any{
											"uid-A": map[string]any{
												"server_id":         "server-A",
												"base_version":      "v-A",
												"client_id":         "client-A",
												"synced_text":       "Milk",
												"synced_checked":    false,
												"synced_sort_value": "1000",
											},
											"uid-B": map[string]any{
												"server_id":    "server-B",
												"base_version": "v-B",
												"client_id":    "client-B",
											},
										},
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_carol")
			Expect(job.Execute()).To(Succeed())
		})

		It("should write item_mapping under adapter_state (Keep adapter's structured shape)", func() {
			keep := connectorSubtreeFM(store.pages["profile_carol"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			adapterState := asMap(entry["adapter_state"])
			Expect(adapterState).To(HaveKey("item_mapping"))
		})

		It("should index item_mapping by server_id", func() {
			keep := connectorSubtreeFM(store.pages["profile_carol"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			itemMapping := asMap(asMap(entry["adapter_state"])["item_mapping"])
			Expect(itemMapping).To(HaveKey("server-A"))
			Expect(itemMapping).To(HaveKey("server-B"))
			Expect(itemMapping).NotTo(HaveKey("uid-A"))
			Expect(itemMapping).NotTo(HaveKey("uid-B"))
		})

		It("should preserve server_id, base_version, client_id on each entry", func() {
			keep := connectorSubtreeFM(store.pages["profile_carol"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			itemMapping := asMap(asMap(entry["adapter_state"])["item_mapping"])
			a := asMap(itemMapping["server-A"])
			Expect(a["server_id"]).To(Equal("server-A"))
			Expect(a["base_version"]).To(Equal("v-A"))
			Expect(a["client_id"]).To(Equal("client-A"))
		})

		It("should drop legacy fingerprint baselines (synced_text/checked/sort_value)", func() {
			keep := connectorSubtreeFM(store.pages["profile_carol"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			itemMapping := asMap(asMap(entry["adapter_state"])["item_mapping"])
			a := asMap(itemMapping["server-A"])
			Expect(a).NotTo(HaveKey("synced_text"))
			Expect(a).NotTo(HaveKey("synced_checked"))
			Expect(a).NotTo(HaveKey("synced_sort_value"))
		})

		It("should preserve adjacent adapter_state keys (keep_cursor)", func() {
			keep := connectorSubtreeFM(store.pages["profile_carol"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			adapterState := asMap(entry["adapter_state"])
			Expect(adapterState["keep_cursor"]).To(Equal("cursor-token"))
		})

		It("should also write item_id_map (uid -> server_id) for the engine's flat lookup", func() {
			// The engine's reconcile loop reads item_id_map[uid] = serverID
			// (flat map[string]string) to decide insert vs. patch. Without
			// this, the engine treats every item as new and dead-letters.
			keep := connectorSubtreeFM(store.pages["profile_carol"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			adapterState := asMap(entry["adapter_state"])
			itemIDMap := asMap(adapterState["item_id_map"])
			Expect(itemIDMap["uid-A"]).To(Equal("server-A"))
			Expect(itemIDMap["uid-B"]).To(Equal("server-B"))
		})
	})

	When("a legacy Keep subscription carries item_id_map flat-string entries", func() {
		// Old-old Keep shape: item_id_map[uid] = "server_id" (just a
		// string). The migration must still produce the new shape; the
		// resulting item_mapping entry has only server_id populated
		// (base_version/client_id absent, will be repopulated by the
		// engine's RebuildAdapterState on next tick).
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_dave": {
					"identifier": "profile_dave",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_keep": map[string]any{
								"subscriptions": []any{
									map[string]any{
										"page":           "shopping",
										"list_name":     "groceries",
										"remote_list_id": "keep-note-1",
										"item_id_map": map[string]any{
											"uid-X": "server-X",
										},
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_dave")
			Expect(job.Execute()).To(Succeed())
		})

		It("should index item_mapping by the string server_id", func() {
			keep := connectorSubtreeFM(store.pages["profile_dave"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			itemMapping := asMap(asMap(entry["adapter_state"])["item_mapping"])
			Expect(itemMapping).To(HaveKey("server-X"))
			Expect(asMap(itemMapping["server-X"])["server_id"]).To(Equal("server-X"))
		})

		It("should also write item_id_map (uid -> server_id)", func() {
			keep := connectorSubtreeFM(store.pages["profile_dave"], "google_keep")
			bindings := asAnySlice(keep["bindings"])
			entry := asMap(bindings[0])
			itemIDMap := asMap(asMap(entry["adapter_state"])["item_id_map"])
			Expect(itemIDMap["uid-X"]).To(Equal("server-X"))
		})
	})

	When("a legacy Tasks subscription carries synced_items fingerprint baselines", func() {
		// Tasks's legacy ConnectorState had a per-binding synced_items
		// subtree (per-uid fingerprint baselines used for the old
		// fingerprint-based divergence check). The new engine uses
		// causal divergence (op-log + last_synced_seq) and never reads
		// synced_items. The migration must drop it.
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"profile_erin": {
					"identifier": "profile_erin",
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"subscriptions": []any{
									map[string]any{
										"page":           "shopping",
										"list_name":     "groceries",
										"remote_list_id": "tasklist-1",
										"item_id_map":    map[string]any{"uid-1": "task-1"},
										"synced_items": map[string]any{
											"uid-1": map[string]any{
												"synced_title":  "old title",
												"synced_status": "needsAction",
											},
										},
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewConnectorsSubscriptionsToBindingsMigrationJob(store, "profile_erin")
			Expect(job.Execute()).To(Succeed())
		})

		It("should drop synced_items from adapter_state", func() {
			tasks := connectorSubtreeFM(store.pages["profile_erin"], "google_tasks")
			bindings := asAnySlice(tasks["bindings"])
			entry := asMap(bindings[0])
			adapterState := asMap(entry["adapter_state"])
			Expect(adapterState).NotTo(HaveKey("synced_items"))
		})

		It("should preserve item_id_map (Tasks engine still uses this shape)", func() {
			tasks := connectorSubtreeFM(store.pages["profile_erin"], "google_tasks")
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
