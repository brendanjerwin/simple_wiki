// keep-debug is a single-binary diagnostic CLI for the Keep bridge.
// It is intentionally structured around branchy subcommand handlers
// that exit on error and write rich progress text to stdout — the
// shape that's most useful when poking live Keep responses by hand.
// The revive rules below relax production-code conventions that
// don't fit a one-off operator tool.
//
//revive:disable:deep-exit
//revive:disable:unhandled-error
//revive:disable:cognitive-complexity
//revive:disable:cyclomatic
//revive:disable:function-length
//revive:disable:unchecked-type-assertion
//revive:disable:add-constant
//revive:disable:flag-parameter
package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
)

func main() {
	cmd := flag.String("cmd", "list", "list | create-and-push | create-with-items | push-item-to-existing | dump-items | dump-write-results | verify-cursor-monotonic | verify-list-push-shape | verify-list-push-shape-broken | verify-listitem-update-shape | verify-list-push-shape-prod-replay | verify-list-push-noop | diagnose-grocery-list-push")
	title := flag.String("title", "Keep CLI Test", "title for create-and-push / create-with-items")
	itemsCSV := flag.String("items", "Eggs,Milk,Bread", "comma-separated items for create-with-items")
	parentID := flag.String("parent-id", "", "for push-item-to-existing: the LIST node's serverID")
	itemText := flag.String("item-text", "Late Add", "for push-item-to-existing: the new item's text")
	labelName := flag.String("label-name", "", "for verify-list-push-shape: existing label to attach (case-insensitive). If empty, push without labels.")
	flag.Parse()

	email := os.Getenv("KEEP_EMAIL")
	masterToken := os.Getenv("KEEP_MASTER_TOKEN")
	deviceID := os.Getenv("KEEP_DEVICE_ID")
	if email == "" || masterToken == "" || deviceID == "" {
		fmt.Fprintln(os.Stderr, "set KEEP_EMAIL, KEEP_MASTER_TOKEN, KEEP_DEVICE_ID")
		os.Exit(2)
	}

	ctx := context.Background()
	authClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}},
			ForceAttemptHTTP2: false,
		},
		Timeout: 30 * time.Second,
	}
	auth := gateway.NewAuthenticator(authClient, gateway.AuthURL, deviceID)
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Stage 2 failed:", err)
		os.Exit(1)
	}
	fmt.Println("✓ bearer obtained, len:", len(bearer))

	keep := gateway.NewKeepClient(http.DefaultClient, gateway.DefaultKeepBaseURL, bearer)

	switch *cmd {
	case "list":
		runList(ctx, keep)
	case "create-and-push":
		runCreateAndPush(ctx, keep, *title)
	case "create-with-items":
		runCreateWithItems(ctx, keep, *title, strings.Split(*itemsCSV, ","))
	case "push-item-to-existing":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "push-item-to-existing requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runPushItemToExisting(ctx, keep, *parentID, *itemText)
	case "dump-items":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "dump-items requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runDumpItems(ctx, keep, *parentID)
	case "raw-pull":
		runRawPull(ctx, email, masterToken, deviceID, *parentID)
	case "trash-one":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "trash-one requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runTrashOne(ctx, keep, *parentID, *itemText)
	case "update-and-trash":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "update-and-trash requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runUpdateAndTrash(ctx, keep, *parentID)
	case "delete-many":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "delete-many requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runDeleteMany(ctx, keep, *parentID)
	case "update-item":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "update-item requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runUpdateItemMatching(ctx, keep, *parentID, *itemText)
	case "dump-write-results":
		runDumpWriteResults(ctx, email, masterToken, deviceID, keep)
	case "verify-cursor-monotonic":
		runVerifyCursorMonotonic(ctx, keep)
	case "verify-list-push-shape":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "verify-list-push-shape requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runVerifyListPushShape(ctx, email, masterToken, deviceID, keep, *parentID, *labelName, false /*sendBrokenShape*/)
	case "verify-list-push-shape-broken":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "verify-list-push-shape-broken requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runVerifyListPushShape(ctx, email, masterToken, deviceID, keep, *parentID, *labelName, true /*sendBrokenShape*/)
	case "verify-listitem-update-shape":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "verify-listitem-update-shape requires -parent-id=<list-serverID>")
			os.Exit(2)
		}
		runVerifyListItemUpdateShape(ctx, email, masterToken, deviceID, keep, *parentID)
	case "verify-list-push-shape-prod-replay":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "verify-list-push-shape-prod-replay requires -parent-id=<sandbox-list-serverID>")
			os.Exit(2)
		}
		if *labelName == "" {
			fmt.Fprintln(os.Stderr, "verify-list-push-shape-prod-replay requires -label-name=<existing-label-name>")
			os.Exit(2)
		}
		runVerifyListPushShapeProdReplay(ctx, email, masterToken, deviceID, *parentID, *labelName)
	case "verify-list-push-noop":
		if *parentID == "" {
			fmt.Fprintln(os.Stderr, "verify-list-push-noop requires -parent-id=<sandbox-list-serverID>")
			os.Exit(2)
		}
		runVerifyListPushNoop(ctx, email, masterToken, deviceID, keep, *parentID)
	case "diagnose-grocery-list-push":
		runDiagnoseGroceryListPush(ctx, email, masterToken, deviceID)
	default:
		fmt.Fprintln(os.Stderr, "unknown cmd:", *cmd)
		os.Exit(2)
	}
}

// runUpdateItemMatching: pull, find ALL LIST_ITEMs under the given
// list whose text contains the substring `match` (or all alive items
// if match is empty), then push them back as UPDATES in a single
// request — reproduces the cron-tick shape.
func runUpdateItemMatching(ctx context.Context, keep *gateway.KeepClient, listServerID, match string) {
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--upd-pull", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "pull failed:", err)
		os.Exit(1)
	}
	var targets []gateway.Node
	for _, n := range pull.Nodes {
		if n.Type != gateway.NodeTypeListItem {
			continue
		}
		if n.ParentID != listServerID && n.ParentServerID != listServerID {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		if match != "" && !strings.Contains(n.Text, match) {
			continue
		}
		targets = append(targets, n)
	}
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "no items matching", match)
		os.Exit(1)
	}
	fmt.Printf("found %d targets\n", len(targets))

	now := time.Now().UTC()
	pushNodes := make([]gateway.Node, 0, len(targets))
	for i, t := range targets {
		_ = i
		pushNodes = append(pushNodes, gateway.Node{
			Kind:           "notes#node",
			ID:             t.ID,
			ServerID:       t.ServerID,
			ParentID:       listServerID,
			ParentServerID: listServerID,
			Type:           gateway.NodeTypeListItem,
			Text:           t.Text + " edit",
			Checked:        t.Checked,
			SortValue:      t.SortValue,
			BaseVersion:    t.BaseVersion,
			Timestamps:     gateway.Timestamps{Updated: now},
		})
	}
	// Wire-shape debug logger: prints the marshaled wire body so we
	// see exactly what Keep gets.
	keep.SetDebugLogger(stderrDebug{})
	resp, err := keep.Changes(ctx, gateway.ChangesRequest{
		Nodes:           pushNodes,
		TargetVersion:   pull.ToVersion,
		SessionID:       fmt.Sprintf("s--%d--upd-push", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "update push failed:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ update succeeded; toVersion=%s response nodes=%d\n", resp.ToVersion, len(resp.Nodes))
}

// runTrashOne: pull, find one alive item under listServerID matching
// `match` (or any first alive if match==""), push a Trashed=now node
// for it. Tests the soft-delete wire shape in isolation.
func runTrashOne(ctx context.Context, keep *gateway.KeepClient, listServerID, match string) {
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--trash-pull", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "pull:", err); os.Exit(1) }
	var target gateway.Node
	for _, n := range pull.Nodes {
		if n.Type != gateway.NodeTypeListItem { continue }
		if n.ParentID != listServerID && n.ParentServerID != listServerID { continue }
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() { continue }
		if match != "" && !strings.Contains(n.Text, match) { continue }
		target = n; break
	}
	if target.ServerID == "" { fmt.Fprintln(os.Stderr, "no alive item match"); os.Exit(1) }
	now := time.Now().UTC()
	keep.SetDebugLogger(stderrDebug{})
	resp, err := keep.Changes(ctx, gateway.ChangesRequest{
		Nodes: []gateway.Node{{
			Kind: "notes#node",
			ID: target.ID,
			ServerID: target.ServerID,
			ParentID: listServerID,
			ParentServerID: listServerID,
			Type: gateway.NodeTypeListItem,
			Text: target.Text,         // include
			Checked: target.Checked,   // include
			SortValue: target.SortValue, // include
			BaseVersion: target.BaseVersion,
			Timestamps: gateway.Timestamps{
				Updated: now,
				Deleted: now,  // try Deleted instead of Trashed
			},
		}},
		TargetVersion: pull.ToVersion,
		SessionID: fmt.Sprintf("s--%d--trash-push", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "trash push:", err); os.Exit(1) }
	fmt.Printf("✓ trash succeeded; toVersion=%s nodes=%d\n", resp.ToVersion, len(resp.Nodes))
}

// runUpdateAndTrash: pull, find 2 alive items, push 1 update + 1
// trash in same Changes call. Tests whether bundling updates with
// soft-deletes is the trigger for the user's 500.
func runUpdateAndTrash(ctx context.Context, keep *gateway.KeepClient, listServerID string) {
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--mix-pull", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "pull:", err); os.Exit(1) }
	var alive []gateway.Node
	for _, n := range pull.Nodes {
		if n.Type != gateway.NodeTypeListItem { continue }
		if n.ParentID != listServerID && n.ParentServerID != listServerID { continue }
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() { continue }
		alive = append(alive, n)
		if len(alive) >= 2 { break }
	}
	if len(alive) < 2 { fmt.Fprintln(os.Stderr, "need >= 2 alive items"); os.Exit(1) }
	now := time.Now().UTC()
	keep.SetDebugLogger(stderrDebug{})
	resp, err := keep.Changes(ctx, gateway.ChangesRequest{
		Nodes: []gateway.Node{
			{ // update
				Kind: "notes#node", ID: alive[0].ID, ServerID: alive[0].ServerID,
				ParentID: listServerID, ParentServerID: listServerID,
				Type: gateway.NodeTypeListItem, Text: alive[0].Text + " edit",
				Checked: alive[0].Checked, SortValue: alive[0].SortValue,
				BaseVersion: alive[0].BaseVersion,
				Timestamps: gateway.Timestamps{Updated: now},
			},
			{ // delete (using Deleted, not Trashed)
				Kind: "notes#node", ID: alive[1].ID, ServerID: alive[1].ServerID,
				ParentID: listServerID, ParentServerID: listServerID,
				Type: gateway.NodeTypeListItem,
				BaseVersion: alive[1].BaseVersion,
				Timestamps: gateway.Timestamps{Updated: now, Deleted: now},
			},
		},
		TargetVersion: pull.ToVersion,
		SessionID: fmt.Sprintf("s--%d--mix-push", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "mixed push:", err); os.Exit(1) }
	fmt.Printf("✓ mixed push succeeded; toVersion=%s nodes=%d\n", resp.ToVersion, len(resp.Nodes))
}

// runDeleteMany: delete ALL alive items in a list as one push
func runDeleteMany(ctx context.Context, keep *gateway.KeepClient, listServerID string) {
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--del-pull", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "pull:", err); os.Exit(1) }
	var alive []gateway.Node
	for _, n := range pull.Nodes {
		if n.Type != gateway.NodeTypeListItem { continue }
		if n.ParentID != listServerID && n.ParentServerID != listServerID { continue }
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() { continue }
		alive = append(alive, n)
	}
	if len(alive) == 0 { fmt.Fprintln(os.Stderr, "no alive items"); os.Exit(1) }
	fmt.Printf("deleting %d items\n", len(alive))
	now := time.Now().UTC()
	pushNodes := make([]gateway.Node, 0, len(alive))
	for _, t := range alive {
		pushNodes = append(pushNodes, gateway.Node{
			Kind: "notes#node", ID: t.ID, ServerID: t.ServerID,
			ParentID: listServerID, ParentServerID: listServerID,
			Type: gateway.NodeTypeListItem,
			BaseVersion: t.BaseVersion,
			Timestamps: gateway.Timestamps{Updated: now, Deleted: now},
		})
	}
	keep.SetDebugLogger(stderrDebug{})
	resp, err := keep.Changes(ctx, gateway.ChangesRequest{
		Nodes: pushNodes, TargetVersion: pull.ToVersion,
		SessionID: fmt.Sprintf("s--%d--del-push", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "delete-many:", err); os.Exit(1) }
	fmt.Printf("✓ ok; toVersion=%s nodes=%d\n", resp.ToVersion, len(resp.Nodes))
}

type stderrDebug struct{}

func (stderrDebug) Info(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
}

// runRawPull does its own raw HTTP request to /changes so we can see
// the literal JSON response (the protocol package decodes into typed
// structs, dropping unknown fields).
func runRawPull(ctx context.Context, email, masterToken, deviceID, filterParent string) {
	authClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}},
			ForceAttemptHTTP2: false,
		},
		Timeout: 30 * time.Second,
	}
	auth := gateway.NewAuthenticator(authClient, gateway.AuthURL, deviceID)
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
	if err != nil {
		fmt.Fprintln(os.Stderr, "auth failed:", err)
		os.Exit(1)
	}

	body := fmt.Sprintf(`{"nodes":[],"clientTimestamp":%q,"requestHeader":{"clientSessionId":"raw-pull","clientPlatform":"ANDROID","clientVersion":{"major":"9","minor":"9","build":"9","revision":"9"},"capabilities":[{"type":"NC"},{"type":"PI"},{"type":"LB"},{"type":"AN"},{"type":"SH"},{"type":"DR"},{"type":"TR"},{"type":"IN"},{"type":"SNB"},{"type":"MI"},{"type":"CO"}]}}`,
		time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"))
	req, _ := http.NewRequestWithContext(ctx, "POST", gateway.DefaultKeepBaseURL+"changes", strings.NewReader(body))
	req.Header.Set("Authorization", "OAuth "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "simple_wiki-keep-debug/1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pull failed:", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		fmt.Fprintln(os.Stderr, "decode failed:", err)
		os.Exit(1)
	}
	nodes, _ := parsed["nodes"].([]any)
	for _, n := range nodes {
		m, _ := n.(map[string]any)
		if m == nil {
			continue
		}
		if filterParent != "" {
			if m["parentServerId"] != filterParent && m["parentId"] != filterParent {
				continue
			}
		}
		out, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(out))
		fmt.Println("---")
	}
}

// runDumpWriteResults posts a deliberate-conflict Changes push (a
// soft-delete keyed to a bogus serverID that does not exist on the
// account) and prints the raw response body. The point is to confirm
// the wire shape of the per-pushed-node status array Keep echoes back
// — assumed at protocol-decode time to be `writeResults: [{id,
// status}]` based on prior keep-debug logs, but never empirically
// pinned. Run against a real account; inspect stdout for the actual
// field name.
//
// We use a raw HTTP request rather than KeepClient.Changes so the
// literal response body lands on stdout — the typed decoder discards
// fields it doesn't model.
func runDumpWriteResults(ctx context.Context, email, masterToken, deviceID string, keep *gateway.KeepClient) {
	// Step 1: pull to capture toVersion. We use the typed client here
	// because we just need the cursor; the response body itself is
	// uninteresting for this diagnostic.
	now := time.Now().UTC()
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--dwr-pull", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "pull failed:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ pull got toVersion=%s (%d nodes)\n", pull.ToVersion, len(pull.Nodes))

	// Step 2: re-auth a raw HTTP client so we can dump the literal
	// response body. The bearer above is on the typed client and isn't
	// reachable from here.
	authClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}},
			ForceAttemptHTTP2: false,
		},
		Timeout: 30 * time.Second,
	}
	auth := gateway.NewAuthenticator(authClient, gateway.AuthURL, deviceID)
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
	if err != nil {
		fmt.Fprintln(os.Stderr, "raw auth failed:", err)
		os.Exit(1)
	}

	// Step 3: build a deliberately-conflicting push body. We send a
	// soft-delete (Deleted=now) for a node whose serverId/parentServerId
	// don't exist on this account. Either Keep responds with a per-node
	// status entry indicating ERROR, or the whole call fails — both
	// outcomes are informative.
	now2 := time.Now().UTC()
	bogusItemClientID := fmt.Sprintf("%x.%016x", now2.UnixMilli(), uint64(0xDEADBEEFCAFEBABE))
	bogusItemServerID := "BOGUS_NONEXISTENT_ITEM_SERVERID_FOR_DIAGNOSTIC"
	bogusParentServerID := "BOGUS_NONEXISTENT_LIST_SERVERID_FOR_DIAGNOSTIC"

	var sessionEntropy [8]byte
	_, _ = rand.Read(sessionEntropy[:])
	sessionID := fmt.Sprintf("s--%d--%010d", now2.UnixMilli(),
		(binary.BigEndian.Uint64(sessionEntropy[:])%9000000000)+1000000000)

	pushBody := map[string]any{
		"nodes": []any{
			map[string]any{
				"kind":           "notes#node",
				"id":             bogusItemClientID,
				"serverId":       bogusItemServerID,
				"parentId":       bogusParentServerID,
				"parentServerId": bogusParentServerID,
				"type":           "LIST_ITEM",
				"timestamps": map[string]any{
					"kind":    "notes#timestamps",
					"updated": now2.Format("2006-01-02T15:04:05.000000Z"),
					"deleted": now2.Format("2006-01-02T15:04:05.000000Z"),
				},
			},
		},
		"clientTimestamp": now2.Format("2006-01-02T15:04:05.000000Z"),
		"targetVersion":   pull.ToVersion,
		"requestHeader": map[string]any{
			"clientSessionId": sessionID,
			"clientPlatform":  "ANDROID",
			"clientVersion": map[string]any{
				"major": "9", "minor": "9", "build": "9", "revision": "9",
			},
			"capabilities": []any{
				map[string]any{"type": "NC"}, map[string]any{"type": "PI"},
				map[string]any{"type": "LB"}, map[string]any{"type": "AN"},
				map[string]any{"type": "SH"}, map[string]any{"type": "DR"},
				map[string]any{"type": "TR"}, map[string]any{"type": "IN"},
				map[string]any{"type": "SNB"}, map[string]any{"type": "MI"},
				map[string]any{"type": "CO"},
			},
		},
	}
	body, err := json.Marshal(pushBody)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal failed:", err)
		os.Exit(1)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		gateway.DefaultKeepBaseURL+"changes", strings.NewReader(string(body)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "new request failed:", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "OAuth "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "simple_wiki-keep-debug/1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "push failed:", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read response body failed:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "✓ push HTTP status: %d\n", resp.StatusCode)
	fmt.Fprintln(os.Stderr, "--- RAW RESPONSE BODY (stdout) ---")
	// Print to stdout so the body can be redirected/piped without
	// the diagnostic chatter on stderr.
	fmt.Println(string(rawBody))

	// Also pretty-print if JSON parses, to make the field shape
	// visually obvious.
	var pretty any
	if err := json.Unmarshal(rawBody, &pretty); err == nil {
		out, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Fprintln(os.Stderr, "--- pretty (stderr) ---")
		fmt.Fprintln(os.Stderr, string(out))
	} else {
		fmt.Fprintln(os.Stderr, "(response is not JSON; only raw bytes printed)")
	}
}

// runVerifyCursorMonotonic performs 6 sequential pulls (the first
// with empty TargetVersion, each subsequent one with the prior pull's
// ToVersion as TargetVersion) and asserts that the captured
// to_version values are strictly lex-ordered in temporal order. This
// empirically confirms whether Keep's cursor encoding is safe to
// compare with Go's `<` string operator. If the assertion fails, the
// runtime invariant in the connector must be relaxed (equality with
// prior cursor) or replaced with a decode-then-compare comparator.
//
// This is the Phase-2 prerequisite from the plan's "Pre-flight
// verification of lex monotonicity" section. Exits 0 on success, 1
// on the first non-monotonic adjacent pair.
func runVerifyCursorMonotonic(ctx context.Context, keep *gateway.KeepClient) {
	const numPulls = 6
	const interPullDelay = 1500 * time.Millisecond

	versions := make([]string, 0, numPulls)
	prevTarget := ""
	for i := 0; i < numPulls; i++ {
		now := time.Now().UTC()
		req := gateway.ChangesRequest{
			TargetVersion:   prevTarget,
			SessionID:       fmt.Sprintf("s--%d--vcm-%d", now.UnixMilli(), i),
			ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
		}
		resp, err := keep.Changes(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pull %d failed (targetVersion=%q): %v\n", i, prevTarget, err)
			os.Exit(1)
		}
		fmt.Printf("pull[%d] targetVersion=%q -> toVersion=%q (nodes=%d, truncated=%v)\n",
			i, prevTarget, resp.ToVersion, len(resp.Nodes), resp.Truncated)
		if resp.ToVersion == "" {
			fmt.Fprintf(os.Stderr, "pull %d returned empty toVersion; cannot verify monotonicity\n", i)
			os.Exit(1)
		}
		versions = append(versions, resp.ToVersion)
		prevTarget = resp.ToVersion

		if i < numPulls-1 {
			time.Sleep(interPullDelay)
		}
	}

	// Assert lex monotonicity for every adjacent pair. We use strict
	// `<` first; if any pair violates strict ordering we report it.
	// Equal-adjacent pairs (no changes between pulls) are allowed —
	// strictly-equal cursors are still safe (the invariant is
	// "non-decreasing", not "strictly increasing").
	for i := 1; i < len(versions); i++ {
		prev, next := versions[i-1], versions[i]
		if next < prev {
			fmt.Fprintf(os.Stderr,
				"FAIL: lex order violated at pair [%d -> %d]: prior=%q next=%q (next < prior)\n",
				i-1, i, prev, next)
			fmt.Fprintln(os.Stderr, "captured to_version values in temporal order:")
			for j, v := range versions {
				fmt.Fprintf(os.Stderr, "  [%d] %q\n", j, v)
			}
			os.Exit(1)
		}
	}

	fmt.Printf("verified: %d consecutive to_version values are lexicographically monotonic\n", len(versions))
}

func runDumpItems(ctx context.Context, keep *gateway.KeepClient, listServerID string) {
	resp, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--dump", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "dump pull failed:", err)
		os.Exit(1)
	}
	for _, n := range resp.Nodes {
		if n.Type != gateway.NodeTypeListItem {
			continue
		}
		if n.ParentID != listServerID && n.ParentServerID != listServerID {
			continue
		}
		fmt.Printf("id=%s\n  serverId=%s\n  text=%q checked=%v\n  baseVersion=%q\n  Created=%s\n  Updated=%s\n  UserEdited=%s\n  Trashed=%s\n  Deleted=%s\n\n",
			n.ID, n.ServerID, n.Text, n.Checked, n.BaseVersion,
			n.Timestamps.Created, n.Timestamps.Updated, n.Timestamps.UserEdited, n.Timestamps.Trashed, n.Timestamps.Deleted)
	}
}

func runPushItemToExisting(ctx context.Context, keep *gateway.KeepClient, parentServerID, text string) {
	now := time.Now().UTC()

	// Step 1: pull to get the latest toVersion. gkeepapi follows a strict
	// sync-then-push pattern; pushing with empty TargetVersion gets a 500
	// "Unknown Error" because the server can't reconcile the partial
	// node update against an unknown client baseline.
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--pull", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "pull failed:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ pull got toVersion=%s (%d nodes)\n", pull.ToVersion, len(pull.Nodes))

	var entropy [8]byte
	_, _ = rand.Read(entropy[:])
	itemClientID := fmt.Sprintf("%x.%016x", now.UnixMilli(), binary.BigEndian.Uint64(entropy[:]))

	var sessionEntropy [8]byte
	_, _ = rand.Read(sessionEntropy[:])
	sessionID := fmt.Sprintf("s--%d--%010d", now.UnixMilli(),
		(binary.BigEndian.Uint64(sessionEntropy[:])%9000000000)+1000000000)

	// Step 2: push with TargetVersion = the toVersion we just pulled.
	// LIST_ITEM going to an existing list needs BOTH a parent_id
	// (a client-side reference, can be the serverId for an existing
	// list) and parent_server_id (the actual serverId). Without
	// parent_server_id, Keep returns 500 "Unknown Error" because it
	// can't reconcile the partial node update against an unknown
	// parent (gkeepapi node.py line 1585).
	resp, err := keep.Changes(ctx, gateway.ChangesRequest{
		Nodes: []gateway.Node{
			{
				Kind:           "notes#node",
				ID:             itemClientID,
				Type:           gateway.NodeTypeListItem,
				ParentID:       parentServerID,
				ParentServerID: parentServerID,
				Text:           text,
				SortValue:      "1000",
				Timestamps: gateway.Timestamps{
					Created: now,
					Updated: now,
				},
			},
		},
		TargetVersion:   pull.ToVersion,
		SessionID:       sessionID,
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "push-item-to-existing failed:", err)
		os.Exit(1)
	}

	for _, n := range resp.Nodes {
		if n.ID == itemClientID && n.Type == gateway.NodeTypeListItem {
			fmt.Printf("✓ item created on existing list: %s -> %s\n", itemClientID, n.ServerID)
			return
		}
	}
	fmt.Println("✗ item NOT echoed back; full response nodes:")
	for _, n := range resp.Nodes {
		if n.Type == gateway.NodeTypeListItem {
			fmt.Printf("  id=%s server=%s parent=%s text=%q\n", n.ID, n.ServerID, n.ParentID, n.Text)
		}
	}
}

func runCreateWithItems(ctx context.Context, keep *gateway.KeepClient, title string, items []string) {
	specs := make([]gateway.ListItemSpec, len(items))
	for i, txt := range items {
		specs[i] = gateway.ListItemSpec{
			Text:      txt,
			SortValue: fmt.Sprintf("%d", (len(items)-i)*1000),
		}
	}
	r, err := keep.CreateListWithItems(ctx, title, specs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "CreateListWithItems failed:", err)
		os.Exit(1)
	}
	fmt.Printf("✓ list created: serverID=%s\n", r.ListServerID)
	fmt.Printf("✓ %d items pushed; server-assigned IDs:\n", len(r.ItemServerIDs))
	for i, id := range r.ItemServerIDs {
		fmt.Printf("  [%d] %q -> %s\n", i, items[i], id)
	}
}

func runList(ctx context.Context, keep *gateway.KeepClient) {
	req := gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--cli", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	}
	resp, err := keep.Changes(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Changes failed:", err)
		os.Exit(1)
	}
	fmt.Println("✓ Changes returned", len(resp.Nodes), "nodes; toVersion:", resp.ToVersion, "truncated:", resp.Truncated)
	byType := map[gateway.NodeType]int{}
	var lists []gateway.Node
	for _, n := range resp.Nodes {
		byType[n.Type]++
		if n.Type == gateway.NodeTypeList {
			lists = append(lists, n)
		}
	}
	fmt.Println("by type:", byType)
	fmt.Println("ALL LIST nodes (with state):")
	for _, n := range lists {
		state := "alive"
		if !n.Timestamps.Trashed.IsZero() {
			state = "trashed " + n.Timestamps.Trashed.Format("2006-01-02")
		}
		if !n.Timestamps.Deleted.IsZero() {
			state = "deleted " + n.Timestamps.Deleted.Format("2006-01-02")
		}
		fmt.Printf("  [%s] serverID=%s title=%q\n", state, n.ServerID, n.Title)
	}
}

func runCreateAndPush(ctx context.Context, keep *gateway.KeepClient, title string) {
	id, err := keep.CreateList(ctx, title)
	if err != nil {
		fmt.Fprintln(os.Stderr, "CreateList failed:", err)
		os.Exit(1)
	}
	fmt.Println("✓ CreateList returned serverID:", id)

	// Now do a pull to confirm the list shows up.
	resp, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--cli2", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "post-create Changes failed:", err)
		os.Exit(1)
	}
	for _, n := range resp.Nodes {
		if n.ServerID == id || n.ID == id {
			out, _ := json.MarshalIndent(n, "  ", "  ")
			fmt.Println("✓ found new list in pull:")
			fmt.Println(string(out))
			return
		}
	}
	fmt.Println("✗ created list NOT visible in next pull. Dumping all LIST nodes:")
	for _, n := range resp.Nodes {
		if n.Type == gateway.NodeTypeList {
			fmt.Printf("  serverID=%s title=%q\n", n.ServerID, n.Title)
		}
	}
}

// runVerifyListPushShape pushes a labelIds-only LIST node update against
// `parentID` (a Sandbox list serverID) using the new push-shape fix:
// `id == LIST.client_id` (from the pull) and `serverId == LIST.server_id`,
// distinct values. Optionally attaches an existing label by case-insensitive
// name match.
//
// Goes raw HTTP for the push so we can capture the literal HTTP status
// and response body — KeepClient.Changes wraps non-200s in classified
// errors that lose the body.
//
// When sendBrokenShape=true, intentionally sets `id == serverId` (the
// pre-fix shape) to confirm Keep returns 500 — the control case proving
// the fix targets the right field.
//
// Exits 0 on PASS (HTTP 200, no error in response body), 1 on FAIL.
func runVerifyListPushShape(ctx context.Context, email, masterToken, deviceID string, keep *gateway.KeepClient, listServerID, labelMatch string, sendBrokenShape bool) {
	mode := "FIXED-SHAPE"
	if sendBrokenShape {
		mode = "BROKEN-SHAPE (id==serverId, expecting 500)"
	}
	fmt.Fprintf(os.Stderr, "=== verify-list-push-shape mode=%s parent-id=%s label=%q ===\n",
		mode, listServerID, labelMatch)

	// Step 1: full pull via the typed client to capture the LIST node's
	// client_id, the toVersion cursor, the title, and any label that
	// matches by case-insensitive name.
	now := time.Now().UTC()
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--vlps-pull", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: pull failed:", err)
		os.Exit(1)
	}

	var listClientID, listTitle string
	for _, n := range pull.Nodes {
		if n.Type == gateway.NodeTypeList && n.ServerID == listServerID {
			listClientID = n.ID
			listTitle = n.Title
			break
		}
	}
	if listClientID == "" {
		fmt.Fprintf(os.Stderr, "FAIL: no LIST node found with serverId=%s in pull (got %d nodes)\n",
			listServerID, len(pull.Nodes))
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ found LIST node: client_id=%s title=%q\n", listClientID, listTitle)

	// Step 2: optionally locate a matching label.
	var labelMainID, labelCanonicalName string
	if labelMatch != "" {
		want := strings.ToLower(labelMatch)
		for _, l := range pull.Labels {
			if strings.ToLower(l.Name) == want {
				labelMainID = l.MainID
				labelCanonicalName = l.Name
				break
			}
		}
		if labelMainID == "" {
			fmt.Fprintf(os.Stderr, "FAIL: no label matching %q (case-insensitive) found among %d labels\n",
				labelMatch, len(pull.Labels))
			for _, l := range pull.Labels {
				fmt.Fprintf(os.Stderr, "  available label: name=%q mainID=%s\n", l.Name, l.MainID)
			}
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "✓ found label: name=%q mainID=%s\n", labelCanonicalName, labelMainID)
	}

	// Step 3: build the push body with the chosen shape.
	pushID := listClientID
	if sendBrokenShape {
		pushID = listServerID // intentionally wrong: id == serverId
	}

	now2 := time.Now().UTC()
	nodeMap := map[string]any{
		"kind":     "notes#node",
		"id":       pushID,
		"serverId": listServerID,
		"type":     "LIST",
		"title":    listTitle,
		"annotationsGroup": map[string]any{
			"kind": "notes#annotationsGroup",
		},
		"timestamps": map[string]any{
			"kind":    "notes#timestamps",
			"updated": now2.Format("2006-01-02T15:04:05.000000Z"),
		},
	}
	if labelMainID != "" {
		nodeMap["labelIds"] = []any{
			map[string]any{"labelId": labelMainID},
		}
	}

	pushBody := map[string]any{
		"nodes":           []any{nodeMap},
		"clientTimestamp": now2.Format("2006-01-02T15:04:05.000000Z"),
		"targetVersion":   pull.ToVersion,
		"requestHeader": map[string]any{
			"clientSessionId": fmt.Sprintf("s--%d--vlps-push", now2.UnixMilli()),
			"clientPlatform":  "ANDROID",
			"clientVersion": map[string]any{
				"major": "9", "minor": "9", "build": "9", "revision": "9",
			},
			"capabilities": []any{
				map[string]any{"type": "NC"}, map[string]any{"type": "PI"},
				map[string]any{"type": "LB"}, map[string]any{"type": "AN"},
				map[string]any{"type": "SH"}, map[string]any{"type": "DR"},
				map[string]any{"type": "TR"}, map[string]any{"type": "IN"},
				map[string]any{"type": "SNB"}, map[string]any{"type": "MI"},
				map[string]any{"type": "CO"},
			},
		},
	}
	// userInfo.labels is reserved for CRUD on the user's label set —
	// adding, renaming, deleting. Echoing an *existing* label there
	// caused Keep to 500 in this verifier. Attaching an existing label
	// to a node is just a labelIds-only update on the node itself; no
	// userInfo.labels CRUD entry needed.
	//
	// The production push path (resolveLabelsForTags) only emits a
	// userInfo.labels CRUD entry when the label does NOT already exist
	// — i.e. for a wiki tag that has no Keep label yet. Echoing a label
	// here would make this verifier diverge from the prod shape.
	_ = labelCanonicalName

	bodyBytes, err := json.MarshalIndent(pushBody, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: marshal:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "--- REQUEST BODY ---")
	fmt.Fprintln(os.Stderr, string(bodyBytes))

	// Step 4: send via raw HTTP so we capture the literal status + body.
	status, respBody, err := rawKeepPost(ctx, email, masterToken, deviceID, bodyBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: HTTP send:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "--- RESPONSE STATUS: %d ---\n", status)
	excerpt := respBody
	if len(excerpt) > 500 {
		excerpt = excerpt[:500]
	}
	fmt.Fprintln(os.Stderr, "--- RESPONSE BODY (first 500) ---")
	fmt.Fprintln(os.Stderr, string(excerpt))

	// Step 5: verdict.
	bodyStr := string(respBody)
	hasError := strings.Contains(bodyStr, `"error"`) || strings.Contains(bodyStr, "Unknown Error")
	if sendBrokenShape {
		if status >= 500 || hasError {
			fmt.Fprintln(os.Stderr, "PASS: broken shape (id==serverId) returned 500 / error as predicted by REFERENCE.md")
			fmt.Println("PASS")
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "UNEXPECTED: broken shape did NOT 500 — REFERENCE.md hypothesis may be wrong")
		fmt.Println("UNEXPECTED-PASS")
		os.Exit(1)
	}
	if status == 200 && !hasError {
		fmt.Fprintln(os.Stderr, "PASS: fixed shape (id!=serverId) returned 200, no error")
		fmt.Println("PASS")
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "FAIL: fixed shape rejected (status=%d, hasError=%v)\n", status, hasError)
	fmt.Println("FAIL")
	os.Exit(1)
}

// runVerifyListItemUpdateShape pulls the sandbox list, picks any alive
// LIST_ITEM, pushes a checked-toggle update with the proper shape (id =
// the original client_id from the pull, serverId = server_id,
// parentServerId = sandbox list serverID, baseVersion = "" since Keep
// returns "" for it on every node, deleted = zero, updated = now).
// Confirms the LIST_ITEM update path against the same hypothesis.
func runVerifyListItemUpdateShape(ctx context.Context, email, masterToken, deviceID string, keep *gateway.KeepClient, listServerID string) {
	fmt.Fprintf(os.Stderr, "=== verify-listitem-update-shape parent-id=%s ===\n", listServerID)

	now := time.Now().UTC()
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--vliu-pull", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: pull failed:", err)
		os.Exit(1)
	}

	var target gateway.Node
	for _, n := range pull.Nodes {
		if n.Type != gateway.NodeTypeListItem {
			continue
		}
		if n.ParentID != listServerID && n.ParentServerID != listServerID {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		target = n
		break
	}
	if target.ServerID == "" {
		fmt.Fprintln(os.Stderr, "FAIL: no alive LIST_ITEM under list", listServerID)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ picked target item: client_id=%s serverId=%s text=%q checked=%v baseVersion=%q\n",
		target.ID, target.ServerID, target.Text, target.Checked, target.BaseVersion)

	// Build the push body. Keep returns baseVersion="" on every node we
	// observed in the user's account, so we send "" — testing the
	// hypothesis that BaseVersion isn't required.
	now2 := time.Now().UTC()
	nodeMap := map[string]any{
		"kind":           "notes#node",
		"id":             target.ID, // client_id from the pull, distinct from serverId
		"serverId":       target.ServerID,
		"parentId":       listServerID,
		"parentServerId": listServerID,
		"type":           "LIST_ITEM",
		"text":           target.Text,
		"checked":        !target.Checked, // toggle
		"sortValue":      target.SortValue,
		"annotationsGroup": map[string]any{
			"kind": "notes#annotationsGroup",
		},
		"timestamps": map[string]any{
			"kind":    "notes#timestamps",
			"updated": now2.Format("2006-01-02T15:04:05.000000Z"),
		},
	}
	pushBody := map[string]any{
		"nodes":           []any{nodeMap},
		"clientTimestamp": now2.Format("2006-01-02T15:04:05.000000Z"),
		"targetVersion":   pull.ToVersion,
		"requestHeader": map[string]any{
			"clientSessionId": fmt.Sprintf("s--%d--vliu-push", now2.UnixMilli()),
			"clientPlatform":  "ANDROID",
			"clientVersion": map[string]any{
				"major": "9", "minor": "9", "build": "9", "revision": "9",
			},
			"capabilities": []any{
				map[string]any{"type": "NC"}, map[string]any{"type": "PI"},
				map[string]any{"type": "LB"}, map[string]any{"type": "AN"},
				map[string]any{"type": "SH"}, map[string]any{"type": "DR"},
				map[string]any{"type": "TR"}, map[string]any{"type": "IN"},
				map[string]any{"type": "SNB"}, map[string]any{"type": "MI"},
				map[string]any{"type": "CO"},
			},
		},
	}
	bodyBytes, err := json.MarshalIndent(pushBody, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: marshal:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "--- REQUEST BODY ---")
	fmt.Fprintln(os.Stderr, string(bodyBytes))

	status, respBody, err := rawKeepPost(ctx, email, masterToken, deviceID, bodyBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: HTTP send:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "--- RESPONSE STATUS: %d ---\n", status)
	excerpt := respBody
	if len(excerpt) > 500 {
		excerpt = excerpt[:500]
	}
	fmt.Fprintln(os.Stderr, "--- RESPONSE BODY (first 500) ---")
	fmt.Fprintln(os.Stderr, string(excerpt))

	bodyStr := string(respBody)
	hasError := strings.Contains(bodyStr, `"error"`) || strings.Contains(bodyStr, "Unknown Error")
	if status == 200 && !hasError {
		fmt.Fprintln(os.Stderr, "PASS: LIST_ITEM update with proper shape returned 200")
		fmt.Println("PASS")
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "FAIL: LIST_ITEM update rejected (status=%d, hasError=%v)\n", status, hasError)
	fmt.Println("FAIL")
	os.Exit(1)
}

// runVerifyListPushShapeProdReplay reproduces the EXACT body our
// production push (after the LIST id≠serverId fix) is sending against
// Sandbox to figure out what's still 500-ing. Runs three verifiers
// against the same sandbox list:
//
//	A) Empty annotationsGroup, NO userInfo. (Production replay.)
//	B) Empty annotationsGroup, WITH userInfo.labels echoing the existing
//	   label.
//	C) Annotations array preserved VERBATIM from the pull, NO userInfo.
//
// All three use the labelIds-only update shape (id=client_id,
// serverId=server_id). All three target the SAME `targetVersion`
// captured from a single up-front raw pull, then re-pull between
// pushes to refresh the cursor. Reports per-verifier HTTP status +
// hasError so caller can pinpoint the difference between production
// and the original task #111 verifier.
//
// Goes raw HTTP throughout because the typed protocol layer drops the
// annotations array (gkeepapi treats annotations opaquely; we model
// only the `kind` sentinel) — and we need to re-emit them verbatim
// for verifier C.
func runVerifyListPushShapeProdReplay(ctx context.Context, email, masterToken, deviceID, listServerID, labelMatch string) {
	fmt.Fprintf(os.Stderr, "=== verify-list-push-shape-prod-replay sandbox=%s label=%q ===\n",
		listServerID, labelMatch)

	// Step 1: raw pull so we can capture the LIST node's full
	// annotationsGroup verbatim — the typed decoder discards inner
	// annotations.
	pullStatus, pullBody, err := rawKeepPostBody(ctx, email, masterToken, deviceID, buildPullBody("vlps-prod-replay-pull"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: pull HTTP send:", err)
		os.Exit(1)
	}
	if pullStatus != 200 {
		fmt.Fprintf(os.Stderr, "FAIL: pull returned status=%d\n", pullStatus)
		os.Exit(1)
	}
	var pullParsed map[string]any
	if err := json.Unmarshal(pullBody, &pullParsed); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: pull decode:", err)
		os.Exit(1)
	}
	toVersion, _ := pullParsed["toVersion"].(string)
	if toVersion == "" {
		fmt.Fprintln(os.Stderr, "FAIL: pull missing toVersion")
		os.Exit(1)
	}

	// Find the list node + capture its title + raw annotationsGroup.
	var listClientID, listTitle string
	var rawAnnotationsGroup any
	rawNodes, _ := pullParsed["nodes"].([]any)
	for _, n := range rawNodes {
		m, _ := n.(map[string]any)
		if m == nil {
			continue
		}
		if t, _ := m["type"].(string); t != "LIST" {
			continue
		}
		if sid, _ := m["serverId"].(string); sid != listServerID {
			continue
		}
		listClientID, _ = m["id"].(string)
		listTitle, _ = m["title"].(string)
		rawAnnotationsGroup = m["annotationsGroup"]
		break
	}
	if listClientID == "" {
		fmt.Fprintf(os.Stderr, "FAIL: no LIST node with serverId=%s in pull\n", listServerID)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ found LIST: client_id=%s title=%q\n", listClientID, listTitle)
	if rawAnnotationsGroup != nil {
		agJSON, _ := json.Marshal(rawAnnotationsGroup)
		fmt.Fprintf(os.Stderr, "✓ verbatim annotationsGroup: %s\n", string(agJSON))
	} else {
		fmt.Fprintln(os.Stderr, "(no annotationsGroup on the LIST node in pull)")
	}

	// Find the matching label (case-insensitive) and capture its raw
	// timestamps for verifier B.
	var labelMainID, labelCanonicalName string
	var labelTimestamps any
	if userInfo, ok := pullParsed["userInfo"].(map[string]any); ok {
		labels, _ := userInfo["labels"].([]any)
		want := strings.ToLower(labelMatch)
		for _, l := range labels {
			lm, _ := l.(map[string]any)
			if lm == nil {
				continue
			}
			name, _ := lm["name"].(string)
			if strings.ToLower(name) == want {
				labelMainID, _ = lm["mainId"].(string)
				labelCanonicalName = name
				labelTimestamps = lm["timestamps"]
				break
			}
		}
	}
	if labelMainID == "" {
		fmt.Fprintf(os.Stderr, "FAIL: no label %q on account\n", labelMatch)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ found label: name=%q mainID=%s\n", labelCanonicalName, labelMainID)

	// Helper to build the LIST node body with a chosen annotationsGroup
	// + optional userInfo.
	buildPush := func(annotationsGroup any, includeUserInfo bool, sessionTag string) []byte {
		now := time.Now().UTC()
		nodeMap := map[string]any{
			"kind":     "notes#node",
			"id":       listClientID,
			"serverId": listServerID,
			"type":     "LIST",
			"title":    listTitle,
			"checked":  false,
			"labelIds": []any{
				map[string]any{"labelId": labelMainID},
			},
			"annotationsGroup": annotationsGroup,
			"timestamps": map[string]any{
				"kind":    "notes#timestamps",
				"updated": now.Format("2006-01-02T15:04:05.000000Z"),
			},
		}
		body := map[string]any{
			"nodes":           []any{nodeMap},
			"clientTimestamp": now.Format("2006-01-02T15:04:05.000000Z"),
			"targetVersion":   toVersion,
			"requestHeader": map[string]any{
				"clientSessionId": fmt.Sprintf("s--%d--%s", now.UnixMilli(), sessionTag),
				"clientPlatform":  "ANDROID",
				"clientVersion": map[string]any{
					"major": "9", "minor": "9", "build": "9", "revision": "9",
				},
				"capabilities": []any{
					map[string]any{"type": "NC"}, map[string]any{"type": "PI"},
					map[string]any{"type": "LB"}, map[string]any{"type": "AN"},
					map[string]any{"type": "SH"}, map[string]any{"type": "DR"},
					map[string]any{"type": "TR"}, map[string]any{"type": "IN"},
					map[string]any{"type": "SNB"}, map[string]any{"type": "MI"},
					map[string]any{"type": "CO"},
				},
			},
		}
		if includeUserInfo {
			labelEntry := map[string]any{
				"mainId": labelMainID,
				"name":   labelCanonicalName,
			}
			if labelTimestamps != nil {
				labelEntry["timestamps"] = labelTimestamps
			}
			body["userInfo"] = map[string]any{
				"labels": []any{labelEntry},
			}
		}
		out, _ := json.Marshal(body)
		return out
	}

	// Refresh cursor between pushes — Keep rejects pushes whose
	// targetVersion is stale after a successful prior write. We pull
	// between attempts so each verifier sees a fresh cursor.
	refreshCursor := func(tag string) string {
		s, b, err := rawKeepPostBody(ctx, email, masterToken, deviceID, buildPullBody(tag))
		if err != nil || s != 200 {
			fmt.Fprintf(os.Stderr, "WARN: cursor refresh %s failed (status=%d err=%v)\n", tag, s, err)
			return toVersion // fall back; the push will surface the real error
		}
		var p map[string]any
		_ = json.Unmarshal(b, &p)
		v, _ := p["toVersion"].(string)
		return v
	}

	type result struct {
		name      string
		status    int
		hasError  bool
		body      string
		bodySent  []byte
	}
	results := make([]result, 0, 3)

	// Helper to send and classify a push.
	send := func(name string, payload []byte) result {
		status, respBody, err := rawKeepPost(ctx, email, masterToken, deviceID, payload)
		if err != nil {
			return result{name: name, status: -1, body: "send error: " + err.Error(), bodySent: payload}
		}
		bs := string(respBody)
		hasErr := strings.Contains(bs, `"error"`) || strings.Contains(bs, "Unknown Error")
		excerpt := bs
		if len(excerpt) > 600 {
			excerpt = excerpt[:600]
		}
		return result{name: name, status: status, hasError: hasErr, body: excerpt, bodySent: payload}
	}

	// === Verifier A: empty annotationsGroup, no userInfo (PROD REPLAY) ===
	emptyAG := map[string]any{"kind": "notes#annotationsGroup"}
	rA := send("A: empty AG, no userInfo (PROD REPLAY)", buildPush(emptyAG, false, "verA"))
	results = append(results, rA)

	// Refresh cursor for next attempt (in case A succeeded).
	toVersion = refreshCursor("vlps-prod-replay-pullB")

	// === Verifier B: empty annotationsGroup, WITH userInfo.labels ===
	rB := send("B: empty AG, with userInfo.labels", buildPush(emptyAG, true, "verB"))
	results = append(results, rB)

	// Refresh cursor again for verifier C.
	toVersion = refreshCursor("vlps-prod-replay-pullC")
	// Re-pull to capture the (possibly new) annotationsGroup state in case A or B mutated it.
	_, pullBody2, err2 := rawKeepPostBody(ctx, email, masterToken, deviceID, buildPullBody("vlps-prod-replay-pullC2"))
	if err2 == nil {
		var pp map[string]any
		_ = json.Unmarshal(pullBody2, &pp)
		if v, ok := pp["toVersion"].(string); ok && v != "" {
			toVersion = v
		}
		nodes2, _ := pp["nodes"].([]any)
		for _, n := range nodes2 {
			m, _ := n.(map[string]any)
			if m == nil {
				continue
			}
			if sid, _ := m["serverId"].(string); sid != listServerID {
				continue
			}
			if ag, ok := m["annotationsGroup"]; ok && ag != nil {
				rawAnnotationsGroup = ag
			}
			break
		}
	}

	// === Verifier C: annotations preserved verbatim, no userInfo ===
	verbatimAG := rawAnnotationsGroup
	if verbatimAG == nil {
		verbatimAG = emptyAG
		fmt.Fprintln(os.Stderr, "(verifier C: no verbatim annotations found, falling back to empty)")
	}
	rC := send("C: verbatim AG, no userInfo", buildPush(verbatimAG, false, "verC"))
	results = append(results, rC)

	// === Report ===
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "================ RESULTS ================")
	for _, r := range results {
		verdict := "FAIL"
		if r.status == 200 && !r.hasError {
			verdict = "PASS"
		}
		fmt.Fprintf(os.Stderr, "[%s] %s status=%d hasError=%v\n", verdict, r.name, r.status, r.hasError)
		fmt.Fprintf(os.Stderr, "  body: %s\n", r.body)
	}
	fmt.Fprintln(os.Stderr, "")
	for _, r := range results {
		if r.status == 200 && !r.hasError {
			fmt.Fprintf(os.Stderr, "WORKING REQUEST BODY for [%s]:\n", r.name)
			var pretty any
			if err := json.Unmarshal(r.bodySent, &pretty); err == nil {
				out, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Fprintln(os.Stderr, string(out))
			} else {
				fmt.Fprintln(os.Stderr, string(r.bodySent))
			}
			fmt.Fprintln(os.Stderr, "")
		}
	}
	// Summarize on stdout for caller capture.
	fmt.Println("PROD-REPLAY VERDICT:")
	for _, r := range results {
		v := "FAIL"
		if r.status == 200 && !r.hasError {
			v = "PASS"
		}
		fmt.Printf("  %s | status=%d hasError=%v | %s\n", v, r.status, r.hasError, r.name)
	}
}

// buildPullBody builds a minimal pull-only ChangesRequest body keyed
// by sessionTag. Used by the prod-replay verifier to keep cursor in
// sync between push attempts.
func buildPullBody(sessionTag string) []byte {
	now := time.Now().UTC()
	body := map[string]any{
		"nodes":           []any{},
		"clientTimestamp": now.Format("2006-01-02T15:04:05.000000Z"),
		"requestHeader": map[string]any{
			"clientSessionId": fmt.Sprintf("s--%d--%s", now.UnixMilli(), sessionTag),
			"clientPlatform":  "ANDROID",
			"clientVersion": map[string]any{
				"major": "9", "minor": "9", "build": "9", "revision": "9",
			},
			"capabilities": []any{
				map[string]any{"type": "NC"}, map[string]any{"type": "PI"},
				map[string]any{"type": "LB"}, map[string]any{"type": "AN"},
				map[string]any{"type": "SH"}, map[string]any{"type": "DR"},
				map[string]any{"type": "TR"}, map[string]any{"type": "IN"},
				map[string]any{"type": "SNB"}, map[string]any{"type": "MI"},
				map[string]any{"type": "CO"},
			},
		},
	}
	out, _ := json.Marshal(body)
	return out
}

// runVerifyListPushNoop empirically confirms the production hypothesis:
// Keep returns HTTP 500 stage3 "Unknown Error" when a LIST node update
// is pushed with `labelIds` matching exactly what Keep already has
// (the no-op case), but accepts the same shape with a DIFFERENT label
// set. Pulls the list, captures its current labelIds, then runs both
// the no-op push (expect 500) and the change push (expect 200).
//
// Exits 0 if BOTH expectations hold, 1 otherwise. Stdout: PASS / FAIL.
func runVerifyListPushNoop(ctx context.Context, email, masterToken, deviceID string, keep *gateway.KeepClient, listServerID string) {
	fmt.Fprintf(os.Stderr, "=== verify-list-push-noop parent-id=%s ===\n", listServerID)

	// Step 1: pull and locate the LIST node.
	now := time.Now().UTC()
	pull, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--vlpn-pull", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: pull failed:", err)
		os.Exit(1)
	}
	var listNode gateway.Node
	for _, n := range pull.Nodes {
		if n.Type == gateway.NodeTypeList && n.ServerID == listServerID {
			listNode = n
			break
		}
	}
	if listNode.ServerID == "" {
		fmt.Fprintf(os.Stderr, "FAIL: no LIST node with serverId=%s in pull (got %d nodes)\n",
			listServerID, len(pull.Nodes))
		os.Exit(1)
	}
	currentLabelIDs := append([]string{}, listNode.LabelIDs...)
	fmt.Fprintf(os.Stderr, "✓ found LIST: client_id=%s title=%q current labelIds=%v\n",
		listNode.ID, listNode.Title, currentLabelIDs)

	// Step 2: pick a different label to use for the second push.
	// Choose any label NOT currently on the LIST. If none exist, error.
	var differentLabelID string
	currentSet := map[string]bool{}
	for _, id := range currentLabelIDs {
		currentSet[id] = true
	}
	for _, l := range pull.Labels {
		if l.MainID == "" || !l.Deleted.IsZero() {
			continue
		}
		if !currentSet[l.MainID] {
			differentLabelID = l.MainID
			fmt.Fprintf(os.Stderr, "✓ chose different label: name=%q mainID=%s\n", l.Name, l.MainID)
			break
		}
	}
	if differentLabelID == "" {
		fmt.Fprintln(os.Stderr, "FAIL: no usable alternative label found (need one not on the LIST)")
		os.Exit(1)
	}

	// Step 3: NO-OP push — labelIds matches what Keep already has.
	noopStatus, noopBody := pushListWithLabelIDs(ctx, email, masterToken, deviceID, listNode, listServerID, currentLabelIDs, pull.ToVersion, "noop")
	noopHasError := strings.Contains(string(noopBody), `"error"`) || strings.Contains(string(noopBody), "Unknown Error")
	noop500 := noopStatus >= 500 || noopHasError
	fmt.Fprintf(os.Stderr, "--- NO-OP RESULT: status=%d hasError=%v ---\n", noopStatus, noopHasError)

	// Step 4: re-pull to refresh the cursor for the second push (Keep may
	// have advanced its toVersion as a side-effect of even a failed push).
	now2 := time.Now().UTC()
	pull2, err := keep.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--vlpn-pull2", now2.UnixMilli()),
		ClientTimestamp: now2.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: second pull failed:", err)
		os.Exit(1)
	}

	// Step 5: CHANGE push — labelIds DIFFERENT from current.
	newLabelIDs := append([]string{}, currentLabelIDs...)
	newLabelIDs = append(newLabelIDs, differentLabelID)
	changeStatus, changeBody := pushListWithLabelIDs(ctx, email, masterToken, deviceID, listNode, listServerID, newLabelIDs, pull2.ToVersion, "change")
	changeHasError := strings.Contains(string(changeBody), `"error"`) || strings.Contains(string(changeBody), "Unknown Error")
	change200 := changeStatus == 200 && !changeHasError
	fmt.Fprintf(os.Stderr, "--- CHANGE RESULT: status=%d hasError=%v ---\n", changeStatus, changeHasError)

	// Step 6: verdict.
	if noop500 && change200 {
		fmt.Fprintln(os.Stderr, "PASS: no-op LIST push 500'd as predicted; change push accepted (200)")
		fmt.Println("PASS")
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "FAIL: expected noop=500 change=200, got noop=%d change=%d\n",
		noopStatus, changeStatus)
	fmt.Println("FAIL")
	os.Exit(1)
}

// pushListWithLabelIDs sends a LIST node update with the given labelIds
// using the proven push-shape (id == listNode.ID, serverId == listServerID).
// Returns status code and response body. Used by runVerifyListPushNoop.
func pushListWithLabelIDs(ctx context.Context, email, masterToken, deviceID string, listNode gateway.Node, listServerID string, labelIDs []string, targetVersion, tag string) (int, []byte) {
	now := time.Now().UTC()
	labelIDsBody := make([]any, 0, len(labelIDs))
	for _, id := range labelIDs {
		labelIDsBody = append(labelIDsBody, map[string]any{"labelId": id})
	}
	nodeMap := map[string]any{
		"kind":     "notes#node",
		"id":       listNode.ID,
		"serverId": listServerID,
		"type":     "LIST",
		"title":    listNode.Title,
		"labelIds": labelIDsBody,
		"annotationsGroup": map[string]any{
			"kind": "notes#annotationsGroup",
		},
		"timestamps": map[string]any{
			"kind":    "notes#timestamps",
			"updated": now.Format("2006-01-02T15:04:05.000000Z"),
		},
	}
	pushBody := map[string]any{
		"nodes":           []any{nodeMap},
		"clientTimestamp": now.Format("2006-01-02T15:04:05.000000Z"),
		"targetVersion":   targetVersion,
		"requestHeader": map[string]any{
			"clientSessionId": fmt.Sprintf("s--%d--vlpn-%s", now.UnixMilli(), tag),
			"clientPlatform":  "ANDROID",
			"clientVersion": map[string]any{
				"major": "9", "minor": "9", "build": "9", "revision": "9",
			},
			"capabilities": []any{
				map[string]any{"type": "NC"}, map[string]any{"type": "PI"},
				map[string]any{"type": "LB"}, map[string]any{"type": "AN"},
				map[string]any{"type": "SH"}, map[string]any{"type": "DR"},
				map[string]any{"type": "TR"}, map[string]any{"type": "IN"},
				map[string]any{"type": "SNB"}, map[string]any{"type": "MI"},
				map[string]any{"type": "CO"},
			},
		},
	}
	bodyBytes, err := json.MarshalIndent(pushBody, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: marshal:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "--- %s REQUEST BODY ---\n%s\n", strings.ToUpper(tag), string(bodyBytes))
	status, respBody, err := rawKeepPost(ctx, email, masterToken, deviceID, bodyBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: HTTP send:", err)
		os.Exit(1)
	}
	excerpt := respBody
	if len(excerpt) > 500 {
		excerpt = excerpt[:500]
	}
	fmt.Fprintf(os.Stderr, "--- %s RESPONSE STATUS=%d BODY (first 500) ---\n%s\n",
		strings.ToUpper(tag), status, string(excerpt))
	return status, respBody
}

// diagTestResult holds the outcome of a single A/B/C/D/E diagnostic
// test in runDiagnoseGroceryListPush.
type diagTestResult struct {
	name     string
	addition string
	status   int
	hasError bool
	body     string
	bodySent []byte
}

// runDiagnoseGroceryListPush runs an A/B/C/D/E test matrix against the
// user's actual Grocery LIST in Keep to identify which preserved field
// triggers the production HTTP 500. Each test pulls a FRESH cursor
// before its push so targetVersion is current. All pushes are
// labelIds-only no-ops (the Household labelId Keep already has).
//
// Test matrix:
//
//	A) Exact production replay — minimal body, no preservation.
//	   Expected: 500 (matches production).
//	B) A + baseNoteRevision (likely fix given the 141 revision).
//	C) A + isPinned: true + color: "GREEN".
//	D) A + nodeSettings (preserved verbatim from pull).
//	E) Everything verbatim from pull, only override labelIds and
//	   timestamps.updated. The known-safe fix path.
//
// The Grocery serverId, client_id, and Household labelId are
// hard-coded — this is a one-shot diagnostic for the user's specific
// account.
func runDiagnoseGroceryListPush(ctx context.Context, email, masterToken, deviceID string) {
	const (
		groceryServerID    = "1jUAdi75xdeDenDIftIEY19O1FVmE78Pwigj7jSXNU2PIcrYZzYTDqYciB_rNle7OVrNU"
		groceryClientID    = "19dd617db86.bec795fb133adf7b"
		householdLabelID   = "19dcbc78892.97722f273a1555c5"
	)

	fmt.Fprintf(os.Stderr, "=== diagnose-grocery-list-push grocery=%s ===\n", groceryServerID)

	// Helper: pull and return parsed body + toVersion + the LIST node map.
	// Pulls fresh on every call so each test sees a current cursor.
	pullGrocery := func(tag string) (toVersion string, listNode map[string]any) {
		status, body, err := rawKeepPostBody(ctx, email, masterToken, deviceID, buildPullBody(tag))
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: pull %s: %v\n", tag, err)
			os.Exit(1)
		}
		if status != 200 {
			fmt.Fprintf(os.Stderr, "FAIL: pull %s status=%d body=%s\n", tag, status, truncate(string(body), 500))
			os.Exit(1)
		}
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: pull %s decode: %v\n", tag, err)
			os.Exit(1)
		}
		toVersion, _ = parsed["toVersion"].(string)
		nodes, _ := parsed["nodes"].([]any)
		for _, n := range nodes {
			m, _ := n.(map[string]any)
			if m == nil {
				continue
			}
			if sid, _ := m["serverId"].(string); sid != groceryServerID {
				continue
			}
			if t, _ := m["type"].(string); t != "LIST" {
				continue
			}
			listNode = m
			break
		}
		return toVersion, listNode
	}

	// Initial pull — capture the full LIST node so we can extract
	// preservation fields for each test.
	toVersion, listNode := pullGrocery("diag-grocery-pull-init")
	if listNode == nil {
		fmt.Fprintf(os.Stderr, "FAIL: no LIST node with serverId=%s in initial pull\n", groceryServerID)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ found Grocery LIST: client_id=%v title=%q\n",
		listNode["id"], listNode["title"])

	// Capture the preservation candidates from the pull.
	capturedTitle, _ := listNode["title"].(string)
	capturedBaseNoteRevision := listNode["baseNoteRevision"]
	capturedColor := listNode["color"]
	capturedIsPinned := listNode["isPinned"]
	capturedNodeSettings := listNode["nodeSettings"]

	if capturedTitle == "" {
		capturedTitle = "Grocery"
	}

	// Log captured fields so the diagnostic is auditable.
	fmt.Fprintf(os.Stderr, "  baseNoteRevision: %v\n", capturedBaseNoteRevision)
	fmt.Fprintf(os.Stderr, "  color: %v\n", capturedColor)
	fmt.Fprintf(os.Stderr, "  isPinned: %v\n", capturedIsPinned)
	if capturedNodeSettings != nil {
		nsJSON, _ := json.Marshal(capturedNodeSettings)
		fmt.Fprintf(os.Stderr, "  nodeSettings: %s\n", string(nsJSON))
	} else {
		fmt.Fprintln(os.Stderr, "  nodeSettings: (absent)")
	}

	// buildBaseBody: the exact production-replay shape (Test A).
	// Fields are deliberately minimal to match what the bridge currently
	// sends.
	buildBaseBody := func() map[string]any {
		now := time.Now().UTC()
		return map[string]any{
			"kind":     "notes#node",
			"id":       groceryClientID,
			"serverId": groceryServerID,
			"type":     "LIST",
			"title":    capturedTitle,
			"checked":  false,
			"labelIds": []any{
				map[string]any{"labelId": householdLabelID},
			},
			"annotationsGroup": map[string]any{
				"kind": "notes#annotationsGroup",
			},
			"timestamps": map[string]any{
				"kind":    "notes#timestamps",
				"updated": now.Format("2006-01-02T15:04:05.000000Z"),
			},
		}
	}

	// wrapPushBody wraps a node body in the full ChangesRequest envelope.
	wrapPushBody := func(nodeMap map[string]any, targetVersion, sessionTag string) []byte {
		now := time.Now().UTC()
		body := map[string]any{
			"nodes":           []any{nodeMap},
			"clientTimestamp": now.Format("2006-01-02T15:04:05.000000Z"),
			"targetVersion":   targetVersion,
			"requestHeader": map[string]any{
				"clientSessionId": fmt.Sprintf("s--%d--%s", now.UnixMilli(), sessionTag),
				"clientPlatform":  "ANDROID",
				"clientVersion": map[string]any{
					"major": "9", "minor": "9", "build": "9", "revision": "9",
				},
				"capabilities": []any{
					map[string]any{"type": "NC"}, map[string]any{"type": "PI"},
					map[string]any{"type": "LB"}, map[string]any{"type": "AN"},
					map[string]any{"type": "SH"}, map[string]any{"type": "DR"},
					map[string]any{"type": "TR"}, map[string]any{"type": "IN"},
					map[string]any{"type": "SNB"}, map[string]any{"type": "MI"},
					map[string]any{"type": "CO"},
				},
			},
		}
		out, _ := json.Marshal(body)
		return out
	}

	results := make([]diagTestResult, 0, 5)

	// runTest: refresh cursor, build node body, send, classify.
	runTest := func(name, addition, sessionTag string, mutate func(node map[string]any)) diagTestResult {
		fmt.Fprintf(os.Stderr, "\n--- TEST %s: %s ---\n", name, addition)
		// Fresh pull for this test.
		freshTarget, freshNode := pullGrocery("diag-pull-" + sessionTag)
		if freshTarget == "" {
			fmt.Fprintf(os.Stderr, "WARN: empty toVersion from fresh pull, falling back to initial=%s\n", toVersion)
			freshTarget = toVersion
		}
		_ = freshNode // captured here for tests that need fully-verbatim shape

		nodeMap := buildBaseBody()
		if mutate != nil {
			// For Test E we need access to the FRESH node, not the captured one.
			// Test E mutate function will use freshNode via closure.
			mutate(nodeMap)
		}
		// Special handling: Test E rebuilds completely from freshNode.
		// We detect that by a sentinel marker key set in mutate.
		if _, isVerbatim := nodeMap["__verbatim__"]; isVerbatim {
			delete(nodeMap, "__verbatim__")
			// Rebuild from freshNode preserving every key, override labelIds
			// and timestamps.updated.
			if freshNode != nil {
				verbatim := map[string]any{}
				for k, v := range freshNode {
					verbatim[k] = v
				}
				verbatim["labelIds"] = []any{
					map[string]any{"labelId": householdLabelID},
				}
				now := time.Now().UTC()
				// Preserve timestamps map but override updated.
				ts, _ := verbatim["timestamps"].(map[string]any)
				if ts == nil {
					ts = map[string]any{"kind": "notes#timestamps"}
				}
				newTS := map[string]any{}
				for k, v := range ts {
					newTS[k] = v
				}
				newTS["updated"] = now.Format("2006-01-02T15:04:05.000000Z")
				verbatim["timestamps"] = newTS
				nodeMap = verbatim
			}
		}

		payload := wrapPushBody(nodeMap, freshTarget, sessionTag)

		// Pretty-print the body sent.
		var pretty any
		_ = json.Unmarshal(payload, &pretty)
		prettyBytes, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Fprintf(os.Stderr, "REQUEST BODY:\n%s\n", string(prettyBytes))

		status, respBody, err := rawKeepPost(ctx, email, masterToken, deviceID, payload)
		if err != nil {
			return diagTestResult{name: name, addition: addition, status: -1, body: "send error: " + err.Error(), bodySent: payload}
		}
		bs := string(respBody)
		hasErr := strings.Contains(bs, `"error"`) || strings.Contains(bs, "Unknown Error")
		excerpt := truncate(bs, 500)
		fmt.Fprintf(os.Stderr, "RESPONSE status=%d hasError=%v body=%s\n", status, hasErr, excerpt)
		return diagTestResult{
			name:     name,
			addition: addition,
			status:   status,
			hasError: hasErr,
			body:     excerpt,
			bodySent: payload,
		}
	}

	// === Test A: exact production replay ===
	rA := runTest("A", "(none — prod replay)", "diagA", nil)
	results = append(results, rA)

	// Early-exit guard: if Test A returned 200, the production trigger
	// has shifted since the user reported. Pause and report.
	if rA.status == 200 && !rA.hasError {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "!!! UNEXPECTED: Test A (prod replay) returned 200/no-error !!!")
		fmt.Fprintln(os.Stderr, "Production was reportedly returning 500 with this exact body.")
		fmt.Fprintln(os.Stderr, "Something has changed — pausing diagnostic and re-investigating needed.")
		fmt.Println("UNEXPECTED-A-PASS")
		printDiagResultsTable(results)
		os.Exit(1)
	}

	// === Test B: + baseNoteRevision ===
	rB := runTest("B", fmt.Sprintf("baseNoteRevision: %v", capturedBaseNoteRevision), "diagB", func(node map[string]any) {
		if capturedBaseNoteRevision == nil {
			fmt.Fprintln(os.Stderr, "WARN: pull had no baseNoteRevision — sending empty string")
			node["baseNoteRevision"] = ""
			return
		}
		// Pass through whatever shape Keep sent (string or number).
		node["baseNoteRevision"] = capturedBaseNoteRevision
	})
	results = append(results, rB)

	// === Test C: + isPinned + color ===
	rC := runTest("C", fmt.Sprintf("isPinned: %v, color: %v", capturedIsPinned, capturedColor), "diagC", func(node map[string]any) {
		if capturedIsPinned != nil {
			node["isPinned"] = capturedIsPinned
		} else {
			node["isPinned"] = true
		}
		if capturedColor != nil {
			node["color"] = capturedColor
		} else {
			node["color"] = "GREEN"
		}
	})
	results = append(results, rC)

	// === Test D: + nodeSettings verbatim ===
	rD := runTest("D", "nodeSettings verbatim", "diagD", func(node map[string]any) {
		if capturedNodeSettings == nil {
			fmt.Fprintln(os.Stderr, "WARN: pull had no nodeSettings — Test D effectively duplicates Test A")
			return
		}
		node["nodeSettings"] = capturedNodeSettings
	})
	results = append(results, rD)

	// === Test E: everything verbatim, override labelIds + timestamps.updated ===
	rE := runTest("E", "everything verbatim from pull", "diagE", func(node map[string]any) {
		// Sentinel: runTest sees this key and rebuilds nodeMap from freshNode.
		node["__verbatim__"] = true
	})
	results = append(results, rE)

	// === Final report ===
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "================ DIAGNOSTIC RESULTS ================")
	printDiagResultsTable(results)
}

// truncate cuts a string to at most n bytes, returning the prefix.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// printDiagResultsTable prints the A/B/C/D/E test results as a table on
// stdout (so callers piping to file capture the verdict) and stderr.
func printDiagResultsTable(results []diagTestResult) {
	header := fmt.Sprintf("%-4s | %-50s | %-6s | %-8s | %s", "Test", "Body addition", "Status", "hasError", "Verdict")
	fmt.Fprintln(os.Stderr, header)
	fmt.Println(header)
	fmt.Fprintln(os.Stderr, strings.Repeat("-", len(header)))
	fmt.Println(strings.Repeat("-", len(header)))
	for _, r := range results {
		verdict := "FAIL"
		if r.status == 200 && !r.hasError {
			verdict = "PASS"
		}
		line := fmt.Sprintf("%-4s | %-50s | %-6d | %-8t | %s", r.name, truncate(r.addition, 50), r.status, r.hasError, verdict)
		fmt.Fprintln(os.Stderr, line)
		fmt.Println(line)
	}
}

// rawKeepPostBody is an alias for rawKeepPost — naming kept for
// readability at the verifier-level call sites where we're sending a
// pull, not a push.
func rawKeepPostBody(ctx context.Context, email, masterToken, deviceID string, body []byte) (int, []byte, error) {
	return rawKeepPost(ctx, email, masterToken, deviceID, body)
}

// rawKeepPost re-authenticates and posts a body to /changes via raw
// HTTP, returning the status code and response body. Necessary for the
// verifiers because KeepClient.Changes wraps non-200s in classified
// errors that lose the body — we want the actual server response so
// the verdict logic can see whether Keep complained about content.
func rawKeepPost(ctx context.Context, email, masterToken, deviceID string, body []byte) (int, []byte, error) {
	authClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}},
			ForceAttemptHTTP2: false,
		},
		Timeout: 30 * time.Second,
	}
	auth := gateway.NewAuthenticator(authClient, gateway.AuthURL, deviceID)
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
	if err != nil {
		return 0, nil, fmt.Errorf("auth: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gateway.DefaultKeepBaseURL+"changes", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "OAuth "+bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "simple_wiki-keep-debug/1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}
