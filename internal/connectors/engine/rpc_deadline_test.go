package engine

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("withRPCDeadline", func() {
	var (
		eng        *Engine
		parentCtx  context.Context
		cancelRoot context.CancelFunc
	)

	BeforeEach(func() {
		// The helper does not consult any Engine field; a zero-valued
		// receiver is sufficient for these tests.
		eng = &Engine{}
		parentCtx, cancelRoot = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancelRoot()
	})

	When("called with a fresh parent context", func() {
		var (
			rpcCtx context.Context
			cancel context.CancelFunc
		)

		BeforeEach(func() {
			rpcCtx, cancel = eng.withRPCDeadline(parentCtx)
		})

		AfterEach(func() {
			cancel()
		})

		It("should attach a deadline to the returned context", func() {
			deadline, ok := rpcCtx.Deadline()
			Expect(ok).To(BeTrue())
			Expect(deadline).To(BeTemporally("~", time.Now().Add(PerRPCDeadline), 100*time.Millisecond))
		})

		It("should propagate cancel to the derived context", func() {
			cancel()
			Expect(errors.Is(rpcCtx.Err(), context.Canceled)).To(BeTrue())
		})
	})

	When("the parent ctx is already cancelled", func() {
		var rpcCtx context.Context

		BeforeEach(func() {
			cancelRoot()
			var cancel context.CancelFunc
			rpcCtx, cancel = eng.withRPCDeadline(parentCtx)
			DeferCleanup(cancel)
		})

		It("should propagate cancellation immediately", func() {
			Expect(rpcCtx.Err()).To(HaveOccurred())
		})
	})

	When("PerRPCDeadline is overridden to a tight value", func() {
		var (
			savedDeadline time.Duration
			rpcCtx        context.Context
		)

		BeforeEach(func() {
			savedDeadline = PerRPCDeadline
			PerRPCDeadline = 20 * time.Millisecond
			var cancel context.CancelFunc
			rpcCtx, cancel = eng.withRPCDeadline(parentCtx)
			DeferCleanup(cancel)
		})

		AfterEach(func() {
			PerRPCDeadline = savedDeadline
		})

		It("should fire the deadline within the configured window", func() {
			Eventually(rpcCtx.Done(), 200*time.Millisecond).Should(BeClosed())
			Expect(errors.Is(rpcCtx.Err(), context.DeadlineExceeded)).To(BeTrue())
		})
	})
})
