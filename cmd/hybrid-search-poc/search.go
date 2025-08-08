package main

import (
	"fmt"
	"sort"
)

// SearchResult represents a search result with metadata
type SearchResult struct {
	ID         int64
	Title      string
	Content    string
	Score      float32
	SearchType string // "keyword", "semantic", or "hybrid"
}

// SearchMode defines the type of search to perform
type SearchMode string

const (
	SearchModeHybrid   SearchMode = "hybrid"
	SearchModeKeyword  SearchMode = "keyword"
	SearchModeSemantic SearchMode = "semantic"
)

// SearchHybrid performs hybrid search combining keyword and semantic results
func SearchHybrid(db *VectorDB, embedder EmbeddingProvider, query string, mode string, semanticWeight float32, limit int) ([]SearchResult, error) {
	searchMode := SearchMode(mode)
	
	switch searchMode {
	case SearchModeKeyword:
		return db.KeywordSearch(query, limit)
		
	case SearchModeSemantic:
		queryEmbedding, err := embedder.GenerateEmbedding(query)
		if err != nil {
			return nil, fmt.Errorf("failed to generate query embedding: %w", err)
		}
		return db.VectorSearch(queryEmbedding, limit)
		
	case SearchModeHybrid:
		return performHybridSearch(db, embedder, query, semanticWeight, limit)
		
	default:
		return nil, fmt.Errorf("unsupported search mode: %s", mode)
	}
}

// performHybridSearch combines keyword and semantic search results
func performHybridSearch(db *VectorDB, embedder EmbeddingProvider, query string, semanticWeight float32, limit int) ([]SearchResult, error) {
	keywordWeight := 1.0 - semanticWeight
	
	// Perform keyword search (use reasonable limit for fusion)
	keywordLimit := limit + 5 // Get a few extra for better fusion
	keywordResults, err := db.KeywordSearch(query, keywordLimit)
	if err != nil {
		return nil, fmt.Errorf("keyword search failed: %w", err)
	}
	
	// Generate embedding for semantic search
	queryEmbedding, err := embedder.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}
	
	// Perform semantic search (use same conservative limit)
	semanticResults, err := db.VectorSearch(queryEmbedding, keywordLimit)
	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}
	
	// Fuse results using Reciprocal Rank Fusion (RRF)
	fusedResults := reciprocalRankFusion(keywordResults, semanticResults, keywordWeight, semanticWeight)
	
	// Limit results
	if len(fusedResults) > limit {
		fusedResults = fusedResults[:limit]
	}
	
	// Mark results as hybrid
	for i := range fusedResults {
		fusedResults[i].SearchType = "hybrid"
	}
	
	return fusedResults, nil
}

// reciprocalRankFusion combines two result sets using RRF algorithm
func reciprocalRankFusion(keywordResults, semanticResults []SearchResult, keywordWeight, semanticWeight float32) []SearchResult {
	const k = 60.0 // RRF parameter
	
	// Create maps for efficient lookup
	resultMap := make(map[int64]*SearchResult)
	keywordRanks := make(map[int64]int)
	semanticRanks := make(map[int64]int)
	
	// Index keyword results by rank
	for i, result := range keywordResults {
		keywordRanks[result.ID] = i + 1 // 1-based ranking
		if _, exists := resultMap[result.ID]; !exists {
			// Create a copy of the result
			resultCopy := result
			resultMap[result.ID] = &resultCopy
		}
	}
	
	// Index semantic results by rank
	for i, result := range semanticResults {
		semanticRanks[result.ID] = i + 1 // 1-based ranking
		if _, exists := resultMap[result.ID]; !exists {
			// Create a copy of the result
			resultCopy := result
			resultMap[result.ID] = &resultCopy
		}
	}
	
	// Calculate RRF scores
	for id, result := range resultMap {
		var score float32 = 0.0
		
		// Add keyword component
		if rank, exists := keywordRanks[id]; exists {
			score += keywordWeight * (1.0 / (k + float32(rank)))
		}
		
		// Add semantic component  
		if rank, exists := semanticRanks[id]; exists {
			score += semanticWeight * (1.0 / (k + float32(rank)))
		}
		
		result.Score = score
	}
	
	// Convert map to slice and sort by score
	var results []SearchResult
	for _, result := range resultMap {
		results = append(results, *result)
	}
	
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	
	return results
}

// rankBoostFusion is an alternative fusion method that uses score boosting
func rankBoostFusion(keywordResults, semanticResults []SearchResult, keywordWeight, semanticWeight float32) []SearchResult {
	resultMap := make(map[int64]*SearchResult)
	
	// Process keyword results
	for _, result := range keywordResults {
		if _, exists := resultMap[result.ID]; !exists {
			resultCopy := result
			resultCopy.Score = result.Score * keywordWeight
			resultMap[result.ID] = &resultCopy
		}
	}
	
	// Process semantic results  
	for _, result := range semanticResults {
		if existing, exists := resultMap[result.ID]; exists {
			// Combine scores
			existing.Score += result.Score * semanticWeight
		} else {
			// New result from semantic search
			resultCopy := result
			resultCopy.Score = result.Score * semanticWeight
			resultMap[result.ID] = &resultCopy
		}
	}
	
	// Convert to slice and sort
	var results []SearchResult
	for _, result := range resultMap {
		results = append(results, *result)
	}
	
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	
	return results
}

// ScoreNormalizer normalizes scores to a 0-1 range
type ScoreNormalizer struct {
	minScore float32
	maxScore float32
}

// NewScoreNormalizer creates a new score normalizer from a set of results
func NewScoreNormalizer(results []SearchResult) *ScoreNormalizer {
	if len(results) == 0 {
		return &ScoreNormalizer{0, 1}
	}
	
	min := results[0].Score
	max := results[0].Score
	
	for _, result := range results {
		if result.Score < min {
			min = result.Score
		}
		if result.Score > max {
			max = result.Score
		}
	}
	
	return &ScoreNormalizer{min, max}
}

// Normalize normalizes a score to the 0-1 range
func (n *ScoreNormalizer) Normalize(score float32) float32 {
	if n.maxScore == n.minScore {
		return 1.0 // All scores are the same
	}
	
	return (score - n.minScore) / (n.maxScore - n.minScore)
}

// NormalizeResults normalizes all scores in a result set
func (n *ScoreNormalizer) NormalizeResults(results []SearchResult) {
	for i := range results {
		results[i].Score = n.Normalize(results[i].Score)
	}
}