package index

import (
	"errors"

	"github.com/brendanjerwin/simple_wiki/common"
)

// MultiMaintainer is an index maintainer that delegates to multiple other maintainers.
type MultiMaintainer struct {
	Maintainers []IMaintainIndex
}

// NewMultiMaintainer creates a new MultiMaintainer.
func NewMultiMaintainer(maintainers ...IMaintainIndex) *MultiMaintainer {
	return &MultiMaintainer{Maintainers: maintainers}
}

// AddPageToIndex adds a page to all underlying indexes.
func (m *MultiMaintainer) AddPageToIndex(identifier common.PageIdentifier) error {
	errs := []error{}
	for _, maintainer := range m.Maintainers {
		err := maintainer.AddPageToIndex(identifier)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// RemovePageFromIndex removes a page from all underlying indexes.
func (m *MultiMaintainer) RemovePageFromIndex(identifier common.PageIdentifier) error {
	errs := []error{}
	for _, maintainer := range m.Maintainers {
		err := maintainer.RemovePageFromIndex(identifier)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
