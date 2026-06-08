package bleve

import (
	"testing"
	"time"

	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

type concurrencyTestPageReader struct{}

func (concurrencyTestPageReader) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return identifier, wikipage.FrontMatter{}, nil
}

func (concurrencyTestPageReader) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return identifier, "", nil
}

func TestQueryDoesNotWaitForIndexerMutationLock(t *testing.T) {
	t.Parallel()

	reader := concurrencyTestPageReader{}
	index, err := NewIndex(reader, frontmatter.NewIndex(reader))
	if err != nil {
		t.Fatalf("NewIndex() error = %v", err)
	}

	index.mu.Lock()
	defer index.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, queryErr := index.Query("home")
		done <- queryErr
	}()

	select {
	case queryErr := <-done:
		if queryErr != nil {
			t.Fatalf("Query() error = %v", queryErr)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Query() blocked on the indexer mutation lock")
	}
}
