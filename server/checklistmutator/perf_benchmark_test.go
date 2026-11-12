// Package checklistmutator_test contains benchmarks that profile the
// server-side cost of checklist item mutations. These benchmarks exist to
// support issue #1409 and must stay runnable as regression guards.
//
// Run with:
//
//	go test -bench=BenchmarkChecklist -benchtime=5s -run=^$ ./server/checklistmutator/
package checklistmutator_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// benchStoreWrapper adds the ReadMarkdown and DeletePage methods that
// PageReaderMutator requires but *pagestore.Store doesn't expose directly.
type benchStoreWrapper struct {
	*pagestore.Store
}

func (w *benchStoreWrapper) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	page, err := w.Store.ReadPage(id)
	if err != nil {
		return id, "", err
	}
	md, err := page.GetMarkdown()
	if err != nil {
		return id, "", err
	}
	return id, md, nil
}

func (*benchStoreWrapper) DeletePage(_ wikipage.PageIdentifier) error { return nil }

// buildSyntheticChecklistFM returns a frontmatter map whose
// wiki.checklists.Grocery subtree has numItems items and numEvents events.
// Writing this map directly to the page store lets benchmarks start with a
// realistically large page without paying the O(N²) cost of building it
// through the mutator one item at a time.
func buildSyntheticChecklistFM(numItems, numEvents int) wikipage.FrontMatter {
	items := make([]any, numItems)
	for i := range numItems {
		items[i] = map[string]any{
			"uid":        fmt.Sprintf("01JBENCH%08X", i),
			"text":       fmt.Sprintf("Item %d", i+1),
			"checked":    false,
			"sort_order": int64((i + 1) * 1000),
			"automated":  false,
			"created_at": "2026-01-01T00:00:00Z",
			"updated_at": "2026-01-01T00:00:00Z",
		}
	}

	events := make([]any, numEvents)
	for i := range numEvents {
		events[i] = map[string]any{
			"seq": int64(i + 1),
			"src": "user:alice@example.com",
			"op":  "add",
			"uid": fmt.Sprintf("01JBENCH%08X", i%numItems),
		}
	}

	return wikipage.FrontMatter{
		"wiki": map[string]any{
			"checklists": map[string]any{
				"Grocery": map[string]any{
					"sync_token": int64(numEvents),
					"max_seq":    int64(numEvents),
					"updated_at": "2026-01-01T00:00:00Z",
					"items":      items,
					"events":     events,
				},
			},
		},
	}
}

// newBenchFixture creates a temp-dir page store populated with a checklist page
// whose frontmatter has numItems items and numEvents events. Returns the mutator,
// page name, human identity, and a cleanup function.
func newBenchFixture(b *testing.B, numItems, numEvents int) (
	*checklistmutator.Mutator, string, tailscale.IdentityValue, func(),
) {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "checklist-bench-")
	if err != nil {
		b.Fatalf("TempDir: %v", err)
	}

	rawStore := pagestore.NewStore(tempDir)
	store := &benchStoreWrapper{rawStore}
	mutator := checklistmutator.New(store, checklistmutator.SystemClock{}, ulid.NewSystemGenerator())

	const pageName = "bench_page"

	fm := buildSyntheticChecklistFM(numItems, numEvents)
	pageText, err := wikipage.CombineFrontMatterAndMarkdown(fm, "# Benchmark page\n")
	if err != nil {
		b.Fatalf("CombineFrontMatterAndMarkdown: %v", err)
	}
	if err := rawStore.ModifyOrCreatePage(pageName, wikipage.AnonymousIdentity, "setup", func(_ string) (string, error) {
		return pageText, nil
	}); err != nil {
		b.Fatalf("write initial page: %v", err)
	}

	human := tailscale.NewIdentity("alice@example.com", "Alice", "alice")

	return mutator, pageName, human, func() {
		_ = os.RemoveAll(tempDir)
	}
}

// BenchmarkChecklistDeleteItem measures the end-to-end server-side cost of a
// single DeleteItem on a synthetic page with ~120 live items and ~2000 events.
//
// Cost hypothesis breakdown:
//
//	(a) frontmatter re-serialize/rewrite: dominant — scales with event log size
//	(b) sync-token + fan-out: negligible — no subscribers in this benchmark
//	(c) fsync: not present — os.WriteFile without O_SYNC
func BenchmarkChecklistDeleteItem(b *testing.B) {
	b.Run("120items_2000events", func(b *testing.B) {
		mutator, pageName, human, cleanup := newBenchFixture(b, 120, 2000)
		defer cleanup()
		ctx := context.Background()

		// Pre-add items for deletion outside the timer so each timed
		// iteration operates on a ~120-item page.
		b.StopTimer()
		uids := make([]string, 0, b.N+1)
		for i := range b.N + 1 {
			item, _, err := mutator.AddItem(ctx, pageName, "Grocery",
				checklistmutator.AddItemArgs{Text: fmt.Sprintf("bench-delete-%d", i)}, human)
			if err != nil {
				b.Fatalf("AddItem setup: %v", err)
			}
			uids = append(uids, item.Uid)
		}
		b.ResetTimer()
		b.StartTimer()

		for i := range b.N {
			_, err := mutator.DeleteItem(ctx, pageName, "Grocery", uids[i], nil, human)
			if err != nil {
				b.Fatalf("DeleteItem: %v", err)
			}
		}
	})
}

// BenchmarkChecklistAddItem measures the end-to-end server-side cost of a
// single AddItem on a synthetic page starting with ~120 live items and ~2000
// events. The page grows by one event per iteration, so later iterations are
// slightly slower; this mirrors real-world O(N²) behaviour.
func BenchmarkChecklistAddItem(b *testing.B) {
	b.Run("120items_2000events", func(b *testing.B) {
		mutator, pageName, human, cleanup := newBenchFixture(b, 120, 2000)
		defer cleanup()
		ctx := context.Background()

		b.ResetTimer()
		for i := range b.N {
			_, _, err := mutator.AddItem(ctx, pageName, "Grocery",
				checklistmutator.AddItemArgs{Text: fmt.Sprintf("bench-add-%d", i)}, human)
			if err != nil {
				b.Fatalf("AddItem: %v", err)
			}
		}
	})
}

// BenchmarkTOMLParseCost measures the cost of parsing a 120-item + 2000-event
// TOML page. Each DeleteItem/AddItem call runs this parse 4 times; this
// benchmark isolates that portion of the total cost.
func BenchmarkTOMLParseCost(b *testing.B) {
	b.Run("120items_2000events", func(b *testing.B) {
		fm := buildSyntheticChecklistFM(120, 2000)
		pageText, err := wikipage.CombineFrontMatterAndMarkdown(fm, "# Benchmark page\n")
		if err != nil {
			b.Fatal(err)
		}
		pageBytes := []byte(pageText)
		b.ResetTimer()
		for range b.N {
			var out map[string]any
			// Simulate the toml.Unmarshal path used by page.GetFrontMatter.
			// CombineFrontMatterAndMarkdown wraps the TOML between +++ delimiters;
			// strip them so the raw TOML is what gets parsed here.
			rawTOML := extractTOML(pageBytes)
			if err := toml.Unmarshal(rawTOML, &out); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkTOMLMarshalCost measures the cost of marshalling a 120-item +
// 2000-event frontmatter map. Each item mutation runs this once via
// CombineFrontMatterAndMarkdown.
func BenchmarkTOMLMarshalCost(b *testing.B) {
	b.Run("120items_2000events", func(b *testing.B) {
		fm := buildSyntheticChecklistFM(120, 2000)
		b.ResetTimer()
		for range b.N {
			_, err := wikipage.CombineFrontMatterAndMarkdown(fm, "# Benchmark page\n")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkHistorySnapshotCost measures the two-file I/O cost (content file +
// metadata JSON sidecar) that captureVersionLockedWithSource performs on every
// item mutation. For a 120-item + 2000-event page this is the history write
// for the outgoing page content.
func BenchmarkHistorySnapshotCost(b *testing.B) {
	b.Run("120items_2000events", func(b *testing.B) {
		fm := buildSyntheticChecklistFM(120, 2000)
		pageText, err := wikipage.CombineFrontMatterAndMarkdown(fm, "# Benchmark page\n")
		if err != nil {
			b.Fatal(err)
		}
		content := []byte(pageText)
		b.SetBytes(int64(len(content)))
		b.ResetTimer()

		tempDir, err := os.MkdirTemp("", "bench-history-")
		if err != nil {
			b.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(tempDir) }()

		for i := range b.N {
			// Two-file write pattern matching captureVersionLockedWithSource.
			contentPath := fmt.Sprintf("%s/snap-%d.md", tempDir, i)
			metaPath := fmt.Sprintf("%s/snap-%d.meta.json", tempDir, i)
			if err := os.WriteFile(contentPath, content, 0o644); err != nil {
				b.Fatal(err)
			}
			meta := fmt.Appendf(nil, `{"version_id":"v%d","byte_size":%d}`, i, len(content))
			if err := os.WriteFile(metaPath, meta, 0o644); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// extractTOML strips the +++ delimiters from a CombineFrontMatterAndMarkdown
// output and returns the raw TOML bytes for benchmarking.
func extractTOML(page []byte) []byte {
	const delim = "+++\n"
	start := len(delim)
	if len(page) < start*2 {
		return page
	}
	end := -1
	for i := start; i < len(page)-3; i++ {
		if page[i] == '+' && page[i+1] == '+' && page[i+2] == '+' {
			end = i
			break
		}
	}
	if end < 0 {
		return page
	}
	return page[start:end]
}
