package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Styling with lipgloss ✨
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
	
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))
	
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("160")).
			Bold(true)
	
	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Italic(true)
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "hybrid-search-poc",
		Short: "POC for SQLite + go-ann hybrid search capabilities", 
		Long:  titleStyle.Render("🔍 Hybrid Search POC") + "\n\nA proof-of-concept demonstrating SQLite + go-ann (MRPT) with semantic search capabilities.",
	}

	// Database initialization command
	var initCmd = &cobra.Command{
		Use:   "init [provider]",
		Short: "Initialize the database and embedding provider",
		Long:  "Set up the SQLite database with go-ann vector search and configure the embedding provider. Defaults to 'ollama' if no provider specified.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  initDatabase,
	}
	
	// Add content command
	var addCmd = &cobra.Command{
		Use:   "add [title] [content]",
		Short: "Add content with embeddings to the database",
		Long:  "Add a piece of content along with its vector embedding to the database.",
		Args:  cobra.ExactArgs(2),
		RunE:  addContent,
	}
	
	// Bulk add command
	var bulkAddCmd = &cobra.Command{
		Use:   "bulk-add [files...]",
		Short: "Add multiple content items",
		Long:  "Add sample content items or import from files. Supports glob patterns like '*.md'. If no files specified, adds predefined sample data.",
		RunE:  bulkAddContent,
	}
	
	// Search command
	var searchCmd = &cobra.Command{
		Use:   "search [query]",
		Short: "Search content using hybrid keyword + semantic search",
		Long:  "Search through content using a combination of keyword and semantic similarity matching.",
		Args:  cobra.ExactArgs(1),
		RunE:  searchContent,
	}
	
	// Info command
	var infoCmd = &cobra.Command{
		Use:   "info",
		Short: "Show database information and statistics",
		Long:  "Display current database state, configuration, and statistics.",
		RunE:  showInfo,
	}
	
	// Reset command
	var resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Delete the database and start fresh",
		Long:  "Remove the database file (hybrid_search_poc.db) and all stored content, embeddings, and configuration.",
		RunE:  resetDatabase,
	}

	// Add flags
	initCmd.Flags().String("model", "nomic-embed-text", "Embedding model to use")
	initCmd.Flags().String("base-url", "", "Base URL for the embedding provider")
	
	searchCmd.Flags().String("mode", "hybrid", "Search mode (hybrid, keyword, semantic)")
	searchCmd.Flags().Float32("semantic-weight", 0.3, "Weight for semantic search in hybrid mode (0.0-1.0)")
	searchCmd.Flags().Int("limit", 10, "Maximum number of results to return")
	
	// Add commands to root
	rootCmd.AddCommand(initCmd, addCmd, bulkAddCmd, searchCmd, infoCmd, resetCmd)
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(errorStyle.Render("Error:"), err)
		os.Exit(1)
	}
}

func initDatabase(cmd *cobra.Command, args []string) error {
	provider := args[0]
	model, _ := cmd.Flags().GetString("model")
	baseURL, _ := cmd.Flags().GetString("base-url")
	
	fmt.Println(titleStyle.Render("🚀 Initializing Hybrid Search POC"))
	fmt.Println()
	
	start := time.Now()
	
	// Initialize database
	fmt.Print("Setting up SQLite database with go-ann vector search... ")
	db, err := NewVectorDB()
	if err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()
	
	fmt.Println(successStyle.Render("✓"))
	
	// Initialize embedding provider
	fmt.Printf("Configuring %s with model %s... ", provider, model)
	embedder, err := NewEmbeddingProvider(provider, model, baseURL)
	if err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return fmt.Errorf("failed to initialize embedding provider: %w", err)
	}
	
	fmt.Println(successStyle.Render("✓"))
	
	// Test embedding generation
	fmt.Print("Testing embedding generation... ")
	testEmbedding, err := embedder.GenerateEmbedding("test content")
	if err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return fmt.Errorf("failed to test embeddings: %w", err)
	}
	
	fmt.Println(successStyle.Render("✓"))

	// Store provider configuration in database
	fmt.Print("Storing provider configuration... ")
	err = db.StoreProviderConfig(provider, model, baseURL, len(testEmbedding))
	if err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return fmt.Errorf("failed to store provider configuration: %w", err)
	}
	fmt.Println(successStyle.Render("✓"))
	
	fmt.Println()
	fmt.Printf("🎉 %s\n", successStyle.Render("Initialization complete!"))
	fmt.Printf("   Database: SQLite with go-ann (MRPT) vector search\n")
	fmt.Printf("   Provider: %s (stored in database)\n", provider)
	fmt.Printf("   Model: %s\n", model)
	fmt.Printf("   Vector dimensions: %d\n", len(testEmbedding))
	fmt.Printf("   Time taken: %v\n", time.Since(start))
	
	return nil
}

// loadStoredEmbeddingProvider loads the embedding provider from database configuration
func loadStoredEmbeddingProvider() (EmbeddingProvider, error) {
	db, err := NewVectorDB()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Check if provider configuration exists
	hasConfig, err := db.HasProviderConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to check provider config: %w", err)
	}
	if !hasConfig {
		return nil, fmt.Errorf("no provider configuration found. Please run 'init' first to set up the embedding provider")
	}

	// Load stored provider configuration
	provider, model, baseURL, _, err := db.LoadProviderConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load provider config: %w", err)
	}

	// Create provider with stored configuration
	embedder, err := NewEmbeddingProvider(provider, model, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding provider: %w", err)
	}

	return embedder, nil
}

func addContent(cmd *cobra.Command, args []string) error {
	title, content := args[0], args[1]
	
	fmt.Printf("Adding content: %s\n", titleStyle.Render(title))
	
	db, err := NewVectorDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()
	
	embedder, err := loadStoredEmbeddingProvider()
	if err != nil {
		return fmt.Errorf("failed to load embedding provider: %w", err)
	}
	
	start := time.Now()
	
	fmt.Print("Generating embedding... ")
	embedding, err := embedder.GenerateEmbedding(content)
	if err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return fmt.Errorf("failed to generate embedding: %w", err)
	}
	fmt.Println(successStyle.Render("✓"))
	
	fmt.Print("Storing in database... ")
	err = db.AddContent(title, content, embedding)
	if err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return fmt.Errorf("failed to store content: %w", err)
	}
	fmt.Println(successStyle.Render("✓"))
	
	fmt.Printf("\n%s Content added successfully in %v\n", 
		successStyle.Render("✓"), time.Since(start))
	
	return nil
}

func bulkAddContent(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// No files specified - use sample data
		fmt.Printf("Adding %s\n", titleStyle.Render("predefined sample data"))
		return addSampleData()
	}

	// Files specified - process them
	fmt.Printf("Processing %d file(s)...\n", len(args))
	
	var allContent []ContentItem
	for _, pattern := range args {
		// Expand glob pattern
		files, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %s: %w", pattern, err)
		}
		
		if len(files) == 0 {
			return fmt.Errorf("no files match pattern: %s", pattern)
		}
		
		// Process each matched file
		for _, filename := range files {
			content, err := parseFile(filename)
			if err != nil {
				fmt.Printf("❌ Failed to parse %s: %v\n", filename, err)
				continue
			}
			allContent = append(allContent, content...)
			fmt.Printf("📄 Parsed %s (%d items)\n", filename, len(content))
		}
	}
	
	if len(allContent) == 0 {
		return fmt.Errorf("no content items found in the specified files")
	}
	
	return addContentItems(allContent)
}

// parseFile parses a single file and extracts content items
func parseFile(filename string) ([]ContentItem, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Determine file type by extension
	ext := strings.ToLower(filepath.Ext(filename))
	
	switch ext {
	case ".md", ".markdown":
		return parseMarkdown(filename, string(content))
	case ".txt":
		return parseText(filename, string(content))
	default:
		// Default to text parsing
		return parseText(filename, string(content))
	}
}

// parseMarkdown parses markdown content and extracts sections
func parseMarkdown(filename, content string) ([]ContentItem, error) {
	var items []ContentItem
	
	lines := strings.Split(content, "\n")
	var currentTitle string
	var currentContent []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check for headers (# ## ###)
		if strings.HasPrefix(line, "#") {
			// Save previous section if it exists
			if currentTitle != "" && len(currentContent) > 0 {
				items = append(items, ContentItem{
					Title:   currentTitle,
					Content: strings.Join(currentContent, "\n"),
				})
			}
			
			// Start new section
			currentTitle = strings.TrimSpace(strings.TrimLeft(line, "#"))
			if currentTitle == "" {
				currentTitle = fmt.Sprintf("Section from %s", filepath.Base(filename))
			}
			currentContent = []string{}
		} else if line != "" {
			// Add non-empty lines to current content
			currentContent = append(currentContent, line)
		}
	}
	
	// Save final section
	if currentTitle != "" && len(currentContent) > 0 {
		items = append(items, ContentItem{
			Title:   currentTitle,
			Content: strings.Join(currentContent, "\n"),
		})
	}
	
	// If no headers found, treat whole file as one item
	if len(items) == 0 && len(content) > 0 {
		title := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		items = append(items, ContentItem{
			Title:   title,
			Content: strings.TrimSpace(content),
		})
	}
	
	return items, nil
}

// parseText parses plain text files
func parseText(filename, content string) ([]ContentItem, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("file is empty")
	}
	
	// Use filename (without extension) as title
	title := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	
	return []ContentItem{
		{
			Title:   title,
			Content: strings.TrimSpace(content),
		},
	}, nil
}

// addSampleData adds the predefined sample data
func addSampleData() error {
	sampleData := GetSampleData()
	
	// Convert SampleContent to ContentItem
	var contentItems []ContentItem
	for _, item := range sampleData {
		contentItems = append(contentItems, ContentItem{
			Title:   item.Title,
			Content: item.Content,
		})
	}
	
	return addContentItems(contentItems)
}

// addContentItems adds multiple content items to the database
func addContentItems(items []ContentItem) error {
	db, err := NewVectorDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()
	
	embedder, err := loadStoredEmbeddingProvider()
	if err != nil {
		return fmt.Errorf("failed to load embedding provider: %w", err)
	}
	
	fmt.Printf("Processing %d items...\n", len(items))
	
	start := time.Now()
	successful := 0
	for i, item := range items {
		fmt.Printf("[%d/%d] %s... ", i+1, len(items), item.Title)
		
		embedding, err := embedder.GenerateEmbedding(item.Content)
		if err != nil {
			fmt.Println(errorStyle.Render("✗"))
			continue
		}
		
		err = db.AddContent(item.Title, item.Content, embedding)
		if err != nil {
			fmt.Println(errorStyle.Render("✗"))
			continue
		}
		
		fmt.Println(successStyle.Render("✓"))
		successful++
	}
	
	fmt.Printf("\n✓ Bulk import completed: %d/%d items in %v\n", successful, len(items), time.Since(start))
	
	return nil
}

func searchContent(cmd *cobra.Command, args []string) error {
	query := args[0]
	mode, _ := cmd.Flags().GetString("mode")
	semanticWeight, _ := cmd.Flags().GetFloat32("semantic-weight")
	limit, _ := cmd.Flags().GetInt("limit")
	
	fmt.Printf("%s %s\n", titleStyle.Render("🔍 Search Results"), 
		infoStyle.Render(fmt.Sprintf("Query: \"%s\"", query)))
	fmt.Printf("%s\n\n", infoStyle.Render(fmt.Sprintf("Mode: %s (semantic weight: %.2f)", mode, semanticWeight)))
	
	db, err := NewVectorDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()
	
	embedder, err := loadStoredEmbeddingProvider()
	if err != nil {
		return fmt.Errorf("failed to load embedding provider: %w", err)
	}
	
	start := time.Now()
	results, err := SearchHybrid(db, embedder, query, mode, semanticWeight, limit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	
	if len(results) == 0 {
		fmt.Println(infoStyle.Render("No results found."))
		return nil
	}
	
	for i, result := range results {
		fmt.Printf("%d. %s %s\n", 
			i+1,
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(fmt.Sprintf("%.3f", result.Score)),
			titleStyle.Render(result.Title))
		
		// Truncate content for display
		content := result.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		fmt.Printf("   %s\n", content)
		fmt.Printf("   %s\n\n", infoStyle.Render(fmt.Sprintf("Type: %s", result.SearchType)))
	}
	
	searchTime := time.Since(start)
	fmt.Printf("⚡ Search completed in %v\n", searchTime)
	fmt.Printf("📊 Found %d results\n", len(results))
	
	return nil
}

func showInfo(cmd *cobra.Command, args []string) error {
	fmt.Println(titleStyle.Render("📊 Database Information"))
	fmt.Println()
	
	db, err := NewVectorDB()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()
	
	stats, err := db.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get database stats: %w", err)
	}
	
	// Load provider configuration to get correct vector dimensions
	hasConfig, err := db.HasProviderConfig()
	var vectorDim int = int(stats.VectorDimensions) // fallback to database default
	var provider, model, baseURL string
	
	if err == nil && hasConfig {
		provider, model, baseURL, vectorDim, err = db.LoadProviderConfig()
		if err == nil {
			// Update stats with actual vector dimensions
			stats.VectorDimensions = int64(vectorDim)
		}
	}
	
	fmt.Printf("Content items: %d\n", stats.ContentCount)
	fmt.Printf("Vector dimensions: %d\n", stats.VectorDimensions)
	fmt.Printf("Database size: %s\n", stats.DatabaseSize)
	fmt.Printf("SQLite version: %s\n", stats.SQLiteVersion)
	fmt.Printf("Vector search: %s\n", stats.SqliteVecVersion)
	
	// Display provider configuration
	if hasConfig && err == nil {
		if baseURL != "" {
			fmt.Printf("Embedding provider: %s (model: %s, base URL: %s)\n", provider, model, baseURL)
		} else {
			fmt.Printf("Embedding provider: %s (model: %s)\n", provider, model)
		}
	} else {
		fmt.Printf("Embedding provider: Not configured (run 'init' to set up)\n")
	}
	
	return nil
}

func resetDatabase(cmd *cobra.Command, args []string) error {
	databaseFile := "hybrid_search_poc.db"
	
	fmt.Println(titleStyle.Render("🔄 Database Reset"))
	fmt.Println()
	
	// Check if database file exists
	if _, err := os.Stat(databaseFile); os.IsNotExist(err) {
		fmt.Printf("%s No database file found (%s)\n", infoStyle.Render("ℹ"), databaseFile)
		return nil
	}
	
	// Get database info before deletion
	db, err := NewVectorDB()
	if err == nil {
		stats, err := db.GetStats()
		if err == nil {
			fmt.Printf("Current database contains:\n")
			fmt.Printf("  • %d content items\n", stats.ContentCount)
			fmt.Printf("  • Vector search index with %s\n", stats.SqliteVecVersion)
		}
		
		// Check provider config
		hasConfig, err := db.HasProviderConfig()
		if err == nil && hasConfig {
			provider, model, baseURL, vectorDim, err := db.LoadProviderConfig()
			if err == nil {
				if baseURL != "" {
					fmt.Printf("  • Provider: %s (model: %s, base URL: %s, %d dims)\n", provider, model, baseURL, vectorDim)
				} else {
					fmt.Printf("  • Provider: %s (model: %s, %d dims)\n", provider, model, vectorDim)
				}
			}
		}
		db.Close()
		fmt.Println()
	}
	
	// Confirm deletion
	fmt.Printf("This will permanently delete all data in %s\n", databaseFile)
	fmt.Print("Are you sure you want to continue? (y/N): ")
	
	var response string
	fmt.Scanln(&response)
	
	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Printf("%s Reset cancelled\n", infoStyle.Render("ℹ"))
		return nil
	}
	
	// Delete the database file
	fmt.Printf("Removing %s... ", databaseFile)
	err = os.Remove(databaseFile)
	if err != nil {
		fmt.Println(errorStyle.Render("✗"))
		return fmt.Errorf("failed to remove database file: %w", err)
	}
	
	fmt.Println(successStyle.Render("✓"))
	fmt.Println()
	fmt.Printf("%s Database reset complete!\n", successStyle.Render("🎉"))
	fmt.Printf("Run '%s' to set up a new database with an embedding provider.\n", 
		titleStyle.Render("./hybrid-search-poc init <provider>"))
	
	return nil
}