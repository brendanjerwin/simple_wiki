// Package index provides interfaces for indexing wiki pages.
package index

import (
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// IMaintainIndex defines the interface for maintaining a wiki page index.
type IMaintainIndex interface {
	AddPageToIndex(identifier wikipage.PageIdentifier) error
	RemovePageFromIndex(identifier wikipage.PageIdentifier) error
}

// IProvideIndexName defines the interface for indexes that can provide their name.
type IProvideIndexName interface {
	GetIndexName() string
}
