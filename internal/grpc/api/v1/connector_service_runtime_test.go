//revive:disable:dot-imports
package v1

import (
	"context"
	"errors"
	"fmt"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type runtimeBindingStore struct {
	loadBindings []connectors.Binding
	loadErr      error
	findBinding  connectors.Binding
	findFound    bool
	findErr      error
}

func (s *runtimeBindingStore) LoadBindings(wikipage.PageIdentifier, connectors.ConnectorKind) ([]connectors.Binding, error) {
	return s.loadBindings, s.loadErr
}

func (s *runtimeBindingStore) FindBinding(wikipage.PageIdentifier, connectors.ConnectorKind, string, string) (connectors.Binding, bool, error) {
	return s.findBinding, s.findFound, s.findErr
}

func (*runtimeBindingStore) SaveBinding(wikipage.PageIdentifier, connectors.ConnectorKind, connectors.Binding) error {
	return nil
}

func (*runtimeBindingStore) DeleteBinding(wikipage.PageIdentifier, connectors.ConnectorKind, string, string) error {
	return nil
}

func (*runtimeBindingStore) WithProfileLock(wikipage.PageIdentifier, func() error) error {
	return nil
}

func (*runtimeBindingStore) ListAllProfilesWithBindings(connectors.ConnectorKind) ([]wikipage.PageIdentifier, error) {
	return nil, nil
}

type runtimeCredentials struct {
	state         *apiv1.ConnectorState
	loadErr       error
	clearErr      error
	configured    bool
	configuredErr error
	missingErr    error
}

func (c *runtimeCredentials) LoadState(context.Context, wikipage.PageIdentifier, []connectors.Binding) (*apiv1.ConnectorState, error) {
	return c.state, c.loadErr
}

func (c *runtimeCredentials) ClearState(context.Context, wikipage.PageIdentifier, []connectors.Binding) (*apiv1.ConnectorState, error) {
	return c.state, c.clearErr
}

func (c *runtimeCredentials) IsConfigured(context.Context, wikipage.PageIdentifier) (bool, error) {
	return c.configured, c.configuredErr
}

func (c *runtimeCredentials) MissingCredentialError() error {
	return c.missingErr
}

type runtimeBackendAdapter struct{}

func (runtimeBackendAdapter) Kind() connectors.ConnectorKind {
	return connectors.ConnectorKindGoogleKeep
}
func (runtimeBackendAdapter) PullRemote(context.Context, connectors.Binding) (connectors.RemotePullResult, error) {
	return connectors.RemotePullResult{}, nil
}
func (runtimeBackendAdapter) InsertRemote(context.Context, connectors.Binding, connectors.WikiItem) (connectors.RemoteRef, error) {
	return "", nil
}
func (runtimeBackendAdapter) PatchRemote(context.Context, connectors.Binding, connectors.RemoteRef, connectors.WikiItem) (connectors.RemoteRef, error) {
	return "", nil
}
func (runtimeBackendAdapter) DeleteRemote(context.Context, connectors.Binding, connectors.RemoteRef) error {
	return nil
}
func (runtimeBackendAdapter) SyncCollectionState(context.Context, connectors.Binding, []connectors.WikiItem) (connectors.Binding, error) {
	return connectors.Binding{}, nil
}
func (runtimeBackendAdapter) RemoteToWiki(connectors.RemoteItem) (connectors.WikiItem, error) {
	return connectors.WikiItem{}, nil
}
func (runtimeBackendAdapter) WikiToRemote(connectors.WikiItem) (connectors.RemoteItem, error) {
	return connectors.RemoteItem{}, nil
}
func (runtimeBackendAdapter) AdvanceCursor(connectors.Binding, connectors.RemotePullResult) connectors.Binding {
	return connectors.Binding{}
}
func (runtimeBackendAdapter) RefreshItemBaseline(connectors.Binding, connectors.RemoteItem) connectors.Binding {
	return connectors.Binding{}
}
func (runtimeBackendAdapter) SeedBindingState(context.Context, wikipage.PageIdentifier, string, []connectors.WikiItem) (connectors.AdapterState, error) {
	return nil, nil
}
func (runtimeBackendAdapter) ValidateRemoteBinding(context.Context, wikipage.PageIdentifier, string) error {
	return nil
}
func (runtimeBackendAdapter) RebuildAdapterState(context.Context, connectors.Binding) (connectors.AdapterState, error) {
	return nil, nil
}
func (runtimeBackendAdapter) FetchRemoteListTitle(context.Context, wikipage.PageIdentifier, string) (string, bool, error) {
	return "", false, nil
}
func (runtimeBackendAdapter) ListRemoteCollections(context.Context, wikipage.PageIdentifier) ([]connectors.RemoteCollection, error) {
	return nil, nil
}
func (runtimeBackendAdapter) CreateRemoteCollection(context.Context, wikipage.PageIdentifier, string, []connectors.WikiItem) (handle string, title string, err error) {
	return "", "", nil
}
func (runtimeBackendAdapter) EncodeAdapterState(connectors.AdapterState) (map[string]any, error) {
	return nil, nil
}
func (runtimeBackendAdapter) DecodeAdapterState(map[string]any) (connectors.AdapterState, error) {
	return nil, nil
}
func (runtimeBackendAdapter) SupportsSubtasks() bool { return false }
func (runtimeBackendAdapter) ReadRemoteByRef(context.Context, connectors.Binding, connectors.RemoteRef) (connectors.RemoteItem, error) {
	return connectors.RemoteItem{}, nil
}
func (runtimeBackendAdapter) ClassifyError(error) connectors.ErrorClass {
	return connectors.ErrorClassFatal
}

var _ = Describe("connectorRuntime shared helpers", func() {
	var (
		ctx       context.Context
		server    *Server
		profileID wikipage.PageIdentifier
		store     *runtimeBindingStore
		creds     *runtimeCredentials
		runtime   *connectorRuntime
		err       error
	)

	BeforeEach(func() {
		ctx = context.Background()
		server = &Server{}
		profileID = wikipage.PageIdentifier("profile")
		store = &runtimeBindingStore{}
		creds = &runtimeCredentials{
			state:      &apiv1.ConnectorState{},
			missingErr: errors.New("missing credentials"),
		}
		runtime = &connectorRuntime{
			protoKind:    apiv1.ConnectorKind_CONNECTOR_KIND_GOOGLE_KEEP,
			kind:         connectors.ConnectorKindGoogleKeep,
			engine:       &engine.Engine{},
			adapter:      runtimeBackendAdapter{},
			bindingStore: store,
			credentials:  creds,
			mapErr: func(source error) error {
				return fmt.Errorf("mapped: %w", source)
			},
			bindingToProto: func(binding connectors.Binding) *apiv1.BindingState {
				return &apiv1.BindingState{Page: binding.Page, ListName: binding.ListName}
			},
		}
		err = nil
	})

	Describe("loadConnectorBindings", func() {
		Describe("when the binding store returns an error", func() {
			BeforeEach(func() {
				store.loadErr = errors.New("load failed")
				_, err = server.loadConnectorBindings(runtime, profileID, "GetState")
			})

			It("should map the error", func() {
				Expect(err).To(MatchError(ContainSubstring("mapped: load failed")))
			})
		})
	})

	Describe("loadConnectorState", func() {
		Describe("when credentials cannot be loaded", func() {
			BeforeEach(func() {
				creds.loadErr = errors.New("credential load failed")
				_, err = server.loadConnectorState(ctx, runtime, profileID, "GetState")
			})

			It("should map the error", func() {
				Expect(err).To(MatchError(ContainSubstring("mapped: credential load failed")))
			})
		})
	})

	Describe("clearConnectorState", func() {
		Describe("when credentials cannot be cleared", func() {
			BeforeEach(func() {
				creds.clearErr = errors.New("credential clear failed")
				_, err = server.clearConnectorState(ctx, runtime, profileID)
			})

			It("should map the error", func() {
				Expect(err).To(MatchError(ContainSubstring("mapped: credential clear failed")))
			})
		})
	})

	Describe("requireConnectorConfigured", func() {
		Describe("when credential lookup fails", func() {
			BeforeEach(func() {
				creds.configuredErr = errors.New("credential check failed")
				err = server.requireConnectorConfigured(ctx, runtime, profileID, "Bind")
			})

			It("should map the error", func() {
				Expect(err).To(MatchError(ContainSubstring("mapped: credential check failed")))
			})
		})

		Describe("when credentials are missing", func() {
			BeforeEach(func() {
				err = server.requireConnectorConfigured(ctx, runtime, profileID, "Bind")
			})

			It("should map the missing-credentials error", func() {
				Expect(err).To(MatchError(ContainSubstring("mapped: missing credentials")))
			})
		})
	})

	Describe("findConnectorBinding", func() {
		Describe("when the binding store returns an error", func() {
			BeforeEach(func() {
				store.findErr = errors.New("find failed")
				_, _, err = server.findConnectorBinding(runtime, profileID, "page", "list", "SyncNow")
			})

			It("should map the error", func() {
				Expect(err).To(MatchError(ContainSubstring("mapped: find failed")))
			})
		})
	})

	Describe("checklistBindingFromConnector", func() {
		Describe("when the runtime is not wired", func() {
			var hit bool

			BeforeEach(func() {
				hit, err = server.checklistBindingFromConnector(ctx, nil, profileID, &apiv1.GetChecklistBindingStateRequest{
					Page:     "page",
					ListName: "list",
				}, &apiv1.ChecklistBindingState{})
			})

			It("should not report a hit", func() {
				Expect(hit).To(BeFalse())
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("when credential lookup fails", func() {
			BeforeEach(func() {
				creds.configuredErr = errors.New("credential check failed")
				_, err = server.checklistBindingFromConnector(ctx, runtime, profileID, &apiv1.GetChecklistBindingStateRequest{
					Page:     "page",
					ListName: "list",
				}, &apiv1.ChecklistBindingState{})
			})

			It("should map the error", func() {
				Expect(err).To(MatchError(ContainSubstring("mapped: credential check failed")))
			})
		})
	})
})
