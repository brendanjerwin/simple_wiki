package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWikiCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "wiki-cli Suite")
}

var _ = Describe("commitsMatch", func() {

	When("the server reports a raw hash matching the CLI commit", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123", "adbef9d2abc123")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the server reports a tagged format like 'v3.5.0 (adbef9d)'", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123def456", "v3.5.0 (adbef9d)")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the server reports a short hash that is a prefix of the CLI commit", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123def456", "adbef9d2")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the CLI commit is shorter than the server hash", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2", "adbef9d2abc123def456")
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the commits do not match at all", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123", "ffff1234567890")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the server reports a tagged format with a non-matching hash", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123", "v3.5.0 (ffff123)")
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the server reports a tagged format with no closing parenthesis", func() {
		var result bool

		BeforeEach(func() {
			result = commitsMatch("adbef9d2abc123", "v3.5.0 (adbef9d")
		})

		It("should fall back to full string comparison and return false", func() {
			Expect(result).To(BeFalse())
		})
	})
})

var _ = Describe("checkVersionCompatibility", func() {
	var originalCommit string

	BeforeEach(func() {
		originalCommit = commit
	})

	AfterEach(func() {
		commit = originalCommit
	})

	When("commit is 'dev'", func() {
		var err error

		BeforeEach(func() {
			commit = "dev"
			err = checkVersionCompatibility("http://localhost:99999")
		})

		It("should skip the check and return nil", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the server returns a matching commit", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			commit = "adbef9d2abc123def456"
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				resp := versionResponse{Commit: "v3.5.1 (adbef9d)"}
				_ = json.NewEncoder(w).Encode(resp)
			}))

			err = checkVersionCompatibility(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return nil", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the server returns a mismatched commit", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			commit = "adbef9d2abc123def456"
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				resp := versionResponse{Commit: "ffff1234567890"}
				_ = json.NewEncoder(w).Encode(resp)
			}))

			err = checkVersionCompatibility(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return a VERSION MISMATCH error", func() {
			Expect(err).To(MatchError(ContainSubstring("VERSION MISMATCH")))
		})
	})

	When("the server returns an empty commit", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			commit = "adbef9d2abc123def456"
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				resp := versionResponse{Commit: ""}
				_ = json.NewEncoder(w).Encode(resp)
			}))

			err = checkVersionCompatibility(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return nil (skip comparison)", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the server is unreachable", func() {
		var err error

		BeforeEach(func() {
			commit = "adbef9d2abc123def456"
			err = checkVersionCompatibility("http://127.0.0.1:1")
		})

		It("should return an UNREACHABLE error", func() {
			Expect(err).To(MatchError(ContainSubstring("UNREACHABLE")))
		})
	})

	When("the server returns a non-200 status", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			commit = "adbef9d2abc123def456"
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))

			err = checkVersionCompatibility(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return an UNREACHABLE error with the status code", func() {
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("HTTP %d", http.StatusInternalServerError))))
		})
	})

	When("the server returns invalid JSON", func() {
		var err error
		var server *httptest.Server

		BeforeEach(func() {
			commit = "adbef9d2abc123def456"
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("not json"))
			}))

			err = checkVersionCompatibility(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should return an UNREACHABLE error about invalid response", func() {
			Expect(err).To(MatchError(ContainSubstring("invalid version response")))
		})
	})
})
