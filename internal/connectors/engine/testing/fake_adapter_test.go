package testing

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

func TestFakeAdapter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "engine/testing FakeAdapter")
}

var _ = Describe("FakeAdapter", func() {
	var fa *FakeAdapter

	BeforeEach(func() {
		fa = &FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks, SubtasksSupport: true}
	})

	Describe("Kind", func() {
		It("should return the configured ConnectorKind", func() {
			Expect(fa.Kind()).To(Equal(connectors.ConnectorKindGoogleTasks))
		})
	})

	Describe("SupportsSubtasks", func() {
		It("should return the configured value", func() {
			Expect(fa.SupportsSubtasks()).To(BeTrue())
		})
	})

	Describe("PullRemote", func() {
		When("no response is queued", func() {
			var result connectors.RemotePullResult
			var err error

			BeforeEach(func() {
				result, err = fa.PullRemote(context.Background(), connectors.Binding{Page: "p"})
			})

			It("should return zero RemotePullResult", func() {
				Expect(result).To(Equal(connectors.RemotePullResult{}))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should record the call", func() {
				Expect(fa.RecordedPullRemote).To(HaveLen(1))
				Expect(fa.RecordedPullRemote[0].Binding.Page).To(Equal("p"))
			})
		})

		When("a response is queued", func() {
			var result connectors.RemotePullResult
			var err error
			programmedResult := connectors.RemotePullResult{Items: []connectors.RemoteItem{{Ref: "x"}}}
			programmedErr := errors.New("boom")

			BeforeEach(func() {
				fa.SetPullRemoteResponse(programmedResult, programmedErr)
				result, err = fa.PullRemote(context.Background(), connectors.Binding{})
			})

			It("should return the programmed result", func() {
				Expect(result).To(Equal(programmedResult))
			})

			It("should return the programmed error", func() {
				Expect(err).To(MatchError(programmedErr))
			})
		})
	})

	Describe("ClassifyError", func() {
		When("err is nil", func() {
			It("should return ErrorClassNone", func() {
				Expect(fa.ClassifyError(nil)).To(Equal(connectors.ErrorClassNone))
			})
		})

		When("err is non-nil and no response queued", func() {
			It("should default to ErrorClassTransient", func() {
				Expect(fa.ClassifyError(errors.New("x"))).To(Equal(connectors.ErrorClassTransient))
			})
		})

		When("a response is queued", func() {
			BeforeEach(func() {
				fa.SetClassifyErrorResponse(connectors.ErrorClassPreconditionFailed)
			})

			It("should return the queued classification", func() {
				Expect(fa.ClassifyError(errors.New("412"))).To(Equal(connectors.ErrorClassPreconditionFailed))
			})
		})
	})

	Describe("AdapterState codec passthrough", func() {
		var raw map[string]any

		BeforeEach(func() {
			state := connectors.AdapterState{"item_id_map": map[string]any{"uid1": "task1"}}
			r, err := fa.EncodeAdapterState(state)
			Expect(err).NotTo(HaveOccurred())
			raw = r
		})

		It("should encode AdapterState by passthrough", func() {
			Expect(raw).To(HaveKey("item_id_map"))
		})

		It("should decode the encoded raw map back to equivalent AdapterState", func() {
			decoded, err := fa.DecodeAdapterState(raw)
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(HaveKey("item_id_map"))
		})
	})

	Describe("SeedBindingState recording", func() {
		BeforeEach(func() {
			_, _ = fa.SeedBindingState(context.Background(), wikipage.PageIdentifier("profile_x"), "remote_handle_42")
		})

		It("should record the profile and remote handle", func() {
			Expect(fa.RecordedSeedBindingState).To(HaveLen(1))
			Expect(fa.RecordedSeedBindingState[0].ProfileID).To(Equal(wikipage.PageIdentifier("profile_x")))
			Expect(fa.RecordedSeedBindingState[0].RemoteHandle).To(Equal("remote_handle_42"))
		})
	})
})
