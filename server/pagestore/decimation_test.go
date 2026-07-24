package pagestore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HistoryDecimationJob", func() {
	var (
		store   *Store
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "decimation-test")
		Expect(err).NotTo(HaveOccurred())
		store = NewStore(tempDir)
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	// writeVersion installs a synthetic version directly into the on-disk
	// history layout. Decimation only reads metadata, so fabricated version
	// IDs are acceptable for these tests.
	writeVersion := func(identifier, versionID string, createdAt time.Time) {
		munged := base32tools.EncodeToBase32(strings.ToLower(identifier))
		pageDir := filepath.Join(tempDir, historyDirName, munged)
		Expect(os.MkdirAll(pageDir, 0o755)).To(Succeed())

		content := "version " + versionID + "\n"
		contentPath := filepath.Join(pageDir, versionID+versionFileExt)
		Expect(os.WriteFile(contentPath, []byte(content), 0o644)).To(Succeed())

		meta := versionMetadataOnDisk{
			VersionID:      versionID,
			PageIdentifier: identifier,
			CreatedAt:      createdAt,
			Author:         "test",
			IsAgent:        false,
			Source:         "test",
			SHA256:         "0000000000000000000000000000000000000000000000000000000000000000",
			ByteSize:       int64(len(content)),
		}
		metaBytes, err := json.Marshal(meta)
		Expect(err).NotTo(HaveOccurred())
		metaPath := filepath.Join(pageDir, versionID+versionMetaExt)
		Expect(os.WriteFile(metaPath, metaBytes, 0o644)).To(Succeed())
	}

	listVersionIDs := func(identifier string) []string {
		munged := base32tools.EncodeToBase32(strings.ToLower(identifier))
		pageDir := filepath.Join(tempDir, historyDirName, munged)
		entries, err := os.ReadDir(pageDir)
		Expect(err).NotTo(HaveOccurred())

		var ids []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, versionMetaExt) {
				ids = append(ids, strings.TrimSuffix(name, versionMetaExt))
			}
		}
		return ids
	}

	Context("when all versions are under 7 days old", func() {
		It("keeps all versions", func() {
			now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
			identifier := "recent-page"

			for i := range 5 {
				writeVersion(identifier, "recent-"+string(rune('A'+i)), now.Add(-time.Duration(i)*24*time.Hour))
			}

			job := NewHistoryDecimationJob(store, now)
			Expect(job.Execute()).To(Succeed())

			Expect(listVersionIDs(identifier)).To(ConsistOf("recent-A", "recent-B", "recent-C", "recent-D", "recent-E"))
		})
	})

	Context("when versions fall in the 7-day to 26-week weekly tier", func() {
		It("keeps only the newest version per ISO week", func() {
			now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
			identifier := "weekly-page"

			// Weekly tier: keep the newest version in each ISO week.
			// 2026-07-15 is a Wednesday. Pick dates that straddle two ISO weeks.
			writeVersion(identifier, "w2-a", now.Add(-16*24*time.Hour))  // 2026-06-29 Mon, W27 (older than w2-b)
			writeVersion(identifier, "w2-b", now.Add(-15*24*time.Hour))  // 2026-06-30 Tue, W27 (newest in week)
			writeVersion(identifier, "w1-a", now.Add(-9*24*time.Hour))   // 2026-07-06 Mon, W28 (older than w1-b)
			writeVersion(identifier, "w1-b", now.Add(-8*24*time.Hour))   // 2026-07-07 Tue, W28 (newest in week)
			writeVersion(identifier, "recent", now.Add(-2*24*time.Hour)) // recent tier: always kept

			job := NewHistoryDecimationJob(store, now)
			Expect(job.Execute()).To(Succeed())

			survivors := listVersionIDs(identifier)
			Expect(survivors).To(ContainElements("recent", "w1-b", "w2-b"))
			Expect(survivors).NotTo(ContainElements("w1-a", "w2-a"))
		})
	})

	Context("when versions fall in the 26-week to 5-year monthly tier", func() {
		It("keeps only the newest version per calendar month", func() {
			now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
			identifier := "monthly-page"

			// One year back, spanning two months.
			writeVersion(identifier, "m1-early", time.Date(2025, 6, 5, 0, 0, 0, 0, time.UTC))
			writeVersion(identifier, "m1-late", time.Date(2025, 6, 28, 0, 0, 0, 0, time.UTC))
			writeVersion(identifier, "m2-early", time.Date(2025, 5, 5, 0, 0, 0, 0, time.UTC))
			writeVersion(identifier, "m2-late", time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC))

			job := NewHistoryDecimationJob(store, now)
			Expect(job.Execute()).To(Succeed())

			survivors := listVersionIDs(identifier)
			Expect(survivors).To(ConsistOf("m1-late", "m2-late"))
		})
	})

	Context("when versions are older than 5 years", func() {
		It("purges all versions older than 5 years", func() {
			now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
			identifier := "old-page"

			writeVersion(identifier, "old1", now.Add(-6*365*24*time.Hour))
			writeVersion(identifier, "old2", now.Add(-5*365*24*time.Hour).Add(-48*time.Hour))
			writeVersion(identifier, "exactly5y", now.Add(-5*365*24*time.Hour).Add(1*time.Hour))

			job := NewHistoryDecimationJob(store, now)
			Expect(job.Execute()).To(Succeed())

			Expect(listVersionIDs(identifier)).To(ConsistOf("exactly5y"))
		})
	})

	Context("when versions span all retention tiers", func() {
		It("applies each tier correctly", func() {
			now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
			identifier := "mixed-page"

			// Recent tier: all kept.
			writeVersion(identifier, "recent-a", now.Add(-3*24*time.Hour))
			writeVersion(identifier, "recent-b", now.Add(-1*24*time.Hour))

			// Weekly tier: one per ISO week.
			writeVersion(identifier, "week-old-a", now.Add(-8*24*time.Hour))
			writeVersion(identifier, "week-old-b", now.Add(-9*24*time.Hour)) // same ISO week, older
			writeVersion(identifier, "two-weeks-a", now.Add(-15*24*time.Hour))
			writeVersion(identifier, "two-weeks-b", now.Add(-16*24*time.Hour)) // same ISO week, older

			// Monthly tier: one per calendar month.
			writeVersion(identifier, "month-a", time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC))
			writeVersion(identifier, "month-b", time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)) // same month, older
			writeVersion(identifier, "year-a", time.Date(2024, 10, 15, 0, 0, 0, 0, time.UTC))
			writeVersion(identifier, "year-b", time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)) // same month, older

			// Purged tier.
			writeVersion(identifier, "ancient", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

			job := NewHistoryDecimationJob(store, now)
			Expect(job.Execute()).To(Succeed())

			survivors := listVersionIDs(identifier)
			Expect(survivors).To(ConsistOf(
				"recent-a", "recent-b",
				"week-old-a", "two-weeks-a",
				"month-a", "year-a",
			))
		})
	})

	Context("when the history directory exists but is empty", func() {
		It("is a no-op", func() {
			now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

			Expect(os.MkdirAll(filepath.Join(tempDir, historyDirName), 0o755)).To(Succeed())

			job := NewHistoryDecimationJob(store, now)
			Expect(job.Execute()).To(Succeed())
		})
	})

	Context("when the history directory does not exist", func() {
		It("returns nil without error", func() {
			now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

			_, err := os.Stat(filepath.Join(tempDir, historyDirName))
			Expect(os.IsNotExist(err)).To(BeTrue())

			job := NewHistoryDecimationJob(store, now)
			Expect(job.Execute()).To(Succeed())
		})
	})
})
