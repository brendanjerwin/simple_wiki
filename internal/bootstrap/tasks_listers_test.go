//revive:disable:dot-imports
package bootstrap

import (
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeFrontmatterIndex records every QueryKeyExistence key the
// callers probe so tests can assert what canonical "is connected?"
// leaf the bootstrap wiring chose. Returns the configured profile IDs
// only when the requested key matches recordedReturnFor — emulating
// the production index, which only returns hits for keys actually
// present in frontmatter.
type fakeFrontmatterIndex struct {
	queriedKeys        []frontmatter.DottedKeyPath
	profilesByLeafKey  map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier
}

func (*fakeFrontmatterIndex) QueryExactMatch(_ frontmatter.DottedKeyPath, _ frontmatter.Value) []wikipage.PageIdentifier {
	return nil
}

func (f *fakeFrontmatterIndex) QueryKeyExistence(key frontmatter.DottedKeyPath) []wikipage.PageIdentifier {
	f.queriedKeys = append(f.queriedKeys, key)
	if hits, ok := f.profilesByLeafKey[key]; ok {
		out := make([]wikipage.PageIdentifier, len(hits))
		copy(out, hits)
		return out
	}
	return nil
}

func (*fakeFrontmatterIndex) QueryPrefixMatch(_ frontmatter.DottedKeyPath, _ string) []wikipage.PageIdentifier {
	return nil
}

func (*fakeFrontmatterIndex) QueryExactMatchSortedBy(_ frontmatter.DottedKeyPath, _ frontmatter.Value, _ frontmatter.DottedKeyPath, _ bool, _ int) []wikipage.PageIdentifier {
	return nil
}

func (*fakeFrontmatterIndex) GetValue(_ wikipage.PageIdentifier, _ frontmatter.DottedKeyPath) frontmatter.Value {
	return ""
}

// memoryFakePages is a minimal in-memory PageReaderMutator for these
// tests. We don't reuse the sync package's fakePages because that's
// in a _test file in another package.
type memoryFakePages struct {
	pages map[wikipage.PageIdentifier]wikipage.FrontMatter
}

func newMemoryFakePages() *memoryFakePages {
	return &memoryFakePages{pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{}}
}

func (s *memoryFakePages) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	fm, ok := s.pages[id]
	if !ok {
		return id, nil, nil
	}
	return id, fm, nil
}

func (s *memoryFakePages) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	s.pages[id] = fm
	return nil
}

func (*memoryFakePages) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (*memoryFakePages) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error {
	return nil
}

func (*memoryFakePages) DeletePage(_ wikipage.PageIdentifier) error {
	return nil
}

func (*memoryFakePages) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

// seedTasksProfileWithRefreshTokenOnly persists a Tasks ConnectorState
// containing a refresh_token (no email) plus one Subscription —
// emulating the post-OAuth-callback profile that triggered the
// scheduler-invisibility bug.
func seedTasksProfileWithRefreshTokenOnly(store *taskssync.SubscriptionStore, profileID wikipage.PageIdentifier, page, listName string) {
	GinkgoHelper()
	state := taskssync.ConnectorState{
		// Email deliberately empty: the OAuth callback handler
		// invokes PersistRefreshToken with an empty accountEmail
		// because Google's /oauth2/v3/token response doesn't carry it.
		RefreshToken: "rt-from-oauth-callback",
		Subscriptions: []taskssync.Subscription{{
			Page:     page,
			ListName: listName,
			State:    taskssync.SubscriptionStateActive,
		}},
	}
	Expect(store.SaveState(profileID, state)).To(Succeed())
}

var _ = Describe("rebuildLeaseTableTasks", func() {
	When("a Tasks profile was connected via OAuth callback (refresh_token only, no email)", func() {
		var (
			pages       *memoryFakePages
			tasksStore  *taskssync.SubscriptionStore
			leaseTable  *connectors.LeaseTable
			fakeIndex   *fakeFrontmatterIndex
			site        *server.Site
			leaseCount  int
			rebuildErr  error
			profileID   = wikipage.PageIdentifier("OBZG6_profile")
			pageName    = "groceries"
			listName    = "Shopping"
		)

		BeforeEach(func() {
			pages = newMemoryFakePages()
			var err error
			tasksStore, err = taskssync.NewSubscriptionStore(pages)
			Expect(err).NotTo(HaveOccurred())
			seedTasksProfileWithRefreshTokenOnly(tasksStore, profileID, pageName, listName)

			fakeIndex = &fakeFrontmatterIndex{
				profilesByLeafKey: map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
					// Production reality: PersistRefreshToken writes
					// only refresh_token (and connected_at /
					// last_verified_at), so the frontmatter index
					// surfaces this profile under refresh_token, NOT
					// under email.
					"wiki.connectors.google_tasks.refresh_token": {profileID},
				},
			}
			site = &server.Site{FrontmatterIndexQueryer: fakeIndex}
			leaseTable = connectors.NewLeaseTable()

			leaseCount, rebuildErr = rebuildLeaseTableTasks(leaseTable, site, tasksStore)
		})

		It("should not error", func() {
			Expect(rebuildErr).NotTo(HaveOccurred())
		})

		It("should find the OAuth-callback-only profile and take its lease", func() {
			Expect(leaseCount).To(Equal(1))
		})

		It("should record the lease so cross-connector LookupOwner sees it", func() {
			owner, found := leaseTable.LookupOwner(connectors.ChecklistKey{Page: pageName, ListName: listName})
			Expect(found).To(BeTrue())
			Expect(owner.Kind).To(Equal(connectors.ConnectorKindGoogleTasks))
			Expect(owner.ProfileID).To(Equal(string(profileID)))
		})

		It("should query the frontmatter index by refresh_token, not email", func() {
			Expect(fakeIndex.queriedKeys).To(ContainElement("wiki.connectors.google_tasks.refresh_token"))
			Expect(fakeIndex.queriedKeys).NotTo(ContainElement("wiki.connectors.google_tasks.email"))
		})
	})
})

var _ = Describe("tasksFannedOutPausedChecker", func() {
	When("a Tasks profile is connected via OAuth callback (refresh_token only, no email) and its subscription is paused", func() {
		var (
			pages      *memoryFakePages
			tasksStore *taskssync.SubscriptionStore
			fakeIndex  *fakeFrontmatterIndex
			checker    *tasksFannedOutPausedChecker
			profileID  = wikipage.PageIdentifier("OBZG6_profile")
			pageName   = "groceries"
			listName   = "Shopping"
			isPaused   bool
		)

		BeforeEach(func() {
			pages = newMemoryFakePages()
			var err error
			tasksStore, err = taskssync.NewSubscriptionStore(pages)
			Expect(err).NotTo(HaveOccurred())

			pausedState := taskssync.ConnectorState{
				RefreshToken: "rt-from-oauth-callback",
				Subscriptions: []taskssync.Subscription{{
					Page:         pageName,
					ListName:     listName,
					State:        taskssync.SubscriptionStatePaused,
					PausedReason: taskssync.PausedReasonAuthFailed,
				}},
			}
			Expect(tasksStore.SaveState(profileID, pausedState)).To(Succeed())

			fakeIndex = &fakeFrontmatterIndex{
				profilesByLeafKey: map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
					"wiki.connectors.google_tasks.refresh_token": {profileID},
				},
			}
			checker = &tasksFannedOutPausedChecker{
				store: tasksStore,
				index: fakeIndex,
			}

			isPaused = checker.IsAnyChecklistSubscriptionPaused(pageName, listName)
		})

		It("should report the paused subscription via fan-out", func() {
			Expect(isPaused).To(BeTrue())
		})

		It("should query the frontmatter index by refresh_token, not email", func() {
			Expect(fakeIndex.queriedKeys).To(ContainElement("wiki.connectors.google_tasks.refresh_token"))
			Expect(fakeIndex.queriedKeys).NotTo(ContainElement("wiki.connectors.google_tasks.email"))
		})
	})
})
