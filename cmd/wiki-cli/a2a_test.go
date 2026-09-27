package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

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
		PublicURL:   "https://mcp.example.net/dorium",
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
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&decoded)
	Expect(err).NotTo(HaveOccurred())
	return resp.StatusCode, decoded
}

func bearerHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
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
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		var card map[string]any
		Expect(json.NewDecoder(resp.Body).Decode(&card)).To(Succeed())
		Expect(card["name"]).To(Equal("dorium"))
		Expect(card["url"]).To(Equal("https://mcp.example.net/dorium"))
		Expect(card["version"]).NotTo(BeEmpty())
		capabilities := a2aMap(card, "capabilities")
		Expect(capabilities["streaming"]).To(BeFalse())
	})

	It("404s unknown paths", func() {
		resp, err := ts.Client().Get(ts.URL + "/nope")
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
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

	It("rejects dispatch without credentials", func() {
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
		code, _ := dispatchA2A(ts, map[string]string{trustedProxySecretHeader: "wrong"}, map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "tasks/list", "params": map[string]any{},
		})
		Expect(code).To(Equal(http.StatusUnauthorized))
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
		defer resp.Body.Close()
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
		defer resp.Body.Close()
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
	It("errors when port>0 and no credentials are set", func() {
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
})
