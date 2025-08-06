package index

import (
	"fmt"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Operation represents the type of index operation to perform.
type Operation int

const (
	// Add operation adds a page to the index.
	Add Operation = iota
	// Remove operation removes a page from the index.
	Remove
)

// IndexOperator defines the interface for index operations.
type IndexOperator interface {
	AddPageToIndex(identifier wikipage.PageIdentifier) error
	RemovePageFromIndex(identifier wikipage.PageIdentifier) error
}

// FrontmatterIndexJob wraps frontmatter index operations as a job.
type FrontmatterIndexJob struct {
	index      IndexOperator
	identifier wikipage.PageIdentifier
	operation  Operation
}

// NewFrontmatterIndexJob creates a new FrontmatterIndexJob.
func NewFrontmatterIndexJob(index IndexOperator, identifier wikipage.PageIdentifier, operation Operation) *FrontmatterIndexJob {
	return &FrontmatterIndexJob{
		index:      index,
		identifier: identifier,
		operation:  operation,
	}
}

// Execute implements the Job interface for FrontmatterIndexJob.
func (f *FrontmatterIndexJob) Execute() error {
	switch f.operation {
	case Add:
		return f.index.AddPageToIndex(f.identifier)
	case Remove:
		return f.index.RemovePageFromIndex(f.identifier)
	default:
		return fmt.Errorf("unknown operation type: %d", f.operation)
	}
}

// GetName implements the Job interface for FrontmatterIndexJob.
func (*FrontmatterIndexJob) GetName() string {
	return "FrontmatterIndex"
}

// BleveIndexJob wraps bleve index operations as a job.
type BleveIndexJob struct {
	index      IndexOperator
	identifier wikipage.PageIdentifier
	operation  Operation
}

// NewBleveIndexJob creates a new BleveIndexJob.
func NewBleveIndexJob(index IndexOperator, identifier wikipage.PageIdentifier, operation Operation) *BleveIndexJob {
	return &BleveIndexJob{
		index:      index,
		identifier: identifier,
		operation:  operation,
	}
}

// Execute implements the Job interface for BleveIndexJob.
func (b *BleveIndexJob) Execute() error {
	switch b.operation {
	case Add:
		return b.index.AddPageToIndex(b.identifier)
	case Remove:
		return b.index.RemovePageFromIndex(b.identifier)
	default:
		return fmt.Errorf("unknown operation type: %d", b.operation)
	}
}

// GetName implements the Job interface for BleveIndexJob.
func (*BleveIndexJob) GetName() string {
	return "BleveIndex"
}