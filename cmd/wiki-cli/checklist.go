package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	cli "gopkg.in/urfave/cli.v1"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/gen/go/api/v1/apiv1connect"
)

// Argument-count constants for the checklist subcommands. Named so the
// expected shape of each subcommand is greppable from one spot.
const (
	checklistAddMinArgs     = 3
	checklistUpdateMinArgs  = 4
	checklistReorderMinArgs = 4
)

// pageListUIDUsage is the canonical "<page> <list> <uid>" string used in
// usage messages. Defining once avoids "string literal appears N times"
// lint complaints and keeps wording consistent.
const pageListUIDUsage = "<page> <list> <uid>"

// buildChecklistCommand creates the `checklist` subcommand tree wrapping
// the gRPC ChecklistService. The dedicated subcommand is for human use
// at the terminal — programmatic callers should use the MCP tool or the
// generic `wiki-cli call` interface.
func buildChecklistCommand(urlFlag cli.StringFlag) cli.Command {
	return cli.Command{
		Name:  "checklist",
		Usage: "Manage checklists on a wiki page (list/add/toggle/update/delete/reorder)",
		Flags: []cli.Flag{urlFlag},
		Subcommands: []cli.Command{
			{
				Name:      "list",
				Usage:     "List items on a checklist",
				ArgsUsage: "<page> <list>",
				Flags:     []cli.Flag{urlFlag},
				Action:    runChecklistList,
			},
			{
				Name:      "add",
				Usage:     "Append an item to a checklist",
				ArgsUsage: "<page> <list> <text...>",
				Flags:     []cli.Flag{urlFlag},
				Action:    runChecklistAdd,
			},
			{
				Name:      "toggle",
				Usage:     "Flip an item's checked state",
				ArgsUsage: pageListUIDUsage,
				Flags:     []cli.Flag{urlFlag},
				Action:    runChecklistToggle,
			},
			{
				Name:      "update",
				Usage:     "Update item text",
				ArgsUsage: pageListUIDUsage + " <text...>",
				Flags:     []cli.Flag{urlFlag},
				Action:    runChecklistUpdate,
			},
			{
				Name:      "delete",
				Usage:     "Remove an item",
				ArgsUsage: pageListUIDUsage,
				Flags:     []cli.Flag{urlFlag},
				Action:    runChecklistDelete,
			},
			{
				Name:      "reorder",
				Usage:     "Update an item's sort_order",
				ArgsUsage: pageListUIDUsage + " <new_sort_order>",
				Flags:     []cli.Flag{urlFlag},
				Action:    runChecklistReorder,
			},
		},
	}
}

func newChecklistClient(c *cli.Context) (apiv1connect.ChecklistServiceClient, error) {
	baseURL, err := normalizeBaseURL(c.GlobalString("url"))
	if err != nil {
		baseURL, err = normalizeBaseURL(c.String("url"))
		if err != nil {
			return nil, err
		}
	}
	return apiv1connect.NewChecklistServiceClient(newAgentAwareHTTPClient(nil), baseURL), nil
}

func runChecklistList(c *cli.Context) error {
	page, listName, err := requireArgs(c, 2, "list", "<page> <list>")
	if err != nil {
		return err
	}
	client, err := newChecklistClient(c)
	if err != nil {
		return err
	}
	resp, err := client.ListItems(context.Background(), connect.NewRequest(&apiv1.ListItemsRequest{
		Page: page, ListName: listName,
	}))
	if err != nil {
		return fmt.Errorf("ListItems: %w", err)
	}
	return printJSON(resp.Msg.GetChecklist())
}

func runChecklistAdd(c *cli.Context) error {
	args := c.Args()
	if len(args) < checklistAddMinArgs {
		return errors.New("usage: checklist add <page> <list> <text...>")
	}
	page, listName := args[0], args[1]
	text := strings.Join(args[2:], " ")
	client, err := newChecklistClient(c)
	if err != nil {
		return err
	}
	resp, err := client.AddItem(context.Background(), connect.NewRequest(&apiv1.AddItemRequest{
		Page: page, ListName: listName, Text: text,
	}))
	if err != nil {
		return fmt.Errorf("AddItem: %w", err)
	}
	return printJSON(resp.Msg.GetItem())
}

func runChecklistToggle(c *cli.Context) error {
	parsed, err := requireThreeArgs(c, "toggle", pageListUIDUsage)
	if err != nil {
		return err
	}
	client, err := newChecklistClient(c)
	if err != nil {
		return err
	}
	resp, err := client.ToggleItem(context.Background(), connect.NewRequest(&apiv1.ToggleItemRequest{
		Page: parsed.page, ListName: parsed.listName, Uid: parsed.uid,
	}))
	if err != nil {
		return fmt.Errorf("ToggleItem: %w", err)
	}
	return printJSON(resp.Msg.GetItem())
}

func runChecklistUpdate(c *cli.Context) error {
	args := c.Args()
	if len(args) < checklistUpdateMinArgs {
		return errors.New("usage: checklist update <page> <list> <uid> <text...>")
	}
	page, listName, uid := args[0], args[1], args[2]
	text := strings.Join(args[3:], " ")
	client, err := newChecklistClient(c)
	if err != nil {
		return err
	}
	resp, err := client.UpdateItem(context.Background(), connect.NewRequest(&apiv1.UpdateItemRequest{
		Page: page, ListName: listName, Uid: uid, Text: &text,
	}))
	if err != nil {
		return fmt.Errorf("UpdateItem: %w", err)
	}
	return printJSON(resp.Msg.GetItem())
}

func runChecklistDelete(c *cli.Context) error {
	parsed, err := requireThreeArgs(c, "delete", pageListUIDUsage)
	if err != nil {
		return err
	}
	client, err := newChecklistClient(c)
	if err != nil {
		return err
	}
	resp, err := client.DeleteItem(context.Background(), connect.NewRequest(&apiv1.DeleteItemRequest{
		Page: parsed.page, ListName: parsed.listName, Uid: parsed.uid,
	}))
	if err != nil {
		return fmt.Errorf("DeleteItem: %w", err)
	}
	return printJSON(resp.Msg.GetChecklist())
}

func runChecklistReorder(c *cli.Context) error {
	args := c.Args()
	if len(args) < checklistReorderMinArgs {
		return errors.New("usage: checklist reorder <page> <list> <uid> <new_sort_order>")
	}
	page, listName, uid := args[0], args[1], args[2]
	var newOrder int64
	if _, err := fmt.Sscanf(args[3], "%d", &newOrder); err != nil {
		return fmt.Errorf("invalid new_sort_order %q: %w", args[3], err)
	}
	client, err := newChecklistClient(c)
	if err != nil {
		return err
	}
	resp, err := client.ReorderItem(context.Background(), connect.NewRequest(&apiv1.ReorderItemRequest{
		Page: page, ListName: listName, Uid: uid, NewSortOrder: newOrder,
	}))
	if err != nil {
		return fmt.Errorf("ReorderItem: %w", err)
	}
	return printJSON(resp.Msg.GetChecklist())
}

// requireArgs returns the first two args and a wrapped usage error when fewer
// than `n` are supplied. (Used for the two-arg subcommands; the three-arg
// helpers below do similar work.)
func requireArgs(c *cli.Context, n int, name, usage string) (page, listName string, err error) {
	args := c.Args()
	if len(args) < n {
		return "", "", fmt.Errorf("usage: checklist %s %s", name, usage)
	}
	return args[0], args[1], nil
}

// pageListUIDArgs captures the (page, list, uid) triple parsed from a
// CLI argv. Returning a struct lets requireThreeArgs stay under the
// linter's max-results-per-function limit.
type pageListUIDArgs struct {
	page, listName, uid string
}

func requireThreeArgs(c *cli.Context, name, usage string) (pageListUIDArgs, error) {
	args := c.Args()
	const minArgs = 3
	if len(args) < minArgs {
		return pageListUIDArgs{}, fmt.Errorf("usage: checklist %s %s", name, usage)
	}
	return pageListUIDArgs{page: args[0], listName: args[1], uid: args[2]}, nil
}

// printJSON marshals a proto message to indented JSON on stdout.
func printJSON(m proto.Message) error {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(m)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, string(b))
	return err
}
