package index

import (
	"errors"

	"github.com/brendanjerwin/simple_wiki/common"
)

type MultiMaintainer struct {
	Maintainers []IMaintainIndex
}

func NewMultiMaintainer(maintainers ...IMaintainIndex) *MultiMaintainer {
	return &MultiMaintainer{Maintainers: maintainers}
}

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
