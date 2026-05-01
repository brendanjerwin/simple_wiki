//revive:disable:dot-imports
//revive:disable:add-constant
package bridge_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/keep/bridge"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Note: debouncerFakeEnqueuer and debouncerFakeLogger are defined in
// sync_debouncer_test.go (same bridge_test package).

const cronTestLogin = "cron-user@example.com"

var _ = Describe("KeepCronTickJob", func() {
	var (
		enqueuer  *debouncerFakeEnqueuer
		logger    *debouncerFakeLogger
		connector *bridge.Connector
		profileID wikipage.PageIdentifier
	)

	BeforeEach(func() {
		var err error
		profileID, err = wikipage.ProfileIdentifierFor(cronTestLogin)
		Expect(err).ToNot(HaveOccurred())

		enqueuer = &debouncerFakeEnqueuer{}
		logger = &debouncerFakeLogger{}

		store := bridge.NewBindingStore(newFakeStore())
		connector = bridge.NewConnector(store, http.DefaultClient, fakeClock{})
	})

	// ------------------------------------------------------------------ Execute

	Describe("Execute", func() {
		Describe("when no connector is set (nil connector)", func() {
			var (
				job *bridge.KeepCronTickJob
				err error
			)

			BeforeEach(func() {
				job = bridge.NewKeepCronTickJob(nil, enqueuer, logger, nil)
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
				job *bridge.KeepCronTickJob
				err error
			)

			BeforeEach(func() {
				lister := func() []bridge.BindingKey { return nil }
				job = bridge.NewKeepCronTickJob(connector, enqueuer, logger, lister)
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
				job  *bridge.KeepCronTickJob
				err  error
				keys []bridge.BindingKey
			)

			BeforeEach(func() {
				keys = []bridge.BindingKey{
					{ProfileID: profileID, Page: "Board", ListName: "todo"},
					{ProfileID: profileID, Page: "Board", ListName: "done"},
				}
				lister := func() []bridge.BindingKey { return keys }
				job = bridge.NewKeepCronTickJob(connector, enqueuer, logger, lister)
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
				job *bridge.KeepCronTickJob
				err error
			)

			BeforeEach(func() {
				connector.RegisterActiveBindings([]bridge.BindingKey{
					{ProfileID: profileID, Page: "Notes", ListName: "list1"},
				})
				// nil lister → falls back to connector.ActiveBindingsSnapshot
				job = bridge.NewKeepCronTickJob(connector, enqueuer, logger, nil)
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
			var snapshot []bridge.BindingKey

			BeforeEach(func() {
				keys := []bridge.BindingKey{
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
			var snapshot []bridge.BindingKey

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
