package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

func main() {
	cmd := flag.String("cmd", "list", "list | create-and-push | create-with-items | push-item-to-existing")
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
	default:
		fmt.Fprintln(os.Stderr, "unknown cmd:", *cmd)
		os.Exit(2)
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
	fmt.Println("ALIVE LIST nodes:")
	for _, n := range lists {
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		fmt.Printf("  serverID=%s title=%q\n", n.ServerID, n.Title)
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
