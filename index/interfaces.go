// Package index provides interfaces for indexing wiki pages.
package index

import (
	"time"

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

// IProvideIndexingProgress defines the interface for components that can provide indexing progress.
type IProvideIndexingProgress interface {
	GetProgress() IndexingProgress
}

// IndexingProgress represents the progress of indexing operations.
type IndexingProgress struct {
	IsRunning           bool
	TotalPages          int
	CompletedPages      int
	QueueDepth          int
	ProcessingRatePerSecond float64
	EstimatedCompletion *time.Duration
	IndexProgress       map[string]SingleIndexProgress
}

// SingleIndexProgress represents progress for a single index type.
type SingleIndexProgress struct {
	Name                string
	Completed           int
	Total               int
	ProcessingRatePerSecond float64
	LastError           *string
	QueueDepth          int
	WorkDistributionComplete bool
}
