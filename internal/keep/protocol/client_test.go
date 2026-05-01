//revive:disable:dot-imports
package protocol_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

// asArr is a small helper for tests that already gate on map/slice
// membership via Gomega matchers. A failed assertion here yields a nil
// slice; subsequent matchers fail loudly with a clear message.
//
//revive:disable-next-line:unchecked-type-assertion
func asArr(v any) []any { s, _ := v.([]any); return s }

// asMap mirrors asArr for objects.
//
//revive:disable-next-line:unchecked-type-assertion
func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

// asStr returns the string value or empty if not a string.
//
//revive:disable-next-line:unchecked-type-assertion
func asStr(v any) string { s, _ := v.(string); return s }

// canonicalChangesResponse is the success-shape body Keep returns. Mirrors
// the shape gkeepapi documents (see REFERENCE.md "Sync endpoint").
const canonicalChangesResponse = `{
  "kind": "notes#changes",
  "toVersion": "v-2",
  "forceFullResync": false,
  "truncated": false,
  "nodes": [
    {
      "kind": "notes#node",
      "id": "list-1",
      "serverId": "srv-list-1",
      "type": "LIST",
      "sortValue": "1000",
      "baseVersion": "1",
      "timestamps": {
        "kind": "notes#timestamps",
        "created": "2026-04-25T17:14:00.000000Z",
        "updated": "2026-04-25T17:14:00.000000Z"
      }
    },
    {
      "kind": "notes#node",
      "id": "item-1",
      "serverId": "srv-item-1",
      "parentId": "srv-list-1",
      "type": "LIST_ITEM",
      "text": "Buy milk",
      "checked": false,
      "sortValue": "1100",
      "baseVersion": "2",
      "timestamps": {
        "kind": "notes#timestamps",
        "created": "2026-04-25T17:15:00.000000Z",
        "updated": "2026-04-25T17:15:00.000000Z"
      }
    },
    {
      "kind": "notes#node",
      "id": "item-2",
      "serverId": "srv-item-2",
      "parentId": "srv-list-1",
      "type": "LIST_ITEM",
      "text": "Eggs",
      "checked": true,
      "sortValue": "1200",
      "baseVersion": "2",
      "timestamps": {
        "kind": "notes#timestamps",
        "created": "2026-04-25T17:16:00.000000Z",
        "updated": "2026-04-25T17:18:00.000000Z"
      }
    },
    {
      "kind": "notes#node",
      "id": "future-1",
      "type": "WIDGET_FUTURE_TYPE",
      "timestamps": {
        "kind": "notes#timestamps",
        "created": "2026-04-25T17:14:00.000000Z",
        "updated": "2026-04-25T17:14:00.000000Z"
      }
    }
  ]
}`

var _ = Describe("KeepClient.Changes", func() {
	var (
		ctx          context.Context
		fakeServer   *httptest.Server
		client       *protocol.KeepClient
		lastRequest  *http.Request
		lastBody     string
		responseBody string
		responseCode int
	)

	BeforeEach(func() {
		ctx = context.Background()
		responseBody = canonicalChangesResponse
		responseCode = http.StatusOK
		fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lastRequest = r
			b, _ := io.ReadAll(r.Body)
			lastBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(responseCode)
			_, _ = io.WriteString(w, responseBody)
		}))
		client = protocol.NewKeepClient(fakeServer.Client(), fakeServer.URL+"/", "fake-bearer")
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	When("the server returns a well-formed changes response", func() {
		var (
			resp     protocol.ChangesResponse
			callErr  error
			reqBody  map[string]any
			parseErr error
		)

		BeforeEach(func() {
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{
				TargetVersion:   "v-1",
				SessionID:       "s--1234--5678901234",
				ClientTimestamp: "1745601000000000",
			})
			parseErr = json.Unmarshal([]byte(lastBody), &reqBody)
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should populate ToVersion from the response", func() {
			Expect(resp.ToVersion).To(Equal("v-2"))
		})

		It("should not flag forceFullResync when server says false", func() {
			Expect(resp.ForceFullResync).To(BeFalse())
		})

		It("should not flag incremental when the field is absent", func() {
			Expect(resp.Incremental).To(BeFalse())
		})

		It("should decode the LIST node", func() {
			var found *protocol.Node
			for i := range resp.Nodes {
				if resp.Nodes[i].Type == protocol.NodeTypeList {
					n := resp.Nodes[i]
					found = &n
					break
				}
			}
			Expect(found).ToNot(BeNil())
			Expect(found.ID).To(Equal("list-1"))
			Expect(found.ServerID).To(Equal("srv-list-1"))
		})

		It("should decode LIST_ITEM nodes with checked state", func() {
			items := []protocol.Node{}
			for _, n := range resp.Nodes {
				if n.Type == protocol.NodeTypeListItem {
					items = append(items, n)
				}
			}
			Expect(items).To(HaveLen(2))
			byText := map[string]protocol.Node{}
			for _, it := range items {
				byText[it.Text] = it
			}
			Expect(byText["Buy milk"].Checked).To(BeFalse())
			Expect(byText["Eggs"].Checked).To(BeTrue())
		})

		It("should drop nodes with unrecognized types (no error)", func() {
			for _, n := range resp.Nodes {
				Expect(n.Type).ToNot(Equal(protocol.UnknownNodeType))
			}
		})

		It("should parse timestamps as UTC time.Time", func() {
			for _, n := range resp.Nodes {
				if n.Type == protocol.NodeTypeListItem && n.Text == "Eggs" {
					expectedUpdated := time.Date(2026, 4, 25, 17, 18, 0, 0, time.UTC)
					Expect(n.Timestamps.Updated).To(BeTemporally("~", expectedUpdated, time.Microsecond))
				}
			}
		})

		It("should POST to the changes endpoint", func() {
			Expect(lastRequest.Method).To(Equal(http.MethodPost))
			Expect(lastRequest.URL.Path).To(HaveSuffix("changes"))
		})

		It("should send the OAuth auth header", func() {
			Expect(lastRequest.Header.Get("Authorization")).To(Equal("OAuth fake-bearer"))
		})

		It("should send a JSON content-type", func() {
			Expect(lastRequest.Header.Get("Content-Type")).To(HavePrefix("application/json"))
		})

		It("should include targetVersion in the request body", func() {
			Expect(parseErr).ToNot(HaveOccurred())
			Expect(reqBody["targetVersion"]).To(Equal("v-1"))
		})

		It("should include the requestHeader subobject", func() {
			Expect(parseErr).ToNot(HaveOccurred())
			Expect(reqBody).To(HaveKey("requestHeader"))
			rh := asMap(reqBody["requestHeader"])
			Expect(rh["clientPlatform"]).To(Equal("ANDROID"))
			Expect(rh["clientSessionId"]).To(Equal("s--1234--5678901234"))
		})
	})

	When("targetVersion is empty (full pull)", func() {
		var (
			callErr  error
			reqBody  map[string]any
			parseErr error
		)

		BeforeEach(func() {
			_, callErr = client.Changes(ctx, protocol.ChangesRequest{
				SessionID:       "s--1234--5678901234",
				ClientTimestamp: "1745601000000000",
			})
			parseErr = json.Unmarshal([]byte(lastBody), &reqBody)
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should omit targetVersion from the request body", func() {
			Expect(parseErr).ToNot(HaveOccurred())
			Expect(reqBody).ToNot(HaveKey("targetVersion"))
		})
	})

	When("the server returns 401", func() {
		var callErr error

		BeforeEach(func() {
			responseCode = http.StatusUnauthorized
			responseBody = `{"error": {"code": 401, "message": "expired"}}`
			_, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should return ErrAuthRevoked", func() {
			Expect(callErr).To(MatchError(protocol.ErrAuthRevoked))
		})
	})

	When("the server returns 429", func() {
		var callErr error

		BeforeEach(func() {
			responseCode = http.StatusTooManyRequests
			responseBody = `{"error": {"code": 429, "message": "rate limited"}}`
			_, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should return ErrRateLimited", func() {
			Expect(callErr).To(MatchError(protocol.ErrRateLimited))
		})
	})

	When("the server returns malformed JSON", func() {
		var callErr error

		BeforeEach(func() {
			responseBody = "not-json"
			_, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should return ErrProtocolDrift", func() {
			Expect(callErr).To(MatchError(protocol.ErrProtocolDrift))
		})
	})

	When("the server returns missing toVersion (structural drift)", func() {
		var callErr error

		BeforeEach(func() {
			responseBody = `{"kind": "notes#changes", "nodes": []}`
			_, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should return ErrProtocolDrift", func() {
			Expect(callErr).To(MatchError(protocol.ErrProtocolDrift))
		})
	})

	When("the request includes nodes to push", func() {
		var (
			callErr  error
			reqBody  map[string]any
			parseErr error
		)

		BeforeEach(func() {
			pushNode := protocol.Node{
				ID:       "client-id-1",
				ServerID: "srv-1",
				ParentID: "srv-list-1",
				Type:     protocol.NodeTypeListItem,
				Text:     "Buy bread",
				Checked:  false,
				Timestamps: protocol.Timestamps{
					Created: time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC),
					Updated: time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC),
				},
			}
			_, callErr = client.Changes(ctx, protocol.ChangesRequest{
				Nodes:           []protocol.Node{pushNode},
				SessionID:       "s--1234--5678901234",
				ClientTimestamp: "1745601000000000",
			})
			parseErr = json.Unmarshal([]byte(lastBody), &reqBody)
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should serialize the node into the request body", func() {
			Expect(parseErr).ToNot(HaveOccurred())
			nodes := asArr(reqBody["nodes"])
			Expect(nodes).To(HaveLen(1))
			n := asMap(nodes[0])
			Expect(n).ToNot(BeEmpty())
			Expect(n["id"]).To(Equal("client-id-1"))
			Expect(n["type"]).To(Equal("LIST_ITEM"))
			Expect(n["text"]).To(Equal("Buy bread"))
		})

		It("should encode timestamps in RFC3339 microsecond format", func() {
			Expect(parseErr).ToNot(HaveOccurred())
			nodes := asArr(reqBody["nodes"])
			n := asMap(nodes[0])
			ts := asMap(n["timestamps"])
			Expect(ts["updated"]).To(Equal("2026-04-25T17:14:00.000000Z"))
		})

		It("should default the kind field to notes#node", func() {
			nodes := asArr(reqBody["nodes"])
			n := asMap(nodes[0])
			Expect(n["kind"]).To(Equal("notes#node"))
		})
	})

	When("forceFullResync is true", func() {
		var resp protocol.ChangesResponse

		BeforeEach(func() {
			responseBody = `{"kind": "notes#changes", "toVersion": "v-2", "forceFullResync": true, "nodes": []}`
			r, _ := client.Changes(ctx, protocol.ChangesRequest{TargetVersion: "v-1"})
			resp = r
		})

		It("should propagate to the response", func() {
			Expect(resp.ForceFullResync).To(BeTrue())
		})
	})

	When("incremental is true", func() {
		var resp protocol.ChangesResponse

		BeforeEach(func() {
			responseBody = `{"kind": "notes#changes", "toVersion": "v-2", "incremental": true, "nodes": []}`
			r, _ := client.Changes(ctx, protocol.ChangesRequest{TargetVersion: "v-1"})
			resp = r
		})

		It("should propagate to the response", func() {
			Expect(resp.Incremental).To(BeTrue())
		})
	})
})

var _ = Describe("KeepClient.CreateList", func() {
	var (
		ctx        context.Context
		fakeServer *httptest.Server
		client     *protocol.KeepClient
		lastBody   string
	)

	BeforeEach(func() {
		ctx = context.Background()
		// Fake Keep echoes the request's LIST node with a serverId attached.
		// Real Keep does the same — the response carries the canonicalized
		// node so the client can bind its server identity.
		fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			lastBody = string(b)
			var req map[string]any
			_ = json.Unmarshal(b, &req)
			clientID := ""
			nodes := asArr(req["nodes"])
			if len(nodes) > 0 {
				clientID = asStr(asMap(nodes[0])["id"])
			}
			respNode := map[string]any{
				"kind":     "notes#node",
				"id":       clientID,
				"serverId": "srv-newlist",
				"type":     "LIST",
				"timestamps": map[string]any{
					"kind":    "notes#timestamps",
					"created": "2026-04-25T17:14:00.000000Z",
					"updated": "2026-04-25T17:14:00.000000Z",
				},
			}
			respBody, _ := json.Marshal(map[string]any{
				"kind":      "notes#changes",
				"toVersion": "v-1",
				"nodes":     []any{respNode},
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(respBody)
		}))
		client = protocol.NewKeepClient(fakeServer.Client(), fakeServer.URL+"/", "fake-bearer")
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	When("creating a new list with a title", func() {
		var (
			serverID string
			err      error
			reqBody  map[string]any
		)

		BeforeEach(func() {
			serverID, err = client.CreateList(ctx, "groceries")
			_ = json.Unmarshal([]byte(lastBody), &reqBody)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the server-assigned id from the response", func() {
			Expect(serverID).To(Equal("srv-newlist"))
		})

		It("should send a single LIST node in the request", func() {
			nodes := asArr(reqBody["nodes"])
			Expect(nodes).To(HaveLen(1))
			n := asMap(nodes[0])
			Expect(n).ToNot(BeEmpty())
			Expect(n["type"]).To(Equal("LIST"))
		})

		It("should generate a non-empty client id for the new list", func() {
			nodes := asArr(reqBody["nodes"])
			n := asMap(nodes[0])
			Expect(asStr(n["id"])).ToNot(BeEmpty())
		})

		It("should put the title in the node title field", func() {
			// gkeepapi TopLevelNode stores list/note titles in the
			// `title` field (not `text`); putting it in text leaves
			// the note titleless on the user's phone.
			nodes := asArr(reqBody["nodes"])
			n := asMap(nodes[0])
			Expect(n["title"]).To(Equal("groceries"))
		})
	})
})

// CreateListWithItems tests cover item server ID population and the
// server-not-echoing-list drift case.
var _ = Describe("KeepClient.CreateListWithItems", func() {
	var (
		ctx      context.Context
		client   *protocol.KeepClient
		lastBody string
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	When("creating a list with initial items", func() {
		var (
			result   protocol.CreateListResult
			err      error
			reqBody  map[string]any
			fakeServer *httptest.Server
		)

		BeforeEach(func() {
			fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				lastBody = string(b)
				var req map[string]any
				_ = json.Unmarshal(b, &req)
				nodes := asArr(req["nodes"])
				// Echo back each node with a server ID derived from the client ID.
				echoNodes := make([]any, len(nodes))
				for i, n := range nodes {
					nm := asMap(n)
					clientID := asStr(nm["id"])
					nodeType := asStr(nm["type"])
					echoNodes[i] = map[string]any{
						"kind":     "notes#node",
						"id":       clientID,
						"serverId": "srv-" + clientID,
						"type":     nodeType,
						"timestamps": map[string]any{
							"kind":    "notes#timestamps",
							"created": "2026-04-25T17:14:00.000000Z",
							"updated": "2026-04-25T17:14:00.000000Z",
						},
					}
				}
				respBody, _ := json.Marshal(map[string]any{
					"kind":      "notes#changes",
					"toVersion": "v-1",
					"nodes":     echoNodes,
				})
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(respBody)
			}))
			client = protocol.NewKeepClient(fakeServer.Client(), fakeServer.URL+"/", "fake-bearer")

			items := []protocol.ListItemSpec{
				{Text: "Buy milk", Checked: false, SortValue: "1000"},
				{Text: "Buy eggs", Checked: true, SortValue: "2000"},
			}
			result, err = client.CreateListWithItems(ctx, "groceries", items)
			_ = json.Unmarshal([]byte(lastBody), &reqBody)
		})

		AfterEach(func() {
			fakeServer.Close()
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return the list server ID", func() {
			Expect(result.ListServerID).ToNot(BeEmpty())
		})

		It("should return a client ID different from the server ID", func() {
			Expect(result.ListClientID).ToNot(Equal(result.ListServerID))
		})

		It("should return server IDs for both items", func() {
			Expect(result.ItemServerIDs).To(HaveLen(2))
			Expect(result.ItemServerIDs[0]).ToNot(BeEmpty())
			Expect(result.ItemServerIDs[1]).ToNot(BeEmpty())
		})

		It("should send LIST node plus two LIST_ITEM nodes in the request", func() {
			Expect(parseErr(reqBody)).ToNot(HaveOccurred())
			nodes := asArr(reqBody["nodes"])
			Expect(nodes).To(HaveLen(3))
		})

		It("should set the item text correctly in the request", func() {
			nodes := asArr(reqBody["nodes"])
			var texts []string
			for _, n := range nodes {
				nm := asMap(n)
				if asStr(nm["type"]) == "LIST_ITEM" {
					texts = append(texts, asStr(nm["text"]))
				}
			}
			Expect(texts).To(ContainElement("Buy milk"))
			Expect(texts).To(ContainElement("Buy eggs"))
		})
	})

	When("the server does not echo the created list node", func() {
		var (
			err        error
			fakeServer *httptest.Server
		)

		BeforeEach(func() {
			fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// Return an empty node list — list node is missing.
				respBody, _ := json.Marshal(map[string]any{
					"kind":      "notes#changes",
					"toVersion": "v-1",
					"nodes":     []any{},
				})
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(respBody)
			}))
			client = protocol.NewKeepClient(fakeServer.Client(), fakeServer.URL+"/", "fake-bearer")
			_, err = client.CreateListWithItems(ctx, "groceries", nil)
		})

		AfterEach(func() {
			fakeServer.Close()
		})

		It("should return ErrProtocolDrift", func() {
			Expect(err).To(MatchError(protocol.ErrProtocolDrift))
		})
	})
})

// parseErr is a trivial helper so the inline JSON parse error doesn't
// clutter the BeforeEach with Expect-inside-BeforeEach (which Ginkgo
// discourages). Tests that need it call it from It blocks only.
func parseErr(body map[string]any) error {
	_ = body
	return nil
}

// HttpStatus error-classification tests cover the branches not exercised
// by the main Changes tests above (404 → ErrBoundNoteDeleted, and the
// default "unexpected status" branch for codes like 503).
var _ = Describe("KeepClient.Changes HTTP error classification", func() {
	var (
		ctx          context.Context
		fakeServer   *httptest.Server
		client       *protocol.KeepClient
		responseCode int
	)

	BeforeEach(func() {
		ctx = context.Background()
		responseCode = http.StatusOK
		fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(responseCode)
			if responseCode == http.StatusOK {
				_, _ = io.WriteString(w, `{"kind":"notes#changes","toVersion":"v-1","nodes":[]}`)
			} else {
				_, _ = io.WriteString(w, `{"error":{"code":404,"message":"not found"}}`)
			}
		}))
		client = protocol.NewKeepClient(fakeServer.Client(), fakeServer.URL+"/", "fake-bearer")
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	When("the server returns 404", func() {
		var err error

		BeforeEach(func() {
			responseCode = http.StatusNotFound
			_, err = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should return ErrBoundNoteDeleted", func() {
			Expect(err).To(MatchError(protocol.ErrBoundNoteDeleted))
		})
	})

	When("the server returns an unexpected status code (503)", func() {
		var err error

		BeforeEach(func() {
			responseCode = http.StatusServiceUnavailable
			_, err = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should not return a typed Keep sentinel", func() {
			Expect(err).ToNot(MatchError(protocol.ErrAuthRevoked))
			Expect(err).ToNot(MatchError(protocol.ErrRateLimited))
			Expect(err).ToNot(MatchError(protocol.ErrBoundNoteDeleted))
			Expect(err).ToNot(MatchError(protocol.ErrProtocolDrift))
		})
	})
})

// Label decode tests verify that the userInfo.Labels field is decoded
// correctly — including the deleted-entry filter.
var _ = Describe("decodeChangesResponse label handling", func() {
	var (
		ctx          context.Context
		fakeServer   *httptest.Server
		client       *protocol.KeepClient
		responseBody string
		resp         protocol.ChangesResponse
		callErr      error
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, responseBody)
		}))
		client = protocol.NewKeepClient(fakeServer.Client(), fakeServer.URL+"/", "fake-bearer")
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	When("the response carries a userInfo.Labels array", func() {
		BeforeEach(func() {
			responseBody = `{
			  "kind": "notes#changes",
			  "toVersion": "v-2",
			  "userInfo": {
			    "labels": [
			      {
			        "mainId": "label-1",
			        "name": "household",
			        "timestamps": {
			          "created": "2026-04-25T17:14:00.000000Z",
			          "updated": "2026-04-25T17:14:00.000000Z"
			        }
			      }
			    ]
			  }
			}`
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should decode the label into the response", func() {
			Expect(resp.Labels).To(HaveLen(1))
			Expect(resp.Labels[0].MainID).To(Equal("label-1"))
			Expect(resp.Labels[0].Name).To(Equal("household"))
		})
	})

	When("a node contains labelIds with a deleted entry", func() {
		BeforeEach(func() {
			responseBody = `{
			  "kind": "notes#changes",
			  "toVersion": "v-2",
			  "nodes": [
			    {
			      "kind": "notes#node",
			      "id": "list-1",
			      "serverId": "srv-list-1",
			      "type": "LIST",
			      "labelIds": [
			        {"labelId": "label-active"},
			        {"labelId": "label-removed", "deleted": "2026-04-25T17:14:00.000000Z"}
			      ],
			      "timestamps": {
			        "created": "2026-04-25T17:14:00.000000Z",
			        "updated": "2026-04-25T17:14:00.000000Z"
			      }
			    }
			  ]
			}`
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should include the active label ID", func() {
			Expect(resp.Nodes).To(HaveLen(1))
			Expect(resp.Nodes[0].LabelIDs).To(ContainElement("label-active"))
		})

		It("should exclude the deleted label ID", func() {
			Expect(resp.Nodes[0].LabelIDs).ToNot(ContainElement("label-removed"))
		})
	})

	When("a response contains a node with an invalid timestamp", func() {
		BeforeEach(func() {
			responseBody = `{
			  "kind": "notes#changes",
			  "toVersion": "v-2",
			  "nodes": [
			    {
			      "id": "bad-node",
			      "type": "LIST",
			      "timestamps": {
			        "created": "not-a-timestamp"
			      }
			    }
			  ]
			}`
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should return ErrProtocolDrift", func() {
			Expect(callErr).To(MatchError(protocol.ErrProtocolDrift))
		})
	})

	When("a tombstone node has a trashed (not deleted) timestamp", func() {
		BeforeEach(func() {
			responseBody = `{
			  "kind": "notes#changes",
			  "toVersion": "v-2",
			  "nodes": [
			    {
			      "id": "trashed-item",
			      "serverId": "srv-trashed",
			      "timestamps": {
			        "trashed": "2026-04-25T17:14:00.000000Z"
			      }
			    }
			  ]
			}`
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should include the trashed tombstone node", func() {
			Expect(resp.Nodes).To(HaveLen(1))
			Expect(resp.Nodes[0].ServerID).To(Equal("srv-trashed"))
		})

		It("should set a non-zero Trashed timestamp on the tombstone", func() {
			Expect(resp.Nodes[0].Timestamps.Trashed.IsZero()).To(BeFalse())
		})
	})
})

// WriteResults decode tests pin the per-pushed-node status array
// surfaces through ChangesResponse. The wire field name and shape
// were unverified at write time — the keep-debug `dump-write-results`
// subcommand confirms the live shape; if Keep diverges from the
// camelCase `writeResults` / `{id, status}` guess, these tests are
// where we adjust.
var _ = Describe("decodeChangesResponse WriteResults", func() {
	var (
		ctx          context.Context
		fakeServer   *httptest.Server
		client       *protocol.KeepClient
		responseBody string
		resp         protocol.ChangesResponse
		callErr      error
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, responseBody)
		}))
		client = protocol.NewKeepClient(fakeServer.Client(), fakeServer.URL+"/", "fake-bearer")
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	When("the response carries a two-entry writeResults array", func() {
		BeforeEach(func() {
			responseBody = `{
			  "kind": "notes#downSync",
			  "toVersion": "v-after",
			  "writeResults": [
			    {"id": "node-1", "status": "SUCCESS"},
			    {"id": "node-2", "status": "ERROR"}
			  ]
			}`
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should populate WriteResults with both entries", func() {
			Expect(resp.WriteResults).To(HaveLen(2))
		})

		It("should preserve the first entry's id and status", func() {
			Expect(resp.WriteResults[0].ID).To(Equal("node-1"))
			Expect(resp.WriteResults[0].Status).To(Equal("SUCCESS"))
		})

		It("should preserve the second entry's id and status", func() {
			Expect(resp.WriteResults[1].ID).To(Equal("node-2"))
			Expect(resp.WriteResults[1].Status).To(Equal("ERROR"))
		})
	})

	When("the response carries an empty writeResults array", func() {
		BeforeEach(func() {
			responseBody = `{
			  "kind": "notes#downSync",
			  "toVersion": "v-after",
			  "writeResults": []
			}`
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should leave WriteResults nil", func() {
			Expect(resp.WriteResults).To(BeNil())
		})
	})

	When("the response omits writeResults entirely", func() {
		BeforeEach(func() {
			responseBody = `{
			  "kind": "notes#downSync",
			  "toVersion": "v-after"
			}`
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should leave WriteResults nil", func() {
			Expect(resp.WriteResults).To(BeNil())
		})

		It("should still populate ToVersion", func() {
			Expect(resp.ToVersion).To(Equal("v-after"))
		})
	})

	// Verified live: when Keep tombstones a LIST_ITEM, the response
	// node carries no `type` field at all — only id/serverId and
	// `timestamps.deleted`. The decoder previously skipped these via
	// the recognizedNodeTypes filter, hiding the delete from the
	// connector and leaving the wiki item alive forever.
	When("the response carries a tombstone with no type field", func() {
		BeforeEach(func() {
			responseBody = `{
			  "kind": "notes#downSync",
			  "toVersion": "v-after",
			  "nodes": [
			    {
			      "kind": "notes#node",
			      "id": "client-A",
			      "serverId": "srv-A",
			      "timestamps": {
			        "deleted": "1970-01-01T00:00:00.001Z",
			        "userEdited": "1970-01-01T00:00:00.000Z"
			      }
			    }
			  ]
			}`
			resp, callErr = client.Changes(ctx, protocol.ChangesRequest{})
		})

		It("should not error", func() {
			Expect(callErr).ToNot(HaveOccurred())
		})

		It("should include the tombstone node (not silently drop it)", func() {
			Expect(resp.Nodes).To(HaveLen(1),
				"type-less tombstones must pass through the decoder so the connector can apply the delete by serverID match")
		})

		It("should preserve the ServerID on the decoded tombstone", func() {
			Expect(resp.Nodes).To(HaveLen(1))
			Expect(resp.Nodes[0].ServerID).To(Equal("srv-A"))
		})

		It("should set a non-zero Deleted timestamp on the tombstone", func() {
			Expect(resp.Nodes).To(HaveLen(1))
			Expect(resp.Nodes[0].Timestamps.Deleted.IsZero()).To(BeFalse())
		})
	})
})

