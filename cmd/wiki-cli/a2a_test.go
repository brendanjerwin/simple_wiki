package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// a2aMapAny asserts v to map[string]any with a Gomega failure instead of a
// panic when the shape is wrong.
func a2aMapAny(v any) map[string]any {
	m, ok := v.(map[string]any)
	Expect(ok).To(BeTrue(), "expected object, got %v", v)
	return m
}

// a2aMap asserts m["key"] to map[string]any with a Gomega failure instead of
// a panic when the shape is wrong.
func a2aMap(m map[string]any, key string) map[string]any {
	v, ok := m[key].(map[string]any)
	Expect(ok).To(BeTrue(), "expected %q to be an object in %v", key, m)
	return v
}

// a2aList asserts m["key"] to []any with a Gomega failure instead of a panic.
func a2aList(m map[string]any, key string) []any {
	v, ok := m[key].([]any)
	Expect(ok).To(BeTrue(), "expected %q to be a list in %v", key, m)
	return v
}

// a2aStr asserts v to string with a Gomega failure instead of a panic.
func a2aStr(v any) string {
	s, ok := v.(string)
	Expect(ok).To(BeTrue(), "expected string, got %v", v)
	return s
}

// newA2ATestServer builds an a2aServer with sane test config and serves it on
// an httptest.Server. taskRunner, when non-nil, is the injected runner seam.
func newA2ATestServer(bearerToken, proxySecret string, taskTimeout time.Duration, maxTasks int, runner taskRunner) (*a2aServer, *httptest.Server) {
	srv, err := newA2AServer(a2aServerConfig{
		Port:        1, // unused; httptest supplies the listener
		Bind:        "127.0.0.1",
		PublicURL:   "https://mcp.example.net/agent",
		BearerToken: bearerToken,
		ProxySecret: proxySecret,
		TaskTimeout: taskTimeout,
		MaxTasks:    maxTasks,
	}, &poolDaemon{})
	Expect(err).NotTo(HaveOccurred())
	srv.taskRunner = runner
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Emulate the Go 1.22 mux routing used in serve().
		mux := http.NewServeMux()
		mux.HandleFunc("GET /.well-known/agent-card.json", srv.handleCard)
		mux.HandleFunc("POST /{$}", srv.handleDispatch)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
		mux.ServeHTTP(w, r)
	}))
	return srv, ts
}

// dispatchA2A posts a JSON-RPC request and decodes the envelope.
func dispatchA2A(ts *httptest.Server, headers map[string]string, body any) (statusCode int, decoded map[string]any) {
	payload, err := json.Marshal(body)
	Expect(err).NotTo(HaveOccurred())
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/", strings.NewReader(string(payload)))
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := ts.Client().Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close() //nolint:errcheck
	err = json.NewDecoder(resp.Body).Decode(&decoded)
	Expect(err).NotTo(HaveOccurred())
	return resp.StatusCode, decoded
}

func bearerHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// dispatchNotification posts a JSON-RPC notification and returns the HTTP
// status code and raw body bytes. Per spec, notifications must not produce
// a JSON-RPC response body, so callers should assert the body is empty.
func dispatchNotification(ts *httptest.Server, headers map[string]string, body any) (statusCode int, responseBody []byte) {
	payload, err := json.Marshal(body)
	Expect(err).NotTo(HaveOccurred())
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/", strings.NewReader(string(payload)))
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := ts.Client().Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close() //nolint:errcheck
	responseBody, err = io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	return resp.StatusCode, responseBody
}

// pollTaskUntil polls tasks/get until the task leaves non-terminal state or
// the deadline passes; returns (state, finalText).
func pollTaskUntil(ts *httptest.Server, token, taskID string, timeout time.Duration) (finalState string, resultText string) {
	deadline := time.Now().Add(timeout)
	for {
		_, resp := dispatchA2A(ts, bearerHeaders(token), map[string]any{
			"jsonrpc": "2.0", "id": 7, "method": "tasks/get", "params": map[string]any{"id": taskID},
		})
		result, _ := resp["result"].(map[string]any)
		if result != nil {
			state, _ := a2aMap(result, "status")["state"].(string)
			if state != "working" {
				text := ""
				if artifacts, ok := result["artifacts"].([]any); ok && len(artifacts) > 0 {
					art := a2aMapAny(artifacts[0])
					if parts, ok := art["parts"].([]any); ok && len(parts) > 0 {
						text = a2aStr(a2aMapAny(parts[0])["text"])
					}
				}
				return state, text
			}
		}
		if time.Now().After(deadline) {
			return "timeout-waiting", ""
		}
		time.Sleep(10 * time.Millisecond)
	}
}

var _ = Describe("a2aServer agent card", func() {
	var ts *httptest.Server

	BeforeEach(func() {
		_, ts = newA2ATestServer("tok", "", 9*time.Minute, 128, nil)
	})
	AfterEach(func() {
		ts.Close()
	})

	It("serves the card without auth", func() {
		resp, err := ts.Client().Get(ts.URL + "/.well-known/agent-card.json")
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close() //nolint:errcheck
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		var card map[string]any
		Expect(json.NewDecoder(resp.Body).Decode(&card)).To(Succeed())
		Expect(card["name"]).To(Equal("wiki-chat-agent"))
		Expect(card["url"]).To(Equal("https://mcp.example.net/agent"))
		Expect(card["version"]).NotTo(BeEmpty())
		capabilities := a2aMap(card, "capabilities")
		Expect(capabilities["streaming"]).To(BeFalse())
	})

	It("404s unknown paths", func() {
		resp, err := ts.Client().Get(ts.URL + "/nope")
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close() //nolint:errcheck
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})
})

var _ = Describe("a2aServer auth and caller identity", func() {
	var (
		srv *a2aServer
		ts  *httptest.Server
	)
	const (
		tok   = "tok-123"
		proxy = "proxy-456"
	)

	BeforeEach(func() {
		srv, ts = newA2ATestServer(tok, proxy, 9*time.Minute, 128, nil)
	})
	AfterEach(func() {
		ts.Close()
	})

	It("rejects uncredentialed dispatch (fail-closed, same as MCP routes)", func() {
		code, resp := dispatchA2A(ts, nil, map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "tasks/list", "params": map[string]any{},
		})
		Expect(code).To(Equal(http.StatusUnauthorized))
		Expect(resp["error"]).To(Equal("unauthorized"))
	})

	It("rejects a wrong bearer token", func() {
		code, _ := dispatchA2A(ts, bearerHeaders("wrong"), map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "tasks/list", "params": map[string]any{},
		})
		Expect(code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a wrong proxy secret (fail closed)", func() {
		code, resp := dispatchA2A(ts, map[string]string{trustedProxySecretHeader: "wrong"}, map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "tasks/list", "params": map[string]any{},
		})
		Expect(code).To(Equal(http.StatusUnauthorized))
		Expect(resp["error"]).To(Equal("unauthorized"))
	})

	It("accepts the bearer and identifies a pi-agent caller", func() {
		var seen a2aCaller
		var mu sync.Mutex
		srv.taskRunner = func(_ context.Context, _ *a2aTask, caller a2aCaller) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			seen = caller
			return "ok", nil
		}
		code, resp := dispatchA2A(ts, bearerHeaders(tok), a2aSendBody("hello there"))
		Expect(code).To(Equal(http.StatusOK))
		taskID := a2aStr(a2aMap(resp, "result")["id"])
		state, text := pollTaskUntil(ts, tok, taskID, 2*time.Second)
		Expect(state).To(Equal("completed"))
		Expect(text).To(Equal("ok"))
		mu.Lock()
		defer mu.Unlock()
		Expect(seen.ClientName).To(Equal("pi-agent"))
		Expect(seen.describe()).To(ContainSubstring("shared tailnet bearer"))
	})

	It("resolves the gateway caller from proxy-secret headers", func() {
		var seen a2aCaller
		var mu sync.Mutex
		srv.taskRunner = func(_ context.Context, _ *a2aTask, caller a2aCaller) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			seen = caller
			return "ok", nil
		}
		headers := map[string]string{
			trustedProxySecretHeader: proxy,
			"Tailscale-User-Login":   "x@y",
			"X-Gateway-Client-ID":    "gwmcp-1",
			"X-Gateway-Client-Name":  "Mars",
			"Authorization":          "Bearer whatever",
			"X-Forwarded-User":       "spoof",
		}
		code, resp := dispatchA2A(ts, headers, a2aSendBody("hello"))
		Expect(code).To(Equal(http.StatusOK))
		taskID := a2aStr(a2aMap(resp, "result")["id"])
		state, _ := pollTaskUntil(ts, tok, taskID, 2*time.Second)
		Expect(state).To(Equal("completed"))
		mu.Lock()
		defer mu.Unlock()
		Expect(seen.Login).To(Equal("x@y"))
		Expect(seen.ClientID).To(Equal("gwmcp-1"))
		Expect(seen.ClientName).To(Equal("Mars"))
		Expect(seen.describe()).To(Equal("Mars (gwmcp-1) authorized by x@y"))
	})

	It("rejects a non-Bearer Authorization header", func() {
		code, _ := dispatchA2A(ts, map[string]string{"Authorization": "Basic tok-123"}, map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "tasks/list", "params": map[string]any{},
		})
		Expect(code).To(Equal(http.StatusUnauthorized))
	})
})

// a2aSendBody builds a message/send request body.
func a2aSendBody(text string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "message/send",
		"params": map[string]any{
			"message": map[string]any{
				"role":      "user",
				"parts":     []map[string]any{{"kind": "text", "text": text}},
				"messageId": "m-1",
				"contextId": "ctx-abc",
			},
			"configuration": map[string]any{"blocking": false},
		},
	}
}

var _ = Describe("a2aServer message/send dispatch", func() {
	var ts *httptest.Server
	const tok = "tok-123"

	BeforeEach(func() {
		_, ts = newA2ATestServer(tok, "", 9*time.Minute, 128, nil)
	})
	AfterEach(func() {
		ts.Close()
	})

	It("creates a working task and echoes contextId", func() {
		code, resp := dispatchA2A(ts, bearerHeaders(tok), a2aSendBody("do a thing"))
		Expect(code).To(Equal(http.StatusOK))
		result := a2aMap(resp, "result")
		Expect(result["id"].(string)).NotTo(BeEmpty())
		Expect(result["id"].(string)).To(HavePrefix("a2a-"))
		status := a2aMap(result, "status")
		Expect(status["state"]).To(Equal("working"))
		Expect(result["contextId"]).To(Equal("ctx-abc"))
	})

	It("generates a contextId when the client omits it", func() {
		body := a2aSendBody("no ctx")
		body["params"].(map[string]any)["message"].(map[string]any)["contextId"] = ""
		code, resp := dispatchA2A(ts, bearerHeaders(tok), body)
		Expect(code).To(Equal(http.StatusOK))
		result := a2aMap(resp, "result")
		Expect(result["contextId"].(string)).To(HavePrefix("ctx-"))
	})

	It("rejects empty text with -32602", func() {
		body := a2aSendBody("")
		code, resp := dispatchA2A(ts, bearerHeaders(tok), body)
		Expect(code).To(Equal(http.StatusOK))
		errObj := a2aMap(resp, "error")
		Expect(errObj["code"]).To(Equal(float64(a2aErrInvalidParams)))
	})

	It("rejects missing params.message with -32602", func() {
		code, resp := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "message/send", "params": map[string]any{},
		})
		Expect(code).To(Equal(http.StatusOK))
		errObj := a2aMap(resp, "error")
		Expect(errObj["code"]).To(Equal(float64(a2aErrInvalidParams)))
	})

	It("rejects unknown methods with -32601", func() {
		code, resp := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "foo/bar", "params": map[string]any{},
		})
		Expect(code).To(Equal(http.StatusOK))
		errObj := a2aMap(resp, "error")
		Expect(errObj["code"]).To(Equal(float64(a2aErrMethodNotFound)))
	})

	It("rejects array bodies with -32600", func() {
		payload := `[{"jsonrpc":"2.0","id":1,"method":"tasks/list"}]`
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/", strings.NewReader(payload))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := ts.Client().Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close() //nolint:errcheck
		var decoded map[string]any
		Expect(json.NewDecoder(resp.Body).Decode(&decoded)).To(Succeed())
		errObj := a2aMap(decoded, "error")
		Expect(errObj["code"]).To(Equal(float64(a2aErrInvalidRequest)))
	})

	It("rejects invalid JSON with -32700", func() {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/", strings.NewReader("{not json"))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := ts.Client().Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close() //nolint:errcheck
		var decoded map[string]any
		Expect(json.NewDecoder(resp.Body).Decode(&decoded)).To(Succeed())
		errObj := a2aMap(decoded, "error")
		Expect(errObj["code"]).To(Equal(float64(a2aErrParse)))
	})
})

var _ = Describe("a2aServer task execution via taskRunner seam", func() {
	var ts *httptest.Server
	const tok = "tok-123"

	AfterEach(func() {
		ts.Close()
	})

	It("completes with the runner text in artifacts and status.message", func() {
		_, ts = newA2ATestServer(tok, "", 9*time.Minute, 128, func(_ context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			return "FINAL-ANSWER", nil
		})
		code, resp := dispatchA2A(ts, bearerHeaders(tok), a2aSendBody("run it"))
		Expect(code).To(Equal(http.StatusOK))
		taskID := a2aStr(a2aMap(resp, "result")["id"])
		state, text := pollTaskUntil(ts, tok, taskID, 2*time.Second)
		Expect(state).To(Equal("completed"))
		Expect(text).To(Equal("FINAL-ANSWER"))

		_, resp2 := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
			"jsonrpc": "2.0", "id": 9, "method": "tasks/get", "params": map[string]any{"id": taskID},
		})
		result := a2aMap(resp2, "result")
		artifacts := a2aList(result, "artifacts")
		art := a2aMapAny(artifacts[0])
		Expect(a2aMapAny(a2aList(art, "parts")[0])["text"]).To(Equal("FINAL-ANSWER"))
		status := a2aMap(result, "status")
		msg := a2aMap(status, "message")
		Expect(msg["role"]).To(Equal("agent"))
		Expect(a2aMapAny(a2aList(msg, "parts")[0])["text"]).To(Equal("FINAL-ANSWER"))
		Expect(msg["taskId"]).To(Equal(taskID))
		Expect(a2aList(result, "history")).To(HaveLen(2))
	})

	It("fails the task when the runner errors", func() {
		_, ts = newA2ATestServer(tok, "", 9*time.Minute, 128, func(_ context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			return "", errors.New("boom")
		})
		_, resp := dispatchA2A(ts, bearerHeaders(tok), a2aSendBody("run it"))
		taskID := a2aStr(a2aMap(resp, "result")["id"])
		state, text := pollTaskUntil(ts, tok, taskID, 2*time.Second)
		Expect(state).To(Equal("failed"))
		Expect(text).To(ContainSubstring("prompt failed: boom"))
	})

	It("fails with a deadline message when the runner blocks past TaskTimeout", func() {
		_, ts = newA2ATestServer(tok, "", 50*time.Millisecond, 128, func(ctx context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		})
		_, resp := dispatchA2A(ts, bearerHeaders(tok), a2aSendBody("run it"))
		taskID := a2aStr(a2aMap(resp, "result")["id"])
		state, text := pollTaskUntil(ts, tok, taskID, 2*time.Second)
		Expect(state).To(Equal("failed"))
		Expect(text).To(ContainSubstring("task deadline exceeded"))
	})
})

var _ = Describe("a2aServer tasks/cancel and tasks/list", func() {
	var (
		ts      *httptest.Server
		release chan struct{}
	)
	const tok = "tok-123"

	BeforeEach(func() {
		release = make(chan struct{})
		rel := release // capture; runner goroutines outlive AfterEach
		_, ts = newA2ATestServer(tok, "", 9*time.Minute, 128, func(ctx context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			select {
			case <-rel:
				return "done", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		})
	})
	AfterEach(func() {
		ts.Close()
	})

	It("cancels a running task", func() {
		_, resp := dispatchA2A(ts, bearerHeaders(tok), a2aSendBody("long task"))
		taskID := a2aStr(a2aMap(resp, "result")["id"])

		// Wait until the runner is actually blocked inside the seam.
		Eventually(func() bool {
			_, r := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
				"jsonrpc": "2.0", "id": 3, "method": "tasks/get", "params": map[string]any{"id": taskID},
			})
			result, _ := r["result"].(map[string]any)
			if result == nil {
				return false
			}
			state, _ := a2aMap(result, "status")["state"].(string)
			return state == "working"
		}).Within(2 * time.Second).Should(BeTrue())

		ccode, cresp := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
			"jsonrpc": "2.0", "id": 4, "method": "tasks/cancel", "params": map[string]any{"id": taskID},
		})
		Expect(ccode).To(Equal(http.StatusOK))
		result := a2aMap(cresp, "result")
		Expect(result["id"]).To(Equal(taskID))
		Expect(result["status"].(map[string]any)["state"]).To(Equal("canceled"))

		// Cancel of an already-terminal task returns it unchanged.
		ccode2, cresp2 := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
			"jsonrpc": "2.0", "id": 5, "method": "tasks/cancel", "params": map[string]any{"id": taskID},
		})
		Expect(ccode2).To(Equal(http.StatusOK))
		Expect(cresp2["result"].(map[string]any)["status"].(map[string]any)["state"]).To(Equal("canceled"))
	})

	It("returns tasks/list with contextId filtering", func() {
		dispatchA2A(ts, bearerHeaders(tok), a2aSendBody("task one")) // ctx-abc
		body := a2aSendBody("task two")
		body["params"].(map[string]any)["message"].(map[string]any)["contextId"] = "ctx-other"
		dispatchA2A(ts, bearerHeaders(tok), body)

		_, resp := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
			"jsonrpc": "2.0", "id": 6, "method": "tasks/list", "params": map[string]any{},
		})
		result := a2aMap(resp, "result")
		tasks := a2aList(result, "tasks")
		Expect(tasks).To(HaveLen(2))

		_, resp2 := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
			"jsonrpc": "2.0", "id": 6, "method": "tasks/list", "params": map[string]any{"contextId": "ctx-other"},
		})
		result2 := a2aMap(resp2, "result")
		tasks2 := a2aList(result2, "tasks")
		Expect(tasks2).To(HaveLen(1))
		Expect(tasks2[0].(map[string]any)["contextId"]).To(Equal("ctx-other"))
	})

	It("404-errors tasks/get for unknown task id", func() {
		_, resp := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
			"jsonrpc": "2.0", "id": 8, "method": "tasks/get", "params": map[string]any{"id": "a2a-nonexistent"},
		})
		errObj := a2aMap(resp, "error")
		Expect(errObj["code"]).To(Equal(float64(a2aErrTaskNotFound)))
		Expect(errObj["message"]).To(ContainSubstring("task not found: a2a-nonexistent"))
	})
})

var _ = Describe("a2aServer task store eviction", func() {
	It("evicts oldest terminal tasks first and keeps running ones", func() {
		srv, ts := newA2ATestServer("tok", "", 9*time.Minute, 3, func(_ context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			return "x", nil
		})
		defer ts.Close()

		// Fill with 3 completed tasks (sequentially so CreatedAt differs).
		var ids []string
		for i := range 3 {
			_, resp := dispatchA2A(ts, bearerHeaders("tok"), a2aSendBody(fmt.Sprintf("t%d", i)))
			ids = append(ids, resp["result"].(map[string]any)["id"].(string))
			Eventually(func() bool {
				_, r := dispatchA2A(ts, bearerHeaders("tok"), map[string]any{
					"jsonrpc": "2.0", "id": 2, "method": "tasks/get", "params": map[string]any{"id": ids[i]},
				})
				result, _ := r["result"].(map[string]any)
				if result == nil {
					return false
				}
				state, _ := a2aMap(result, "status")["state"].(string)
				return state == "completed"
			}).Within(2 * time.Second).Should(BeTrue())
		}

		// A 4th task evicts the oldest terminal (first).
		dispatchA2A(ts, bearerHeaders("tok"), a2aSendBody("t3"))
		srv.mu.Lock()
		_, oldestPresent := srv.tasks[ids[0]]
		_, secondPresent := srv.tasks[ids[1]]
		srv.mu.Unlock()
		Expect(oldestPresent).To(BeFalse())
		Expect(secondPresent).To(BeTrue())
		Expect(srv.tasksCount()).To(Equal(3))
	})

	It("never evicts non-terminal tasks", func() {
		blocked := make(chan struct{})
		srv, ts := newA2ATestServer("tok", "", 9*time.Minute, 2, func(ctx context.Context, task *a2aTask, _ a2aCaller) (string, error) {
			if task.UserText == "running" {
				<-blocked
			}
			return "x", nil
		})
		defer ts.Close()

		// Task 1: still running (never completes).
		_, resp := dispatchA2A(ts, bearerHeaders("tok"), a2aSendBody("running"))
		runningID := a2aStr(a2aMap(resp, "result")["id"])
		// Task 2: completes.
		Eventually(func() string {
			_, r := dispatchA2A(ts, bearerHeaders("tok"), a2aSendBody("done"))
			result := a2aMap(r, "result")
			id := a2aStr(result["id"])
			srv.mu.Lock()
			t := srv.tasks[id]
			srv.mu.Unlock()
			if t == nil {
				return ""
			}
			return t.State
		}).Within(2 * time.Second).Should(Equal("completed"))

		srv.mu.Lock()
		_, runningStillThere := srv.tasks[runningID]
		srv.mu.Unlock()
		Expect(runningStillThere).To(BeTrue())
		close(blocked)
	})
})

var _ = Describe("newA2AServer validation", func() {
	It("errors when port>0 and no credentials are set (fail-closed)", func() {
		_, err := newA2AServer(a2aServerConfig{Port: 8091}, &poolDaemon{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("refusing to start without credentials"))
	})

	It("errors when exactly one TLS path is set", func() {
		_, err := newA2AServer(a2aServerConfig{Port: 8091, BearerToken: "t", TLSCertPath: "/tmp/c"}, &poolDaemon{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("must be set together"))
	})

	It("accepts a valid bearer-only config", func() {
		_, err := newA2AServer(a2aServerConfig{Port: 8091, BearerToken: "t"}, &poolDaemon{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("accepts a valid proxy-secret-only config", func() {
		_, err := newA2AServer(a2aServerConfig{Port: 8091, ProxySecret: "s"}, &poolDaemon{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("honors a configured trusted-proxy header override", func() {
		srv, err := newA2AServer(a2aServerConfig{Port: 8091, BearerToken: "t", ProxyHeader: "X-Custom-Secret"}, &poolDaemon{})
		Expect(err).NotTo(HaveOccurred())
		Expect(srv.cfg.ProxyHeader).To(Equal("X-Custom-Secret"))
	})
})

var _ = Describe("a2aServer session continuation (spec §3.4.3)", func() {
	var (
		srv      *a2aServer
		spawns   int
		cleanups []string
	)

	BeforeEach(func() {
		srv, _ = newA2ATestServer("tok", "proxy", 9*time.Minute, 128, nil)
		spawns = 0
		cleanups = nil
		srv.daemon.a2aEphemeralSpawner = func(_ context.Context, _ acp.Client, _, _ string) (*ephemeralAgent, error) {
			spawns++
			n := spawns
			return &ephemeralAgent{
				sessionID: acp.SessionId(fmt.Sprintf("sess-%d", n)),
				cleanup:   func() { cleanups = append(cleanups, fmt.Sprintf("sess-%d", n)) },
			}, nil
		}
	})

	It("reuses the session for a follow-up dispatch on the same contextId", func() {
		t1 := &a2aTask{ID: "a2a-aaa", ContextID: "ctx-1", UserText: "first", Caller: a2aCaller{ClientName: a2aCallerTailnet}, State: a2aStateWorking}
		turn1, err := srv.acquireSession(context.Background(), t1)
		Expect(err).NotTo(HaveOccurred())
		Expect(turn1.firstTurn).To(BeTrue())
		Expect(turn1.agent).NotTo(BeNil())
		Expect(spawns).To(Equal(1))
		Expect(srv.sessions).To(HaveKey("ctx-1"))
		client1 := srv.sessions["ctx-1"].client

		t2 := &a2aTask{ID: "a2a-bbb", ContextID: "ctx-1", UserText: "second", Caller: t1.Caller, State: a2aStateWorking}
		turn2, err := srv.acquireSession(context.Background(), t2)
		Expect(err).NotTo(HaveOccurred())
		Expect(turn2.firstTurn).To(BeFalse(), "second dispatch must reuse the session")
		Expect(turn2.agent).To(BeIdenticalTo(turn1.agent), "same ACP connection reused")
		Expect(srv.sessions["ctx-1"].client).To(BeIdenticalTo(client1))
		Expect(spawns).To(Equal(1), "no second spawn for the same contextId")
		Expect(srv.sessions["ctx-1"].turnCount).To(Equal(2))
	})

	It("clears the reserved slot when spawn fails", func() {
		srv.daemon.a2aEphemeralSpawner = func(context.Context, acp.Client, string, string) (*ephemeralAgent, error) {
			return nil, errors.New("spawn failed in test")
		}
		t := &a2aTask{ID: "a2a-ccc", ContextID: "ctx-2", UserText: "x", Caller: a2aCaller{ClientName: a2aCallerTailnet}, State: a2aStateWorking}
		_, err := srv.acquireSession(context.Background(), t)
		Expect(err).To(HaveOccurred())
		srv.mu.Lock()
		_, present := srv.sessions["ctx-2"]
		srv.mu.Unlock()
		Expect(present).To(BeFalse(), "failed spawn must not leave a reserved slot")
	})

	It("drops a session so the next turn respawns fresh", func() {
		t := &a2aTask{ID: "a2a-ddd", ContextID: "ctx-3", UserText: "first", Caller: a2aCaller{ClientName: a2aCallerTailnet}, State: a2aStateWorking}
		_, err := srv.acquireSession(context.Background(), t)
		Expect(err).NotTo(HaveOccurred())
		Expect(srv.sessions).To(HaveKey("ctx-3"))
		Expect(spawns).To(Equal(1))

		srv.dropSession("ctx-3")
		Expect(srv.sessions).NotTo(HaveKey("ctx-3"))
		Expect(cleanups).To(ContainElement("sess-1"), "dropped session's agent must be cleaned up")

		// Next dispatch on the same contextId spawns a NEW session.
		turn, err := srv.acquireSession(context.Background(), t)
		Expect(err).NotTo(HaveOccurred())
		Expect(turn.firstTurn).To(BeTrue())
		Expect(spawns).To(Equal(2))
	})

	It("beginTurn resets the turn text collector", func() {
		t := &a2aTask{ID: "a2a-eee", ContextID: "ctx-4", UserText: "x", Caller: a2aCaller{ClientName: a2aCallerTailnet}, State: a2aStateWorking}
		_, err := srv.acquireSession(context.Background(), t)
		Expect(err).NotTo(HaveOccurred())
		client := srv.sessions["ctx-4"].client
		Expect(client).NotTo(BeNil())

		client.mu.Lock()
		client.text.WriteString("turn-one-output")
		client.mu.Unlock()
		Expect(client.finalText()).To(Equal("turn-one-output"))

		getTurn := srv.sessions["ctx-4"].beginTurn()
		// beginTurn already reset the builder under its own lock; assert
		// emptiness WITHOUT holding the lock (finalText re-locks).
		Expect(client.finalText()).To(Equal(""), "collector must be empty at turn start")
		client.mu.Lock()
		client.text.WriteString("turn-two")
		client.mu.Unlock()
		Expect(getTurn()).To(Equal("turn-two"))
	})

	It("reaper evicts idle sessions and cleans up their agents", func() {
		t := &a2aTask{ID: "a2a-fff", ContextID: "ctx-5", UserText: "x", Caller: a2aCaller{ClientName: a2aCallerTailnet}, State: a2aStateWorking}
		_, err := srv.acquireSession(context.Background(), t)
		Expect(err).NotTo(HaveOccurred())

		// Force idle: backdate lastUsed beyond the TTL.
		srv.mu.Lock()
		srv.sessions["ctx-5"].lastUsed = time.Now().Add(-a2aSessionIdleTTL - time.Minute)
		srv.mu.Unlock()

		srv.reapSessionsOnce()
		srv.mu.Lock()
		_, present := srv.sessions["ctx-5"]
		srv.mu.Unlock()
		Expect(present).To(BeFalse())
		Expect(cleanups).To(ContainElement("sess-1"), "reaped session's agent must be cleaned up")
	})

	It("reaper never evicts a spawn-in-progress session", func() {
		srv.mu.Lock()
		srv.sessions["ctx-6"] = spawningSession // sentinel
		srv.mu.Unlock()

		srv.reapSessionsOnce()
		srv.mu.Lock()
		_, present := srv.sessions["ctx-6"]
		srv.mu.Unlock()
		Expect(present).To(BeTrue(), "sentinel must survive the reaper")
	})
})

var _ = Describe("a2aServer reaper busy-session safety", func() {
	It("never tears down a busy (mid-Prompt) session's agent", func() {
		srv, _ := newA2ATestServer("tok", "proxy", 9*time.Minute, 128, nil)
		cleaned := make(chan string, 8)
		srv.daemon.a2aEphemeralSpawner = func(_ context.Context, _ acp.Client, _, _ string) (*ephemeralAgent, error) {
			return &ephemeralAgent{
				sessionID: acp.SessionId("sess-busy"),
				cleanup:   func() { cleaned <- "sess-busy" },
			}, nil
		}

		t := &a2aTask{ID: "a2a-ggg", ContextID: "ctx-busy", UserText: "x", Caller: a2aCaller{ClientName: a2aCallerTailnet}, State: a2aStateWorking}
		_, err := srv.acquireSession(context.Background(), t)
		Expect(err).NotTo(HaveOccurred())

		// Force eviction candidacy and make the session busy (mid-Prompt).
		srv.mu.Lock()
		sess := srv.sessions["ctx-busy"]
		sess.lastUsed = time.Now().Add(-a2aSessionIdleTTL - time.Minute)
		srv.mu.Unlock()
		sess.promptMu.Lock()

		srv.reapSessionsOnce()

		// The session must still be registered and NOT cleaned up.
		srv.mu.Lock()
		_, present := srv.sessions["ctx-busy"]
		srv.mu.Unlock()
		Expect(present).To(BeTrue(), "busy session must stay in the map")
		Consistently(func() bool { return len(cleaned) == 0 }, 100*time.Millisecond, 10*time.Millisecond).
			Should(BeTrue(), "busy session's agent must not be torn down")

		// Release the prompt; the NEXT reaper tick evicts it.
		sess.promptMu.Unlock()
		srv.reapSessionsOnce()
		srv.mu.Lock()
		_, present = srv.sessions["ctx-busy"]
		srv.mu.Unlock()
		Expect(present).To(BeFalse())
		Eventually(func() int { return len(cleaned) }, 1*time.Second, 10*time.Millisecond).Should(Equal(1))
	})
})

var _ = Describe("a2aServer task persistence", func() {
	var stateFile string

	BeforeEach(func() {
		stateFile = filepath.Join(os.TempDir(), fmt.Sprintf("a2a-state-%d.json", time.Now().UnixNano()))
	})
	AfterEach(func() {
		_ = os.Remove(stateFile)
	})

	newPersistTestServer := func() *a2aServer {
		srv, err := newA2AServer(a2aServerConfig{
			Port:        1,
			Bind:        "127.0.0.1",
			PublicURL:   "https://mcp.example.net/agent",
			BearerToken: "tok",
			StatePath:   stateFile,
		}, &poolDaemon{a2aEphemeralSpawner: func(context.Context, acp.Client, string, string) (*ephemeralAgent, error) {
			return nil, errors.New("no spawn in this test")
		}})
		Expect(err).NotTo(HaveOccurred())
		return srv
	}

	It("persists a working row at task creation and sweeps it to failed on reload", func() {
		srv := newPersistTestServer()
		// Simulate a task created and persisted while working (crash mid-flight).
		srv.tasks["a2a-crash"] = &a2aTask{
			ID: "a2a-crash", ContextID: "ctx-crash", UserText: "do it",
			Caller: a2aCaller{ClientName: a2aCallerPIAgent}, State: a2aStateWorking,
			CreatedAt: time.Now(), cancel: func() {},
		}
		srv.persistTaskRecords()
		// working rows must be persisted (not skipped).
		data, err := os.ReadFile(stateFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"working"`))

		// "Restart": fresh server loads the same state file.
		srv2 := newPersistTestServer()
		t, ok := srv2.tasks["a2a-crash"]
		Expect(ok).To(BeTrue(), "task must survive restart")
		Expect(t.State).To(Equal(a2aStateFailed), "in-flight row swept to failed")
		Expect(t.FinalText).To(ContainSubstring("service restarted"))
		Expect(t.terminal()).To(BeTrue())
		// tasks/get on the swept id must NOT be -32001.
		snap := srv2.snapshot(t)
		Expect(snap.Status.State).To(Equal(a2aStateFailed))
	})

	It("persists completed tasks and returns them across restarts unchanged", func() {
		srv := newPersistTestServer()
		srv.tasks["a2a-done"] = &a2aTask{
			ID: "a2a-done", ContextID: "ctx-done", UserText: "q",
			FinalText: "THE-ANSWER", State: a2aStateCompleted,
			Caller: a2aCaller{Login: "u@x"}, CreatedAt: time.Now(),
		}
		srv.persistTaskRecords()

		srv2 := newPersistTestServer()
		t, ok := srv2.tasks["a2a-done"]
		Expect(ok).To(BeTrue())
		Expect(t.State).To(Equal(a2aStateCompleted), "terminal state is preserved verbatim")
		Expect(t.FinalText).To(Equal("THE-ANSWER"))
		Expect(t.terminal()).To(BeTrue())
	})

	It("sweeps a persisted canceled-ambiguous row to failed correctly", func() {
		// A completed record must NOT be swept.
		records := []persistedTaskRecord{{
			ID: "a2a-ok", ContextID: "ctx-ok", State: a2aStateCompleted, FinalText: "kept",
			Caller: a2aCaller{ClientName: a2aCallerPIAgent}, CreatedAt: time.Now(),
		}}
		data, err := json.Marshal(records)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(stateFile, data, 0o600)).To(Succeed())

		srv := newPersistTestServer()
		t := srv.tasks["a2a-ok"]
		Expect(t).NotTo(BeNil())
		Expect(t.State).To(Equal(a2aStateCompleted))
		Expect(t.FinalText).To(Equal("kept"))
	})

	It("tasks/get returns the swept task instead of -32001", func() {
		// Pre-seed a working row, then load it as a "restarted" server would.
		records := []persistedTaskRecord{{
			ID: "a2a-lost", ContextID: "ctx-lost", UserText: "q",
			State: a2aStateWorking, Caller: a2aCaller{ClientName: a2aCallerPIAgent},
			CreatedAt: time.Now(),
		}}
		data, err := json.Marshal(records)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(stateFile, data, 0o600)).To(Succeed())

		srv := newPersistTestServer()
		t, ok := srv.tasks["a2a-lost"]
		Expect(ok).To(BeTrue())
		Expect(t.State).To(Equal(a2aStateFailed))
		snap := srv.snapshot(t)
		Expect(snap.Status.State).To(Equal("failed"))
	})
})

var _ = Describe("a2aServer $/cancel_request", func() {
	var (
		ts      *httptest.Server
		release chan struct{}
	)
	const tok = "tok-123"

	BeforeEach(func() {
		release = make(chan struct{})
		rel := release // capture for runner goroutines that outlive BeforeEach
		_, ts = newA2ATestServer(tok, "", 9*time.Minute, 128, func(ctx context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			select {
			case <-rel:
				return "done", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		})
	})

	AfterEach(func() {
		close(release)
		ts.Close()
	})

	When("a $/cancel_request notification targets an in-flight message/send by request id", func() {
		var taskID string
		var cancelCode int
		var cancelBody []byte

		BeforeEach(func() {
			// Start a long-running task with JSON-RPC id 3.
			_, resp := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
				"jsonrpc": "2.0",
				"id":      3,
				"method":  "message/send",
				"params": map[string]any{
					"message": map[string]any{
						"role":      "user",
						"parts":     []map[string]any{{"kind": "text", "text": "long task"}},
						"contextId": "ctx-cancel-test",
					},
				},
			})
			taskID = a2aStr(a2aMap(resp, "result")["id"])

			// Wait until the runner is blocked inside the seam.
			Eventually(func() bool {
				_, r := dispatchA2A(ts, bearerHeaders(tok), map[string]any{
					"jsonrpc": "2.0", "id": 7, "method": "tasks/get",
					"params": map[string]any{"id": taskID},
				})
				result, _ := r["result"].(map[string]any)
				if result == nil {
					return false
				}
				state, _ := a2aMap(result, "status")["state"].(string)
				return state == "working"
			}).Within(2 * time.Second).Should(BeTrue())

			// Send $/cancel_request referencing the original JSON-RPC id 3.
			cancelCode, cancelBody = dispatchNotification(ts, bearerHeaders(tok), map[string]any{
				"jsonrpc": "2.0",
				"method":  "$/cancel_request",
				"params":  map[string]any{"requestId": 3},
			})
		})

		It("returns 200 OK", func() {
			Expect(cancelCode).To(Equal(http.StatusOK))
		})

		It("returns an empty response body", func() {
			Expect(cancelBody).To(BeEmpty())
		})

		When("the task is polled after cancellation", func() {
			var finalState string

			BeforeEach(func() {
				finalState, _ = pollTaskUntil(ts, tok, taskID, 2*time.Second)
			})

			It("finishes as canceled, not failed", func() {
				Expect(finalState).To(Equal(a2aStateCanceled))
			})
		})
	})

	When("$/cancel_request targets a request id with no known task", func() {
		var cancelCode int
		var cancelBody []byte

		BeforeEach(func() {
			cancelCode, cancelBody = dispatchNotification(ts, bearerHeaders(tok), map[string]any{
				"jsonrpc": "2.0",
				"method":  "$/cancel_request",
				"params":  map[string]any{"requestId": 42},
			})
		})

		It("returns 200 OK (graceful no-op)", func() {
			Expect(cancelCode).To(Equal(http.StatusOK))
		})

		It("returns an empty response body", func() {
			Expect(cancelBody).To(BeEmpty())
		})
	})
})

var _ = Describe("a2aServer per-task timeout via metadata", func() {
	var ts *httptest.Server
	const tok = "tok-123"

	AfterEach(func() {
		if ts != nil {
			ts.Close()
			ts = nil
		}
	})

	// sendWithMeta dispatches a message/send with metadata merged in.
	sendWithMeta := func(meta map[string]any) (int, map[string]any) {
		body := a2aSendBody("long task")
		body["id"] = 77
		body["params"].(map[string]any)["metadata"] = meta
		return dispatchA2A(ts, bearerHeaders(tok), body)
	}

	It("completes a task past the default 9m via metadata override", func() {
		// Default timeout 50ms would kill this; metadata override keeps it alive.
		_, ts = newA2ATestServer(tok, "", 50*time.Millisecond, 128, func(_ context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			time.Sleep(300 * time.Millisecond)
			return "LONG-DONE", nil
		})
		code, resp := sendWithMeta(map[string]any{"a2a_task_timeout_seconds": 10})
		Expect(code).To(Equal(http.StatusOK))
		taskID := a2aStr(a2aMap(resp, "result")["id"])
		state, text := pollTaskUntil(ts, tok, taskID, 2*time.Second)
		Expect(state).To(Equal("completed"))
		Expect(text).To(Equal("LONG-DONE"))
	})

	It("still fails a task that exceeds its own metadata override", func() {
		_, ts = newA2ATestServer(tok, "", 9*time.Minute, 128, func(ctx context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		})
		code, resp := sendWithMeta(map[string]any{"a2a_task_timeout_seconds": 1})
		Expect(code).To(Equal(http.StatusOK))
		taskID := a2aStr(a2aMap(resp, "result")["id"])
		state, text := pollTaskUntil(ts, tok, taskID, 3*time.Second)
		Expect(state).To(Equal("failed"))
		Expect(text).To(ContainSubstring("task deadline exceeded (1s)"))
	})

	It("rejects a malformed metadata timeout with -32602", func() {
		_, ts = newA2ATestServer(tok, "", 9*time.Minute, 128, nil)
		code, resp := sendWithMeta(map[string]any{"a2a_task_timeout_seconds": "not-a-number"})
		Expect(code).To(Equal(http.StatusOK))
		errObj := a2aMap(resp, "error")
		Expect(errObj["code"]).To(Equal(float64(a2aErrInvalidParams)))
		Expect(errObj["message"]).To(ContainSubstring("a2a_task_timeout_seconds"))
	})

	It("clamps an oversized override to MaxTaskTimeout", func() {
		srv, tsClose := newA2ATestServer(tok, "", 9*time.Minute, 128, func(_ context.Context, _ *a2aTask, _ a2aCaller) (string, error) {
			return "fast", nil
		})
		defer tsClose.Close()
		// Ask for a week; the server must clamp to MaxTaskTimeout (default 60m).
		got := srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": 604800.0})
		Expect(got).To(Equal(60 * time.Minute))
	})

	It("uses the server default when metadata is absent", func() {
		srv, _ := newA2ATestServer(tok, "", 9*time.Minute, 128, nil)
		// Explicit config passes through unchanged...
		Expect(srv.taskTimeoutFromMetadata(nil)).To(Equal(9 * time.Minute))
		Expect(srv.taskTimeoutFromMetadata(map[string]any{})).To(Equal(9 * time.Minute))
		// ...and the code default is 30m (pool flag Value references it).
		srv2, _ := newA2ATestServer(tok, "", 0, 128, nil) // 0 → defaultA2ATaskTimeout
		Expect(srv2.cfg.TaskTimeout).To(Equal(defaultA2ATaskTimeout))
		Expect(srv2.cfg.TaskTimeout).To(Equal(30 * time.Minute))
	})
})

var _ = Describe("a2aServer progress mirroring", func() {
	// No httptest server: this block exercises the collector/sink plumbing
	// directly inside the server object.

	const tok = "tok-123"

	It("mirrors SessionUpdate chunks into ProgressText by hand", func() {
		srv, _ := newA2ATestServer(tok, "", 9*time.Minute, 128, nil)
		t := &a2aTask{ID: "a2a-prog", ContextID: "ctx-p", UserText: "q", Caller: a2aCaller{ClientName: a2aCallerTailnet}, State: a2aStateWorking}

		sink := make(chan string, 4)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case text, ok := <-sink:
					if !ok {
						return
					}
					srv.mu.Lock()
					t.ProgressText = text
					srv.mu.Unlock()
				case <-done:
					return
				}
			}
		}()
		client := &a2aTaskClient{task: t, progressSink: sink}
		chunk := func(s string) acp.SessionNotification {
			return acp.SessionNotification{Update: acp.SessionUpdate{AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
				Content: acp.ContentBlock{Text: &acp.ContentBlockText{Text: s, Type: "text"}},
			}}}
		}
		Expect(client.SessionUpdate(context.Background(), chunk("first "))).To(Succeed())
		Expect(client.SessionUpdate(context.Background(), chunk("second"))).To(Succeed())
		close(sink)
		Eventually(done, time.Second).Should(BeClosed())
		srv.mu.Lock()
		// For short turns (under a2aProgressMaxRunes), the sink carries the
		// full turn text — not only the last chunk.
		Expect(t.ProgressText).To(Equal("first second"))
		srv.mu.Unlock()
		Expect(client.finalText()).To(Equal("first second"))
	})

	It("truncates mirrored progress to a tail window", func() {
		long := strings.Repeat("ab", a2aProgressMaxRunes) + "TAIL"
		tr := truncateProgress(long)
		Expect(tr).To(HaveSuffix("TAIL"))
		Expect(len([]rune(tr))).To(Equal(a2aProgressMaxRunes)) // tail window only
		Expect(truncateProgress("short")).To(Equal("short"))
	})

	When("chunks exceed a2aProgressMaxRunes in total", func() {
		var client *a2aTaskClient
		var allSinkValues []string
		var numChunks int

		BeforeEach(func() {
			const chunkRunes = 50
			numChunks = (a2aProgressMaxRunes / chunkRunes) + 5 // clearly exceeds the limit
			sink := make(chan string, numChunks)                // buffered to capture every value
			client = &a2aTaskClient{task: &a2aTask{}, progressSink: sink}
			chunkText := strings.Repeat("x", chunkRunes)

			chunkNotif := func(s string) acp.SessionNotification {
				return acp.SessionNotification{Update: acp.SessionUpdate{AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
					Content: acp.ContentBlock{Text: &acp.ContentBlockText{Text: s, Type: "text"}},
				}}}
			}
			for i := 0; i < numChunks; i++ {
				Expect(client.SessionUpdate(context.Background(), chunkNotif(chunkText))).To(Succeed())
			}
			close(sink)

			allSinkValues = nil
			for v := range sink {
				allSinkValues = append(allSinkValues, v)
			}
		})

		It("should have received a sink value for every chunk", func() {
			Expect(allSinkValues).To(HaveLen(numChunks))
		})

		It("limits every sink payload to a2aProgressMaxRunes runes", func() {
			for _, v := range allSinkValues {
				Expect(len([]rune(v))).To(BeNumerically("<=", a2aProgressMaxRunes))
			}
		})

		It("prefixes the last tail value with an ellipsis indicating clipping", func() {
			Expect(allSinkValues[len(allSinkValues)-1]).To(HavePrefix("…"))
		})

		It("preserves the full accumulated text in finalText", func() {
			Expect(client.finalText()).To(HaveLen(numChunks * 50))
		})
	})
})

var _ = Describe("taskTimeoutFromMetadata edge cases", func() {
	var srv *a2aServer

	BeforeEach(func() {
		srv, _ = newA2ATestServer("tok", "", 9*time.Minute, 128, nil)
	})

	When("the metadata key is a valid numeric string", func() {
		var result time.Duration

		BeforeEach(func() {
			result = srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": "10"})
		})

		It("parses the string as seconds", func() {
			Expect(result).To(Equal(10 * time.Second))
		})
	})

	When("the metadata key is a json.Number", func() {
		var result time.Duration

		BeforeEach(func() {
			result = srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": json.Number("5")})
		})

		It("parses the json.Number as seconds", func() {
			Expect(result).To(Equal(5 * time.Second))
		})
	})

	When("the metadata key is an invalid json.Number", func() {
		var result time.Duration

		BeforeEach(func() {
			result = srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": json.Number("not-a-number")})
		})

		It("returns -1", func() {
			Expect(result).To(Equal(time.Duration(-1)))
		})
	})

	When("the metadata key is an unknown type", func() {
		var result time.Duration

		BeforeEach(func() {
			result = srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": true})
		})

		It("returns -1", func() {
			Expect(result).To(Equal(time.Duration(-1)))
		})
	})

	When("the metadata value is zero seconds", func() {
		var result time.Duration

		BeforeEach(func() {
			result = srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": 0.0})
		})

		It("returns -1", func() {
			Expect(result).To(Equal(time.Duration(-1)))
		})
	})

	When("the metadata value is negative seconds", func() {
		var result time.Duration

		BeforeEach(func() {
			result = srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": -1.0})
		})

		It("returns -1", func() {
			Expect(result).To(Equal(time.Duration(-1)))
		})
	})

	When("the metadata value is a sub-second positive number", func() {
		var result time.Duration

		BeforeEach(func() {
			result = srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": 0.001})
		})

		It("returns -1", func() {
			Expect(result).To(Equal(time.Duration(-1)))
		})
	})

	When("the metadata key exists but is nil", func() {
		var result time.Duration

		BeforeEach(func() {
			result = srv.taskTimeoutFromMetadata(map[string]any{"a2a_task_timeout_seconds": nil})
		})

		It("returns the server default timeout", func() {
			Expect(result).To(Equal(9 * time.Minute))
		})
	})
})

var _ = Describe("installProgressSink", func() {
	const tok = "tok-123"

	When("a progress sink is installed on a client", func() {
		var srv *a2aServer
		var task *a2aTask
		var client *a2aTaskClient
		var teardown func()

		BeforeEach(func() {
			srv, _ = newA2ATestServer(tok, "", 9*time.Minute, 128, nil)
			task = &a2aTask{
				ID: "a2a-sink-test", ContextID: "ctx-sink", UserText: "q",
				Caller: a2aCaller{ClientName: a2aCallerTailnet}, State: a2aStateWorking,
			}
			client = &a2aTaskClient{task: task}
			teardown = srv.installProgressSink(client, task)
		})

		It("sets progressSink on the client", func() {
			client.mu.Lock()
			defer client.mu.Unlock()
			Expect(client.progressSink).NotTo(BeNil())
		})

		When("a chunk arrives and teardown is called", func() {
			BeforeEach(func() {
				Expect(client.SessionUpdate(context.Background(), acp.SessionNotification{
					Update: acp.SessionUpdate{AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
						Content: acp.ContentBlock{Text: &acp.ContentBlockText{Text: "live text", Type: "text"}},
					}},
				})).To(Succeed())
				teardown()
			})

			It("clears progressSink on the client after teardown", func() {
				client.mu.Lock()
				defer client.mu.Unlock()
				Expect(client.progressSink).To(BeNil())
			})

			It("mirrors the chunk into task.ProgressText", func() {
				srv.mu.Lock()
				defer srv.mu.Unlock()
				Expect(task.ProgressText).To(Equal("live text"))
			})
		})
	})
})

var _ = Describe("a2aServer snapshot progress branch", func() {
	When("a working task has ProgressText set", func() {
		var snap *a2aTaskJSON
		var srv *a2aServer

		BeforeEach(func() {
			srv, _ = newA2ATestServer("tok", "", 9*time.Minute, 128, nil)
			t := &a2aTask{
				ID: "a2a-snap", ContextID: "ctx-s", UserText: "q",
				Caller: a2aCaller{ClientName: a2aCallerTailnet},
				State: a2aStateWorking, ProgressText: "live output",
			}
			snap = srv.snapshot(t)
		})

		It("exposes ProgressText as status.progress", func() {
			Expect(snap.Status.Progress).To(Equal("live output"))
		})

		It("does not set Artifacts", func() {
			Expect(snap.Artifacts).To(BeNil())
		})
	})
})

var _ = Describe("newA2AServer config validation", func() {
	When("MaxTaskTimeout is smaller than TaskTimeout", func() {
		var err error

		BeforeEach(func() {
			_, err = newA2AServer(a2aServerConfig{
				BearerToken:    "tok",
				TaskTimeout:    10 * time.Minute,
				MaxTaskTimeout: 5 * time.Minute,
			}, &poolDaemon{})
		})

		It("returns an error mentioning the constraint", func() {
			Expect(err).To(MatchError(ContainSubstring("must be >=")))
		})
	})
})
