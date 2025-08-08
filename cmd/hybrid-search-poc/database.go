package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rikonor/go-ann"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// VectorDB wraps SQLite with vector search capabilities
type VectorDB struct {
	db        *sql.DB
	annIndex  ann.ANNer
	vectors   [][]float64
	vectorIDs []int64
	vectorDim int
}

// ContentItem represents a piece of content with metadata
type ContentItem struct {
	ID      int64
	Title   string
	Content string
}

// DatabaseStats holds database statistics
type DatabaseStats struct {
	ContentCount     int64
	VectorDimensions int64
	DatabaseSize     string
	SQLiteVersion    string
	SqliteVecVersion string
}

// NewVectorDB creates a new vector database instance
func NewVectorDB() (*VectorDB, error) {
	// Open SQLite database using modernc.org/sqlite (pure Go)
	sqlDB, err := sql.Open("sqlite", "hybrid_search_poc.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize vector storage with 1024 dimensions
	vectorDim := 1024
	
	db := &VectorDB{
		db:        sqlDB,
		vectors:   [][]float64{},
		vectorIDs: []int64{},
		vectorDim: vectorDim,
	}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Load existing vectors into memory if any exist
	if err := db.loadVectorsToMemory(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to load vectors to memory: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *VectorDB) Close() error {
	if db.db != nil {
		return db.db.Close()
	}
	return nil
}

// initSchema creates the necessary tables
func (db *VectorDB) initSchema() error {
	// Create content table
	_, err := db.db.Exec(`
		CREATE TABLE IF NOT EXISTS content (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create content table: %w", err)
	}

	// Create embeddings table to store vectors as JSON
	_, err = db.db.Exec(`
		CREATE TABLE IF NOT EXISTS content_embeddings (
			content_id INTEGER PRIMARY KEY,
			embedding TEXT NOT NULL,
			FOREIGN KEY (content_id) REFERENCES content(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create embeddings table: %w", err)
	}

	// Create configuration table to store provider settings
	_, err = db.db.Exec(`
		CREATE TABLE IF NOT EXISTS configuration (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create configuration table: %w", err)
	}

	return nil
}

// loadVectorsToMemory loads existing vectors from database into memory
func (db *VectorDB) loadVectorsToMemory() error {
	rows, err := db.db.Query("SELECT content_id, embedding FROM content_embeddings ORDER BY content_id")
	if err != nil {
		return fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contentID int64
		var embeddingJSON string
		
		if err := rows.Scan(&contentID, &embeddingJSON); err != nil {
			return fmt.Errorf("failed to scan embedding row: %w", err)
		}
		
		var embedding []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
			return fmt.Errorf("failed to unmarshal embedding for content %d: %w", contentID, err)
		}
		
		// Convert float32 to float64 for go-ann
		vector := make([]float64, len(embedding))
		for i, v := range embedding {
			vector[i] = float64(v)
		}
		
		// Add vector to memory
		db.vectors = append(db.vectors, vector)
		db.vectorIDs = append(db.vectorIDs, contentID)
	}
	
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over embeddings: %w", err)
	}
	
	// Build the ANN index if we loaded any vectors
	if len(db.vectors) > 0 {
		db.annIndex = ann.NewMRPTANNer(5, 10, db.vectors) // 5 trees, depth 10
	}
	
	return nil
}

// rebuildANNIndex rebuilds the ANN index from current vectors
func (db *VectorDB) rebuildANNIndex() {
	if len(db.vectors) > 0 {
		db.annIndex = ann.NewMRPTANNer(5, 10, db.vectors)
	} else {
		db.annIndex = nil
	}
}

// AddContent adds content with its vector embedding
func (db *VectorDB) AddContent(title, content string, embedding []float32) error {
	// Start transaction
	tx, err := db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	// Set up rollback on error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert content and get the ID
	result, err := tx.Exec(`
		INSERT INTO content (title, content) VALUES (?, ?)
	`, title, content)
	if err != nil {
		return fmt.Errorf("failed to insert content: %w", err)
	}
	
	contentID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get inserted content ID: %w", err)
	}
	
	// Serialize embedding as JSON
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	// Insert embedding
	_, err = tx.Exec(`
		INSERT INTO content_embeddings (content_id, embedding) VALUES (?, ?)
	`, contentID, string(embeddingJSON))
	if err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}

	// Convert float32 to float64 and add to memory
	vector := make([]float64, len(embedding))
	for i, v := range embedding {
		vector[i] = float64(v)
	}
	db.vectors = append(db.vectors, vector)
	db.vectorIDs = append(db.vectorIDs, contentID)
	
	// Rebuild the ANN index (in production, you'd do this less frequently)
	db.rebuildANNIndex()

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// KeywordSearch performs traditional keyword search
func (db *VectorDB) KeywordSearch(query string, limit int) ([]SearchResult, error) {
	queryPattern := "%" + strings.ToLower(query) + "%"
	
	rows, err := db.db.Query(`
		SELECT id, title, content 
		FROM content 
		WHERE LOWER(title) LIKE ? OR LOWER(content) LIKE ?
		ORDER BY CASE 
			WHEN LOWER(title) LIKE ? THEN 1 
			ELSE 2 
		END
		LIMIT ?
	`, queryPattern, queryPattern, queryPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute keyword search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.ID, &result.Title, &result.Content); err != nil {
			return nil, fmt.Errorf("failed to scan keyword search result: %w", err)
		}
		
		result.Score = 0.8 // Fixed score for keyword matches
		result.SearchType = "keyword"
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating keyword search results: %w", err)
	}

	return results, nil
}

// VectorSearch performs semantic vector search using the go-ann index
func (db *VectorDB) VectorSearch(queryEmbedding []float32, limit int) ([]SearchResult, error) {
	// Check if we have any vectors in the index
	if db.annIndex == nil || len(db.vectors) == 0 {
		return []SearchResult{}, nil
	}
	
	// Convert query embedding to float64
	query := make([]float64, len(queryEmbedding))
	for i, v := range queryEmbedding {
		query[i] = float64(v)
	}
	
	// Find similar vectors using ANN index (limit to available vectors)
	requestLimit := limit
	if requestLimit > len(db.vectors) {
		requestLimit = len(db.vectors)
	}
	similarIndices := db.annIndex.ANN(query, requestLimit)
	
	if len(similarIndices) == 0 {
		return []SearchResult{}, nil
	}
	
	// Convert vector indices to content IDs
	var contentIDs []interface{}
	for _, idx := range similarIndices {
		if idx >= 0 && idx < len(db.vectorIDs) {
			contentIDs = append(contentIDs, db.vectorIDs[idx])
		}
	}
	
	if len(contentIDs) == 0 {
		return []SearchResult{}, nil
	}

	// Build SQL query with placeholders
	placeholders := make([]string, len(contentIDs))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	sqlQuery := fmt.Sprintf(`
		SELECT id, title, content 
		FROM content 
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.db.Query(sqlQuery, contentIDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search query: %w", err)
	}
	defer rows.Close()

	// Execute query and collect results
	var results []SearchResult
	idToResult := make(map[int64]*SearchResult)

	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.ID, &result.Title, &result.Content); err != nil {
			return nil, fmt.Errorf("failed to scan vector search result: %w", err)
		}

		result.SearchType = "semantic"
		results = append(results, result)
		idToResult[result.ID] = &results[len(results)-1]
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vector search results: %w", err)
	}

	// Calculate similarity scores based on ranking
	for i, idx := range similarIndices {
		if idx >= 0 && idx < len(db.vectorIDs) {
			contentID := db.vectorIDs[idx]
			if result, exists := idToResult[contentID]; exists {
				// Simple ranking-based score (higher rank = higher score)
				result.Score = 1.0 - (float32(i) / float32(len(similarIndices)))
			}
		}
	}

	// Sort results by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// GetContentCount returns the number of content items
func (db *VectorDB) GetContentCount() (int64, error) {
	var count int64
	err := db.db.QueryRow("SELECT COUNT(*) FROM content").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get count: %w", err)
	}
	return count, nil
}

// GetStats returns database statistics
func (db *VectorDB) GetStats() (*DatabaseStats, error) {
	stats := &DatabaseStats{}

	// Get content count
	count, err := db.GetContentCount()
	if err != nil {
		return nil, err
	}
	stats.ContentCount = count

	// Get SQLite version
	var version string
	err = db.db.QueryRow("SELECT sqlite_version()").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get SQLite version: %w", err)
	}
	stats.SQLiteVersion = version

	// Update for go-ann implementation
	vectorCount := len(db.vectors)
	stats.SqliteVecVersion = fmt.Sprintf("go-ann (MRPT index with %d vectors)", vectorCount)

	// Set values for ANN-based stats
	stats.VectorDimensions = int64(db.vectorDim)
	stats.DatabaseSize = "File: hybrid_search_poc.db"

	return stats, nil
}

// StoreProviderConfig stores the provider configuration in the database
func (db *VectorDB) StoreProviderConfig(provider, model, baseURL string, vectorDim int) error {
	tx, err := db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Store provider configuration including vector dimensions
	configs := map[string]string{
		"embedding_provider":   provider,
		"embedding_model":      model,
		"embedding_base_url":   baseURL,
		"embedding_dimensions": fmt.Sprintf("%d", vectorDim),
	}

	for key, value := range configs {
		_, err = tx.Exec(`
			INSERT OR REPLACE INTO configuration (key, value, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
		`, key, value)
		if err != nil {
			return fmt.Errorf("failed to store config %s: %w", key, err)
		}
	}

	return tx.Commit()
}

// LoadProviderConfig loads the stored provider configuration from the database
func (db *VectorDB) LoadProviderConfig() (provider, model, baseURL string, vectorDim int, err error) {
	rows, err := db.db.Query(`
		SELECT key, value FROM configuration 
		WHERE key IN ('embedding_provider', 'embedding_model', 'embedding_base_url', 'embedding_dimensions')
	`)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to query configuration: %w", err)
	}
	defer rows.Close()

	configs := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return "", "", "", 0, fmt.Errorf("failed to scan configuration row: %w", err)
		}
		configs[key] = value
	}

	if err := rows.Err(); err != nil {
		return "", "", "", 0, fmt.Errorf("error iterating configuration rows: %w", err)
	}

	// Parse vector dimensions
	dimensions := 1024 // default fallback
	if dimStr, exists := configs["embedding_dimensions"]; exists && dimStr != "" {
		if parsed, err := fmt.Sscanf(dimStr, "%d", &dimensions); err != nil || parsed != 1 {
			dimensions = 1024 // fallback on parse error
		}
	}

	return configs["embedding_provider"], configs["embedding_model"], configs["embedding_base_url"], dimensions, nil
}

// HasProviderConfig checks if provider configuration exists in the database
func (db *VectorDB) HasProviderConfig() (bool, error) {
	var count int
	err := db.db.QueryRow(`
		SELECT COUNT(*) FROM configuration 
		WHERE key = 'embedding_provider'
	`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check provider config: %w", err)
	}
	return count > 0, nil
}