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

		It("should send the GoogleLogin auth header", func() {
			Expect(lastRequest.Header.Get("Authorization")).To(Equal("GoogleLogin auth=fake-bearer"))
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

		It("should put the title in the node text", func() {
			nodes := asArr(reqBody["nodes"])
			n := asMap(nodes[0])
			Expect(n["text"]).To(Equal("groceries"))
		})
	})
})

