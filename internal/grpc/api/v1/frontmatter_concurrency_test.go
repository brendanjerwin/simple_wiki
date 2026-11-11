//revive:disable:dot-imports
package v1_test

import (
	"context"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// pausingFrontmatterStore is a real pagestore.Store (so the store's per-page
// canonical lock behaves exactly as in production) whose ReadFrontMatter calls
// for one page pause on a channel. That is the "pause a read" technique from
// server/checklistmutator/mutator_test.go, applied at the gRPC layer: it lets a
// test land a concurrent checklist append precisely inside the window between a
// frontmatter handler's read and its write.
type pausingFrontmatterStore struct {
	*pagestore.Store
	targetPage wikipage.PageIdentifier
	armed      atomic.Bool
	paused     chan struct{}
	release    chan struct{}
}

func (*pausingFrontmatterStore) DeletePage(wikipage.PageIdentifier) error { return nil }

// ReadMarkdown is not used by the frontmatter handlers under test; the stub
// keeps the double satisfying wikipage.PageReaderMutator like the mutator
// suite's pausedReadStore does.
func (*pausingFrontmatterStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (s *pausingFrontmatterStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	actual, fm, err := s.Store.ReadFrontMatter(id)
	if id == s.targetPage && s.armed.Load() {
		s.paused <- struct{}{}
		<-s.release
	}
	return actual, fm, err
}

var _ = Describe("frontmatter writes racing a checklist append", func() {
	const page = wikipage.PageIdentifier("ai_assistant")

	var (
		ctx           context.Context
		store         *pausingFrontmatterStore
		mutator       *checklistmutator.Mutator
		server        *v1.Server
		human         = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
		appendedTexts []string
		mergedErr     error
		list          *apiv1.Checklist
		listErr       error
	)

	BeforeEach(func() {
		ctx = context.Background()
		appendedTexts = nil

		store = &pausingFrontmatterStore{
			Store:      pagestore.NewStore(GinkgoT().TempDir()),
			targetPage: page,
			paused:     make(chan struct{}),
			release:    make(chan struct{}),
		}
		mutator = checklistmutator.New(store, checklistmutator.SystemClock{}, ulid.NewSystemGenerator())
		server = mustNewServer(store, nil, nil)
		// The mutator only appends to pages that already exist.
		Expect(store.Store.WriteFrontMatter(page, wikipage.FrontMatter{}, wikipage.AnonymousIdentity)).To(Succeed())

		_, _, seedErr := mutator.AddItem(ctx, string(page), "todos", checklistmutator.AddItemArgs{Text: "seed"}, human)
		Expect(seedErr).NotTo(HaveOccurred())
	})

	Describe("when a MergeFrontmatter write overlaps the append", func() {
		BeforeEach(func() {
			store.armed.Store(true)
			done := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				defer close(done)

				body, bodyErr := structpb.NewStruct(map[string]any{"owner": "brendan"})
				Expect(bodyErr).NotTo(HaveOccurred())
				_, mergedErr = server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        string(page),
					Frontmatter: body,
				})
			}()

			// Every pause is a frontmatter read performed by the merge path; each
			// one is a chance to land an append inside a read→write window that
			// the old unlocked form left open. Append on every pause so the
			// interleaving happens on the code that HAS the window.
			appendOne := func() {
				store.armed.Store(false)
				text := fmt.Sprintf("appended-%d", len(appendedTexts))
				_, _, appendErr := mutator.AddItem(ctx, string(page), "todos", checklistmutator.AddItemArgs{Text: text}, human)
				Expect(appendErr).NotTo(HaveOccurred())
				appendedTexts = append(appendedTexts, text)
				store.armed.Store(true)
			}
			for running := true; running; {
				select {
				case <-store.paused:
					appendOne()
					store.release <- struct{}{}
				case <-done:
					running = false
				}
			}
			store.armed.Store(false)

			// The atomic form leaves no window to interleave into, so no pause
			// ever fires. Append once on the far side of the merge anyway: the
			// assertion below must not pass by iterating over an empty set, and
			// "an append that lands after a completed merge still survives" is
			// part of the contract this test guards.
			if len(appendedTexts) == 0 {
				appendOne()
			}

			list, listErr = mutator.ListItems(ctx, string(page), "todos")
		})

		It("should complete the merge without error", func() {
			Expect(mergedErr).NotTo(HaveOccurred())
		})

		It("should keep every appended checklist item", func() {
			Expect(listErr).NotTo(HaveOccurred())
			texts := make([]string, 0, len(list.Items))
			for _, item := range list.Items {
				texts = append(texts, item.Text)
			}
			Expect(texts).To(ContainElements(appendedTexts))
		})

		It("should keep the seed item", func() {
			Expect(listErr).NotTo(HaveOccurred())
			texts := make([]string, 0, len(list.Items))
			for _, item := range list.Items {
				texts = append(texts, item.Text)
			}
			Expect(texts).To(ContainElement("seed"))
		})

		It("should persist the merged frontmatter key", func() {
			_, fm, readErr := store.Store.ReadFrontMatter(page)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(fm["owner"]).To(Equal("brendan"))
		})
	})
})
