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
			rh, ok := reqBody["requestHeader"].(map[string]any)
			Expect(ok).To(BeTrue())
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

