//revive:disable:dot-imports
package v1_test

import (
	"context"
	"errors"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MockPageHistoryReader is a test double for v1.PageHistoryReader.
type MockPageHistoryReader struct {
	Versions    []v1.PageVersionMetadata
	ListErr     error
	ReadContent string
	ReadErr     error
	RestoreErr  error
	Diff        string
	DiffErr     error
}

func (m *MockPageHistoryReader) ListVersions(_ wikipage.PageIdentifier) ([]v1.PageVersionMetadata, error) {
	return m.Versions, m.ListErr
}

func (m *MockPageHistoryReader) ReadVersion(_ wikipage.PageIdentifier, _ string) (string, error) {
	return m.ReadContent, m.ReadErr
}

func (m *MockPageHistoryReader) RestoreVersion(_ wikipage.PageIdentifier, _ string, _ wikipage.Identity) error {
	return m.RestoreErr
}

func (m *MockPageHistoryReader) DiffVersions(_ wikipage.PageIdentifier, _, _ string) (string, error) {
	return m.Diff, m.DiffErr
}

// MockHistorySearcher is a test double for v1.HistorySearcher.
type MockHistorySearcher struct {
	PageResults []v1.HistorySearchResult
	PageErr     error
	Results     []v1.HistorySearchResult
	Err         error
}

func (m *MockHistorySearcher) SearchPageHistory(_ wikipage.PageIdentifier, _ string) ([]v1.HistorySearchResult, error) {
	return m.PageResults, m.PageErr
}

func (m *MockHistorySearcher) SearchHistory(_ v1.HistorySearchFilter) ([]v1.HistorySearchResult, error) {
	return m.Results, m.Err
}

func newHistoryServer(hr v1.PageHistoryReader, hs v1.HistorySearcher) *v1.Server {
	s := mustNewServer(nil, nil, nil)
	if hr != nil {
		s = s.WithHistoryReader(hr)
	}
	if hs != nil {
		s = s.WithHistorySearcher(hs)
	}
	return s
}

var _ = Describe("PageHistoryService handlers", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("ListPageVersions", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.ListPageVersions(ctx, &apiv1.ListPageVersionsRequest{})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page is required"))
			})
		})

		When("historyReader is not configured", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.ListPageVersions(ctx, &apiv1.ListPageVersionsRequest{Page: "my-page"})
			})

			It("should return Unavailable", func() {
				Expect(err).To(HaveGrpcStatus(codes.Unavailable, "history reader not configured"))
			})
		})

		When("historyReader returns an error", func() {
			var err error

			BeforeEach(func() {
				hr := &MockPageHistoryReader{ListErr: errors.New("disk failure")}
				s := newHistoryServer(hr, nil)
				_, err = s.ListPageVersions(ctx, &apiv1.ListPageVersionsRequest{Page: "my-page"})
			})

			It("should return Internal", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to list versions"))
			})
		})

		When("historyReader returns versions", func() {
			var resp *apiv1.ListPageVersionsResponse
			var err error

			BeforeEach(func() {
				fixedTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
				hr := &MockPageHistoryReader{
					Versions: []v1.PageVersionMetadata{
						{VersionID: "v1", CreatedAt: fixedTime, Author: "alice", IsAgent: false, Source: "web", SHA256: "abc", ByteSize: 100},
						{VersionID: "v2", CreatedAt: fixedTime, Author: "bob", IsAgent: true, Source: "api", SHA256: "def", ByteSize: 200},
					},
				}
				s := newHistoryServer(hr, nil)
				resp, err = s.ListPageVersions(ctx, &apiv1.ListPageVersionsRequest{Page: "my-page"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return all versions", func() {
				Expect(resp.GetVersions()).To(HaveLen(2))
			})

			It("should map version fields correctly", func() {
				v := resp.GetVersions()[0]
				Expect(v.GetVersionId()).To(Equal("v1"))
				Expect(v.GetAuthor()).To(Equal("alice"))
				Expect(v.GetIsAgent()).To(BeFalse())
				Expect(v.GetSource()).To(Equal("web"))
				Expect(v.GetSha256()).To(Equal("abc"))
				Expect(v.GetByteSize()).To(BeEquivalentTo(100))
			})
		})

		When("limit is set", func() {
			var resp *apiv1.ListPageVersionsResponse
			var err error

			BeforeEach(func() {
				fixedTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
				hr := &MockPageHistoryReader{
					Versions: []v1.PageVersionMetadata{
						{VersionID: "v1", CreatedAt: fixedTime},
						{VersionID: "v2", CreatedAt: fixedTime},
						{VersionID: "v3", CreatedAt: fixedTime},
					},
				}
				s := newHistoryServer(hr, nil)
				resp, err = s.ListPageVersions(ctx, &apiv1.ListPageVersionsRequest{Page: "my-page", Limit: 2})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return only the limited number of versions", func() {
				Expect(resp.GetVersions()).To(HaveLen(2))
			})
		})
	})

	Describe("ReadPageVersion", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.ReadPageVersion(ctx, &apiv1.ReadPageVersionRequest{VersionId: "v1"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page is required"))
			})
		})

		When("version_id is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.ReadPageVersion(ctx, &apiv1.ReadPageVersionRequest{Page: "my-page"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "version_id is required"))
			})
		})

		When("historyReader is not configured", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.ReadPageVersion(ctx, &apiv1.ReadPageVersionRequest{Page: "my-page", VersionId: "v1"})
			})

			It("should return Unavailable", func() {
				Expect(err).To(HaveGrpcStatus(codes.Unavailable, "history reader not configured"))
			})
		})

		When("historyReader returns an error", func() {
			var err error

			BeforeEach(func() {
				hr := &MockPageHistoryReader{ReadErr: errors.New("not found")}
				s := newHistoryServer(hr, nil)
				_, err = s.ReadPageVersion(ctx, &apiv1.ReadPageVersionRequest{Page: "my-page", VersionId: "v1"})
			})

			It("should return Internal", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read version"))
			})
		})

		When("historyReader returns content", func() {
			var resp *apiv1.ReadPageVersionResponse
			var err error

			BeforeEach(func() {
				hr := &MockPageHistoryReader{ReadContent: "# Hello\n\nWorld"}
				s := newHistoryServer(hr, nil)
				resp, err = s.ReadPageVersion(ctx, &apiv1.ReadPageVersionRequest{Page: "my-page", VersionId: "v1"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the content", func() {
				Expect(resp.GetContent()).To(Equal("# Hello\n\nWorld"))
			})
		})
	})

	Describe("RestorePageVersion", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.RestorePageVersion(ctx, &apiv1.RestorePageVersionRequest{VersionId: "v1"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page is required"))
			})
		})

		When("version_id is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.RestorePageVersion(ctx, &apiv1.RestorePageVersionRequest{Page: "my-page"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "version_id is required"))
			})
		})

		When("historyReader is not configured", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.RestorePageVersion(ctx, &apiv1.RestorePageVersionRequest{Page: "my-page", VersionId: "v1"})
			})

			It("should return Unavailable", func() {
				Expect(err).To(HaveGrpcStatus(codes.Unavailable, "history reader not configured"))
			})
		})

		When("historyReader returns an error", func() {
			var err error

			BeforeEach(func() {
				hr := &MockPageHistoryReader{RestoreErr: errors.New("cannot restore")}
				s := newHistoryServer(hr, nil)
				_, err = s.RestorePageVersion(ctx, &apiv1.RestorePageVersionRequest{Page: "my-page", VersionId: "v1"})
			})

			It("should return Internal", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to restore version"))
			})
		})

		When("restore succeeds", func() {
			var resp *apiv1.RestorePageVersionResponse
			var err error

			BeforeEach(func() {
				hr := &MockPageHistoryReader{}
				s := newHistoryServer(hr, nil)
				resp, err = s.RestorePageVersion(ctx, &apiv1.RestorePageVersionRequest{Page: "my-page", VersionId: "v1"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty response", func() {
				Expect(resp).NotTo(BeNil())
			})
		})
	})

	Describe("DiffPageVersions", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.DiffPageVersions(ctx, &apiv1.DiffPageVersionsRequest{OldVersionId: "v1", NewVersionId: "v2"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page is required"))
			})
		})

		When("old_version_id is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.DiffPageVersions(ctx, &apiv1.DiffPageVersionsRequest{Page: "my-page", NewVersionId: "v2"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "old_version_id is required"))
			})
		})

		When("new_version_id is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.DiffPageVersions(ctx, &apiv1.DiffPageVersionsRequest{Page: "my-page", OldVersionId: "v1"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "new_version_id is required"))
			})
		})

		When("historyReader is not configured", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.DiffPageVersions(ctx, &apiv1.DiffPageVersionsRequest{Page: "my-page", OldVersionId: "v1", NewVersionId: "v2"})
			})

			It("should return Unavailable", func() {
				Expect(err).To(HaveGrpcStatus(codes.Unavailable, "history reader not configured"))
			})
		})

		When("historyReader returns an error", func() {
			var err error

			BeforeEach(func() {
				hr := &MockPageHistoryReader{DiffErr: errors.New("cannot diff")}
				s := newHistoryServer(hr, nil)
				_, err = s.DiffPageVersions(ctx, &apiv1.DiffPageVersionsRequest{Page: "my-page", OldVersionId: "v1", NewVersionId: "v2"})
			})

			It("should return Internal", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to diff versions"))
			})
		})

		When("diff succeeds", func() {
			var resp *apiv1.DiffPageVersionsResponse
			var err error

			BeforeEach(func() {
				hr := &MockPageHistoryReader{Diff: "--- v1\n+++ v2\n@@ -1 +1 @@\n-old\n+new"}
				s := newHistoryServer(hr, nil)
				resp, err = s.DiffPageVersions(ctx, &apiv1.DiffPageVersionsRequest{Page: "my-page", OldVersionId: "v1", NewVersionId: "v2"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the diff", func() {
				Expect(resp.GetDiff()).To(ContainSubstring("--- v1"))
			})
		})
	})

	Describe("SearchPageHistory", func() {
		When("page is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.SearchPageHistory(ctx, &apiv1.SearchPageHistoryRequest{Query: "foo"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page is required"))
			})
		})

		When("query is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.SearchPageHistory(ctx, &apiv1.SearchPageHistoryRequest{Page: "my-page"})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "query is required"))
			})
		})

		When("historySearcher is not configured", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.SearchPageHistory(ctx, &apiv1.SearchPageHistoryRequest{Page: "my-page", Query: "foo"})
			})

			It("should return Unavailable", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Unavailable, "history search not configured"))
			})
		})

		When("historySearcher returns an error", func() {
			var err error

			BeforeEach(func() {
				hs := &MockHistorySearcher{PageErr: errors.New("index unavailable")}
				s := newHistoryServer(nil, hs)
				_, err = s.SearchPageHistory(ctx, &apiv1.SearchPageHistoryRequest{Page: "my-page", Query: "foo"})
			})

			It("should return Internal", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to search page history"))
			})
		})

		When("historySearcher returns results", func() {
			var resp *apiv1.SearchPageHistoryResponse
			var err error

			BeforeEach(func() {
				fixedTime := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
				hs := &MockHistorySearcher{
					PageResults: []v1.HistorySearchResult{
						{Page: "my-page", Version: v1.PageVersionMetadata{VersionID: "v1", CreatedAt: fixedTime, Author: "alice"}, Snippet: "found here"},
					},
				}
				s := newHistoryServer(nil, hs)
				resp, err = s.SearchPageHistory(ctx, &apiv1.SearchPageHistoryRequest{Page: "my-page", Query: "foo"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the results", func() {
				Expect(resp.GetResults()).To(HaveLen(1))
			})

			It("should map result fields", func() {
				r := resp.GetResults()[0]
				Expect(r.GetPage()).To(Equal("my-page"))
				Expect(r.GetSnippet()).To(Equal("found here"))
				Expect(r.GetVersion().GetVersionId()).To(Equal("v1"))
				Expect(r.GetVersion().GetAuthor()).To(Equal("alice"))
			})
		})
	})

	Describe("SearchHistory", func() {
		When("query is empty", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.SearchHistory(ctx, &apiv1.SearchHistoryRequest{})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "query is required"))
			})
		})

		When("historySearcher is not configured", func() {
			var err error

			BeforeEach(func() {
				s := newHistoryServer(nil, nil)
				_, err = s.SearchHistory(ctx, &apiv1.SearchHistoryRequest{Query: "foo"})
			})

			It("should return Unavailable", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Unavailable, "history search not configured"))
			})
		})

		When("historySearcher returns an error", func() {
			var err error

			BeforeEach(func() {
				hs := &MockHistorySearcher{Err: errors.New("index offline")}
				s := newHistoryServer(nil, hs)
				_, err = s.SearchHistory(ctx, &apiv1.SearchHistoryRequest{Query: "foo"})
			})

			It("should return Internal", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to search history"))
			})
		})

		When("historySearcher returns results with from/to filters", func() {
			var resp *apiv1.SearchHistoryResponse
			var err error

			BeforeEach(func() {
				fixedTime := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
				hs := &MockHistorySearcher{
					Results: []v1.HistorySearchResult{
						{Page: "page-a", Version: v1.PageVersionMetadata{VersionID: "va", CreatedAt: fixedTime, Author: "carol"}, Snippet: "match here"},
					},
				}
				s := newHistoryServer(nil, hs)
				from := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
				to := timestamppb.New(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
				resp, err = s.SearchHistory(ctx, &apiv1.SearchHistoryRequest{
					Query:        "foo",
					PageFilter:   "page-a",
					AuthorFilter: "carol",
					From:         from,
					To:           to,
				})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the results", func() {
				Expect(resp.GetResults()).To(HaveLen(1))
			})

			It("should map result fields", func() {
				r := resp.GetResults()[0]
				Expect(r.GetPage()).To(Equal("page-a"))
				Expect(r.GetSnippet()).To(Equal("match here"))
				Expect(r.GetVersion().GetAuthor()).To(Equal("carol"))
			})
		})

		When("historySearcher returns results without from/to", func() {
			var resp *apiv1.SearchHistoryResponse
			var err error

			BeforeEach(func() {
				hs := &MockHistorySearcher{
					Results: []v1.HistorySearchResult{},
				}
				s := newHistoryServer(nil, hs)
				resp, err = s.SearchHistory(ctx, &apiv1.SearchHistoryRequest{Query: "nothing"})
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty result list", func() {
				Expect(resp.GetResults()).To(BeEmpty())
			})
		})
	})
})
