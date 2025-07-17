// Package index provides interfaces for indexing wiki pages.
package index

import (
	"github.com/brendanjerwin/simple_wiki/common"
)

// IMaintainIndex defines the interface for maintaining a wiki page index.
type IMaintainIndex interface {
	AddPageToIndex(identifier common.PageIdentifier) error
	RemovePageFromIndex(identifier common.PageIdentifier) error
}
