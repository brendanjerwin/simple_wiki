//revive:disable:dot-imports
//revive:disable:add-constant
package sync_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	keepsync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/sync"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Note: debouncerFakeEnqueuer and debouncerFakeLogger are defined in
// sync_debouncer_test.go (same bridge_test package).

const cronTestLogin = "cron-user@example.com"

var _ = Describe("KeepCronTickJob", func() {
	var (
		enqueuer  *debouncerFakeEnqueuer
		logger    *debouncerFakeLogger
		connector *keepsync.Connector
		profileID wikipage.PageIdentifier
	)

	BeforeEach(func() {
		var err error
		profileID, err = wikipage.ProfileIdentifierFor(cronTestLogin)
		Expect(err).ToNot(HaveOccurred())

		enqueuer = &debouncerFakeEnqueuer{}
		logger = &debouncerFakeLogger{}

		store := keepsync.NewBindingStore(newFakeStore())
		connector = keepsync.NewConnector(store, http.DefaultClient, fakeClock{})
	})

	// ------------------------------------------------------------------ Execute

	Describe("Execute", func() {
		Describe("when no connector is set (nil connector)", func() {
			var (
				job *keepsync.KeepCronTickJob
				err error
			)

			BeforeEach(func() {
				job = keepsync.NewKeepCronTickJob(nil, enqueuer, logger, nil)
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not enqueue any jobs", func() {
				Expect(enqueuer.jobCount()).To(Equal(0))
			})
		})

		Describe("when the lister returns zero bindings", func() {
			var (
				job *keepsync.KeepCronTickJob
				err error
			)

			BeforeEach(func() {
				lister := func() []keepsync.BindingKey { return nil }
				job = keepsync.NewKeepCronTickJob(connector, enqueuer, logger, lister)
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not enqueue any jobs", func() {
				Expect(enqueuer.jobCount()).To(Equal(0))
			})

			It("should log one tick-fired info line", func() {
				Expect(logger.infoCount()).To(Equal(1))
			})
		})

		Describe("when the lister returns two bindings (registry/discovery walk path)", func() {
			var (
				job  *keepsync.KeepCronTickJob
				err  error
				keys []keepsync.BindingKey
			)

			BeforeEach(func() {
				keys = []keepsync.BindingKey{
					{ProfileID: profileID, Page: "Board", ListName: "todo"},
					{ProfileID: profileID, Page: "Board", ListName: "done"},
				}
				lister := func() []keepsync.BindingKey { return keys }
				job = keepsync.NewKeepCronTickJob(connector, enqueuer, logger, lister)
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should enqueue one sync job per binding", func() {
				Expect(enqueuer.jobCount()).To(Equal(2))
			})

			It("should log one tick-fired info line", func() {
				Expect(logger.infoCount()).To(Equal(1))
			})
		})

		Describe("when using the in-memory active set (no lister, connector path)", func() {
			var (
				job *keepsync.KeepCronTickJob
				err error
			)

			BeforeEach(func() {
				connector.RegisterActiveBindings([]keepsync.BindingKey{
					{ProfileID: profileID, Page: "Notes", ListName: "list1"},
				})
				// nil lister → falls back to connector.ActiveBindingsSnapshot
				job = keepsync.NewKeepCronTickJob(connector, enqueuer, logger, nil)
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should enqueue one sync job for the registered binding", func() {
				Expect(enqueuer.jobCount()).To(Equal(1))
			})
		})
	})

	// ------------------------------------------------------------------ RegisterActiveBindings / ActiveBindingsSnapshot

	Describe("RegisterActiveBindings and ActiveBindingsSnapshot", func() {
		Describe("when two bindings are registered", func() {
			var snapshot []keepsync.BindingKey

			BeforeEach(func() {
				keys := []keepsync.BindingKey{
					{ProfileID: profileID, Page: "A", ListName: "x"},
					{ProfileID: profileID, Page: "B", ListName: "y"},
				}
				connector.RegisterActiveBindings(keys)
				snapshot = connector.ActiveBindingsSnapshot()
			})

			It("should return two entries in the snapshot", func() {
				Expect(snapshot).To(HaveLen(2))
			})
		})

		Describe("when RegisterActiveBindings is called on a fresh connector", func() {
			var snapshot []keepsync.BindingKey

			BeforeEach(func() {
				// Fresh connector — no prior registration.
				snapshot = connector.ActiveBindingsSnapshot()
			})

			It("should return an empty snapshot", func() {
				Expect(snapshot).To(BeEmpty())
			})
		})
	})
})
