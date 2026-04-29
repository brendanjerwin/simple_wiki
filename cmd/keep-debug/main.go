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

	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

func main() {
	cmd := flag.String("cmd", "list", "list | create-and-push | create-with-items | push-item-to-existing | dump-items | dump-write-results | verify-cursor-monotonic")
	title := flag.String("title", "Keep CLI Test", "title for create-and-push / create-with-items")
	itemsCSV := flag.String("items", "Eggs,Milk,Bread", "comma-separated items for create-with-items")
	parentID := flag.String("parent-id", "", "for push-item-to-existing: the LIST node's serverID")
	itemText := flag.String("item-text", "Late Add", "for push-item-to-existing: the new item's text")
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
	auth := protocol.NewAuthenticator(authClient, protocol.AuthURL, deviceID)
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Stage 2 failed:", err)
		os.Exit(1)
	}
	fmt.Println("✓ bearer obtained, len:", len(bearer))

	keep := protocol.NewKeepClient(http.DefaultClient, protocol.DefaultKeepBaseURL, bearer)

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
	default:
		fmt.Fprintln(os.Stderr, "unknown cmd:", *cmd)
		os.Exit(2)
	}
}

// runUpdateItemMatching: pull, find ALL LIST_ITEMs under the given
// list whose text contains the substring `match` (or all alive items
// if match is empty), then push them back as UPDATES in a single
// request — reproduces the cron-tick shape.
func runUpdateItemMatching(ctx context.Context, keep *protocol.KeepClient, listServerID, match string) {
	pull, err := keep.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--upd-pull", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "pull failed:", err)
		os.Exit(1)
	}
	var targets []protocol.Node
	for _, n := range pull.Nodes {
		if n.Type != protocol.NodeTypeListItem {
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
	pushNodes := make([]protocol.Node, 0, len(targets))
	for i, t := range targets {
		_ = i
		pushNodes = append(pushNodes, protocol.Node{
			Kind:           "notes#node",
			ID:             t.ID,
			ServerID:       t.ServerID,
			ParentID:       listServerID,
			ParentServerID: listServerID,
			Type:           protocol.NodeTypeListItem,
			Text:           t.Text + " edit",
			Checked:        t.Checked,
			SortValue:      t.SortValue,
			BaseVersion:    t.BaseVersion,
			Timestamps:     protocol.Timestamps{Updated: now},
		})
	}
	// Wire-shape debug logger: prints the marshaled wire body so we
	// see exactly what Keep gets.
	keep.SetDebugLogger(stderrDebug{})
	resp, err := keep.Changes(ctx, protocol.ChangesRequest{
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
func runTrashOne(ctx context.Context, keep *protocol.KeepClient, listServerID, match string) {
	pull, err := keep.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--trash-pull", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "pull:", err); os.Exit(1) }
	var target protocol.Node
	for _, n := range pull.Nodes {
		if n.Type != protocol.NodeTypeListItem { continue }
		if n.ParentID != listServerID && n.ParentServerID != listServerID { continue }
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() { continue }
		if match != "" && !strings.Contains(n.Text, match) { continue }
		target = n; break
	}
	if target.ServerID == "" { fmt.Fprintln(os.Stderr, "no alive item match"); os.Exit(1) }
	now := time.Now().UTC()
	keep.SetDebugLogger(stderrDebug{})
	resp, err := keep.Changes(ctx, protocol.ChangesRequest{
		Nodes: []protocol.Node{{
			Kind: "notes#node",
			ID: target.ID,
			ServerID: target.ServerID,
			ParentID: listServerID,
			ParentServerID: listServerID,
			Type: protocol.NodeTypeListItem,
			Text: target.Text,         // include
			Checked: target.Checked,   // include
			SortValue: target.SortValue, // include
			BaseVersion: target.BaseVersion,
			Timestamps: protocol.Timestamps{
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
func runUpdateAndTrash(ctx context.Context, keep *protocol.KeepClient, listServerID string) {
	pull, err := keep.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--mix-pull", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "pull:", err); os.Exit(1) }
	var alive []protocol.Node
	for _, n := range pull.Nodes {
		if n.Type != protocol.NodeTypeListItem { continue }
		if n.ParentID != listServerID && n.ParentServerID != listServerID { continue }
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() { continue }
		alive = append(alive, n)
		if len(alive) >= 2 { break }
	}
	if len(alive) < 2 { fmt.Fprintln(os.Stderr, "need >= 2 alive items"); os.Exit(1) }
	now := time.Now().UTC()
	keep.SetDebugLogger(stderrDebug{})
	resp, err := keep.Changes(ctx, protocol.ChangesRequest{
		Nodes: []protocol.Node{
			{ // update
				Kind: "notes#node", ID: alive[0].ID, ServerID: alive[0].ServerID,
				ParentID: listServerID, ParentServerID: listServerID,
				Type: protocol.NodeTypeListItem, Text: alive[0].Text + " edit",
				Checked: alive[0].Checked, SortValue: alive[0].SortValue,
				BaseVersion: alive[0].BaseVersion,
				Timestamps: protocol.Timestamps{Updated: now},
			},
			{ // delete (using Deleted, not Trashed)
				Kind: "notes#node", ID: alive[1].ID, ServerID: alive[1].ServerID,
				ParentID: listServerID, ParentServerID: listServerID,
				Type: protocol.NodeTypeListItem,
				BaseVersion: alive[1].BaseVersion,
				Timestamps: protocol.Timestamps{Updated: now, Deleted: now},
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
func runDeleteMany(ctx context.Context, keep *protocol.KeepClient, listServerID string) {
	pull, err := keep.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--del-pull", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil { fmt.Fprintln(os.Stderr, "pull:", err); os.Exit(1) }
	var alive []protocol.Node
	for _, n := range pull.Nodes {
		if n.Type != protocol.NodeTypeListItem { continue }
		if n.ParentID != listServerID && n.ParentServerID != listServerID { continue }
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() { continue }
		alive = append(alive, n)
	}
	if len(alive) == 0 { fmt.Fprintln(os.Stderr, "no alive items"); os.Exit(1) }
	fmt.Printf("deleting %d items\n", len(alive))
	now := time.Now().UTC()
	pushNodes := make([]protocol.Node, 0, len(alive))
	for _, t := range alive {
		pushNodes = append(pushNodes, protocol.Node{
			Kind: "notes#node", ID: t.ID, ServerID: t.ServerID,
			ParentID: listServerID, ParentServerID: listServerID,
			Type: protocol.NodeTypeListItem,
			BaseVersion: t.BaseVersion,
			Timestamps: protocol.Timestamps{Updated: now, Deleted: now},
		})
	}
	keep.SetDebugLogger(stderrDebug{})
	resp, err := keep.Changes(ctx, protocol.ChangesRequest{
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
	auth := protocol.NewAuthenticator(authClient, protocol.AuthURL, deviceID)
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
	if err != nil {
		fmt.Fprintln(os.Stderr, "auth failed:", err)
		os.Exit(1)
	}

	body := fmt.Sprintf(`{"nodes":[],"clientTimestamp":%q,"requestHeader":{"clientSessionId":"raw-pull","clientPlatform":"ANDROID","clientVersion":{"major":"9","minor":"9","build":"9","revision":"9"},"capabilities":[{"type":"NC"},{"type":"PI"},{"type":"LB"},{"type":"AN"},{"type":"SH"},{"type":"DR"},{"type":"TR"},{"type":"IN"},{"type":"SNB"},{"type":"MI"},{"type":"CO"}]}}`,
		time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"))
	req, _ := http.NewRequestWithContext(ctx, "POST", protocol.DefaultKeepBaseURL+"changes", strings.NewReader(body))
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
func runDumpWriteResults(ctx context.Context, email, masterToken, deviceID string, keep *protocol.KeepClient) {
	// Step 1: pull to capture toVersion. We use the typed client here
	// because we just need the cursor; the response body itself is
	// uninteresting for this diagnostic.
	now := time.Now().UTC()
	pull, err := keep.Changes(ctx, protocol.ChangesRequest{
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
	auth := protocol.NewAuthenticator(authClient, protocol.AuthURL, deviceID)
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
		protocol.DefaultKeepBaseURL+"changes", strings.NewReader(string(body)))
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
func runVerifyCursorMonotonic(ctx context.Context, keep *protocol.KeepClient) {
	const numPulls = 6
	const interPullDelay = 1500 * time.Millisecond

	versions := make([]string, 0, numPulls)
	prevTarget := ""
	for i := 0; i < numPulls; i++ {
		now := time.Now().UTC()
		req := protocol.ChangesRequest{
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

func runDumpItems(ctx context.Context, keep *protocol.KeepClient, listServerID string) {
	resp, err := keep.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--dump", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "dump pull failed:", err)
		os.Exit(1)
	}
	for _, n := range resp.Nodes {
		if n.Type != protocol.NodeTypeListItem {
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

func runPushItemToExisting(ctx context.Context, keep *protocol.KeepClient, parentServerID, text string) {
	now := time.Now().UTC()

	// Step 1: pull to get the latest toVersion. gkeepapi follows a strict
	// sync-then-push pattern; pushing with empty TargetVersion gets a 500
	// "Unknown Error" because the server can't reconcile the partial
	// node update against an unknown client baseline.
	pull, err := keep.Changes(ctx, protocol.ChangesRequest{
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
	resp, err := keep.Changes(ctx, protocol.ChangesRequest{
		Nodes: []protocol.Node{
			{
				Kind:           "notes#node",
				ID:             itemClientID,
				Type:           protocol.NodeTypeListItem,
				ParentID:       parentServerID,
				ParentServerID: parentServerID,
				Text:           text,
				SortValue:      "1000",
				Timestamps: protocol.Timestamps{
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
		if n.ID == itemClientID && n.Type == protocol.NodeTypeListItem {
			fmt.Printf("✓ item created on existing list: %s -> %s\n", itemClientID, n.ServerID)
			return
		}
	}
	fmt.Println("✗ item NOT echoed back; full response nodes:")
	for _, n := range resp.Nodes {
		if n.Type == protocol.NodeTypeListItem {
			fmt.Printf("  id=%s server=%s parent=%s text=%q\n", n.ID, n.ServerID, n.ParentID, n.Text)
		}
	}
}

func runCreateWithItems(ctx context.Context, keep *protocol.KeepClient, title string, items []string) {
	specs := make([]protocol.ListItemSpec, len(items))
	for i, txt := range items {
		specs[i] = protocol.ListItemSpec{
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

func runList(ctx context.Context, keep *protocol.KeepClient) {
	req := protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--cli", time.Now().UnixMilli()),
		ClientTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	}
	resp, err := keep.Changes(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Changes failed:", err)
		os.Exit(1)
	}
	fmt.Println("✓ Changes returned", len(resp.Nodes), "nodes; toVersion:", resp.ToVersion, "truncated:", resp.Truncated)
	byType := map[protocol.NodeType]int{}
	var lists []protocol.Node
	for _, n := range resp.Nodes {
		byType[n.Type]++
		if n.Type == protocol.NodeTypeList {
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

func runCreateAndPush(ctx context.Context, keep *protocol.KeepClient, title string) {
	id, err := keep.CreateList(ctx, title)
	if err != nil {
		fmt.Fprintln(os.Stderr, "CreateList failed:", err)
		os.Exit(1)
	}
	fmt.Println("✓ CreateList returned serverID:", id)

	// Now do a pull to confirm the list shows up.
	resp, err := keep.Changes(ctx, protocol.ChangesRequest{
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
		if n.Type == protocol.NodeTypeList {
			fmt.Printf("  serverID=%s title=%q\n", n.ServerID, n.Title)
		}
	}
}
