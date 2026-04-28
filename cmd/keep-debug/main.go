package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
)

func main() {
	cmd := flag.String("cmd", "list", "list | create-and-push")
	title := flag.String("title", "Keep CLI Test", "title for create-and-push")
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
	default:
		fmt.Fprintln(os.Stderr, "unknown cmd:", *cmd)
		os.Exit(2)
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
	fmt.Println("LIST nodes (top 10):")
	for i, n := range lists {
		if i >= 10 {
			break
		}
		fmt.Printf("  serverID=%s title=%q text=%q trashed=%v deleted=%v\n",
			n.ServerID, n.Title, n.Text, !n.Timestamps.Trashed.IsZero(), !n.Timestamps.Deleted.IsZero())
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
