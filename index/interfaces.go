package index

import (
	"github.com/brendanjerwin/simple_wiki/common"
)

type IMaintainIndex interface {
	AddPageToIndex(identifier common.PageIdentifier) error
	RemovePageFromIndex(identifier common.PageIdentifier) error
}
