//revive:disable:dot-imports
package main

import (
	"flag"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "gopkg.in/urfave/cli.v1"
)

// newRunCLIContext mirrors the urfave/cli wiring for a subcommand:
// parent context carries the global --url; child context carries the
// subcommand's positional args and its own --url.
func newRunCLIContext(urlValue string, args ...string) *cli.Context {
	app := cli.NewApp()
	parent := flag.NewFlagSet("test", flag.ContinueOnError)
	parent.String("url", urlValue, "")
	_ = parent.Parse(nil)
	parentCtx := cli.NewContext(app, parent, nil)
	sub := flag.NewFlagSet("sub", flag.ContinueOnError)
	sub.String("url", urlValue, "")
	_ = sub.Parse(args)
	return cli.NewContext(app, sub, parentCtx)
}

var _ = Describe("runChecklist* subcommand argument validation", func() {
	var server *httptest.Server

	BeforeEach(func() {
		// httptest server we never actually hit — args validation must
		// short-circuit before any HTTP request would be made.
		server = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	})

	AfterEach(func() {
		server.Close()
	})

	When("runChecklistList is invoked with too few args", func() {
		var err error

		BeforeEach(func() {
			ctx := newRunCLIContext(server.URL, "shopping")
			err = runChecklistList(ctx)
		})

		It("should return a usage error mentioning the subcommand", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("usage: checklist list"))
		})
	})

	When("runChecklistAdd is invoked with too few args", func() {
		var err error

		BeforeEach(func() {
			ctx := newRunCLIContext(server.URL, "shopping", "groceries")
			err = runChecklistAdd(ctx)
		})

		It("should return a usage error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("usage: checklist add"))
		})
	})

	When("runChecklistToggle is invoked with too few args", func() {
		var err error

		BeforeEach(func() {
			ctx := newRunCLIContext(server.URL, "shopping", "groceries")
			err = runChecklistToggle(ctx)
		})

		It("should return a usage error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("usage: checklist toggle"))
		})
	})

	When("runChecklistUpdate is invoked with too few args", func() {
		var err error

		BeforeEach(func() {
			ctx := newRunCLIContext(server.URL, "shopping", "groceries", "uid")
			err = runChecklistUpdate(ctx)
		})

		It("should return a usage error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("usage: checklist update"))
		})
	})

	When("runChecklistDelete is invoked with too few args", func() {
		var err error

		BeforeEach(func() {
			ctx := newRunCLIContext(server.URL, "shopping", "groceries")
			err = runChecklistDelete(ctx)
		})

		It("should return a usage error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("usage: checklist delete"))
		})
	})

	When("runChecklistReorder is invoked with too few args", func() {
		var err error

		BeforeEach(func() {
			ctx := newRunCLIContext(server.URL, "shopping", "groceries", "uid")
			err = runChecklistReorder(ctx)
		})

		It("should return a usage error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("usage: checklist reorder"))
		})
	})

	When("runChecklistReorder is invoked with a non-numeric sort_order", func() {
		var err error

		BeforeEach(func() {
			ctx := newRunCLIContext(server.URL, "shopping", "groceries", "uid", "not-a-number")
			err = runChecklistReorder(ctx)
		})

		It("should return a parse error naming the bad value", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid new_sort_order"))
		})
	})
})

var _ = Describe("buildChecklistCommand", func() {
	var cmd cli.Command

	BeforeEach(func() {
		urlFlag := cli.StringFlag{Name: "url", Value: "http://wiki.example"}
		cmd = buildChecklistCommand(urlFlag)
	})

	It("should declare 'checklist' as the top-level command name", func() {
		Expect(cmd.Name).To(Equal("checklist"))
	})

	It("should declare exactly six subcommands", func() {
		names := make([]string, 0, len(cmd.Subcommands))
		for _, s := range cmd.Subcommands {
			names = append(names, s.Name)
		}
		Expect(names).To(ConsistOf("list", "add", "toggle", "update", "delete", "reorder"))
	})

	It("should reuse pageListUIDUsage in the three-arg subcommands", func() {
		for _, s := range cmd.Subcommands {
			switch s.Name {
			case "toggle", "delete":
				Expect(s.ArgsUsage).To(Equal(pageListUIDUsage))
			case "update":
				Expect(s.ArgsUsage).To(ContainSubstring(pageListUIDUsage))
				Expect(s.ArgsUsage).To(ContainSubstring("<text..."))
			case "reorder":
				Expect(s.ArgsUsage).To(ContainSubstring(pageListUIDUsage))
				Expect(s.ArgsUsage).To(ContainSubstring("<new_sort_order>"))
			default:
				// Two-arg subcommands (list, add) — usage doesn't include
				// pageListUIDUsage, so nothing to assert here.
			}
		}
	})
})

var _ = Describe("runChecklist* against a closed server (RPC error path)", func() {
	When("runChecklistList hits a closed Connect endpoint", func() {
		var err error

		BeforeEach(func() {
			closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			closed.Close()
			ctx := newRunCLIContext(closed.URL, "shopping", "groceries")
			err = runChecklistList(ctx)
		})

		It("should propagate the transport error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ListItems"))
		})
	})

	When("runChecklistAdd hits a closed Connect endpoint", func() {
		var err error

		BeforeEach(func() {
			closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			closed.Close()
			ctx := newRunCLIContext(closed.URL, "shopping", "groceries", "buy", "milk")
			err = runChecklistAdd(ctx)
		})

		It("should propagate the transport error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("AddItem"))
		})
	})

	When("runChecklistToggle hits a closed Connect endpoint", func() {
		var err error

		BeforeEach(func() {
			closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			closed.Close()
			ctx := newRunCLIContext(closed.URL, "shopping", "groceries", "uid-xxx")
			err = runChecklistToggle(ctx)
		})

		It("should propagate the transport error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ToggleItem"))
		})
	})

	When("runChecklistUpdate hits a closed Connect endpoint", func() {
		var err error

		BeforeEach(func() {
			closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			closed.Close()
			ctx := newRunCLIContext(closed.URL, "shopping", "groceries", "uid-xxx", "new", "text")
			err = runChecklistUpdate(ctx)
		})

		It("should propagate the transport error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("UpdateItem"))
		})
	})

	When("runChecklistDelete hits a closed Connect endpoint", func() {
		var err error

		BeforeEach(func() {
			closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			closed.Close()
			ctx := newRunCLIContext(closed.URL, "shopping", "groceries", "uid-xxx")
			err = runChecklistDelete(ctx)
		})

		It("should propagate the transport error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("DeleteItem"))
		})
	})

	When("runChecklistReorder hits a closed Connect endpoint", func() {
		var err error

		BeforeEach(func() {
			closed := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			closed.Close()
			ctx := newRunCLIContext(closed.URL, "shopping", "groceries", "uid-xxx", "1500")
			err = runChecklistReorder(ctx)
		})

		It("should propagate the transport error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ReorderItem"))
		})
	})
})
