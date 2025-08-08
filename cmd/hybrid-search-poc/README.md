# Hybrid Search POC

A proof-of-concept implementation of hybrid search combining traditional keyword search with semantic vector search using pure Go dependencies.

## Architecture

This POC demonstrates how to build a hybrid search system that:

- **Keyword Search**: Uses SQLite's LIKE queries for traditional text matching
- **Semantic Search**: Uses vector embeddings with go-ann's MRPT algorithm for similarity search
- **Hybrid Search**: Combines both approaches using Reciprocal Rank Fusion (RRF) for optimal results

### Technology Stack

- **SQLite**: Pure Go SQLite driver (`modernc.org/sqlite`) for structured data storage
- **Vector Search**: `github.com/rikonor/go-ann` for approximate nearest neighbor search using Multiple Random Projection Trees (MRPT)
- **Embeddings**: LangChainGo with support for both Ollama and OpenRouter providers
- **CLI**: Cobra framework with lipgloss styling for beautiful terminal output

## Quick Start

### 1. Initialize the Database

Choose your embedding provider:

```bash
# Using mock embeddings (for testing)
./hybrid-search-poc init mock

# Using Ollama (requires Ollama server running)
./hybrid-search-poc init ollama

# Using OpenRouter (requires API key)
./hybrid-search-poc init openrouter
```

### 2. Add Content

Add individual content:
```bash
./hybrid-search-poc add "Machine Learning Basics" "Introduction to supervised and unsupervised learning algorithms..."
```

Add sample data in bulk:
```bash
./hybrid-search-poc bulk-add
```

### 3. Search

**Keyword search** (traditional text matching):
```bash
./hybrid-search-poc search "golang programming" --mode keyword
```

**Semantic search** (vector similarity):
```bash
./hybrid-search-poc search "golang programming" --mode semantic
```

**Hybrid search** (combines both approaches):
```bash
./hybrid-search-poc search "golang programming" --mode hybrid
```

### 4. View Statistics

```bash
./hybrid-search-poc info
```

## Command Reference

### Initialize Database
```bash
./hybrid-search-poc init <provider>
```
- `provider`: One of `mock`, `ollama`, or `openrouter`
- Creates database schema and sets up embedding provider

### Add Content
```bash
./hybrid-search-poc add <title> <content>
```
- Adds a single content item with generated embeddings
- Content is indexed for both keyword and semantic search

### Bulk Add Content
```bash
# Add predefined sample data (12 items)
./hybrid-search-poc bulk-add

# Import from Markdown files using a glob pattern
./hybrid-search-poc bulk-add ../../data/*.md

# Import from a specific text file
./hybrid-search-poc bulk-add documents/report.txt
```
- **No arguments**: Adds 12 predefined sample content items for demonstration.
- **With file arguments**: Processes files from specified paths or glob patterns.
  - **Chunking Strategy**:
    - **Markdown (`.md`, `.markdown`)**: The file is chunked into sections based on headers (`#`, `##`, `###`). Each section becomes a separate item for embedding. If no headers are found, the entire file is treated as a single item.
    - **Text (`.txt`) & Other Types**: The entire file contents are treated as a single item. The title is derived from the filename.

### Search
```bash
./hybrid-search-poc search <query> [flags]
```

**Flags:**
- `--mode, -m`: Search mode (`hybrid`, `keyword`, `semantic`) [default: hybrid]
- `--limit, -l`: Maximum number of results [default: 10]
- `--semantic-weight, -w`: Weight for semantic results in hybrid mode [default: 0.7]

**Examples:**
```bash
# Default hybrid search
./hybrid-search-poc search "machine learning"

# Keyword-only search
./hybrid-search-poc search "machine learning" --mode keyword

# Semantic search with custom limit
./hybrid-search-poc search "machine learning" --mode semantic --limit 5

# Hybrid search with custom weighting (favor keyword results)
./hybrid-search-poc search "machine learning" --semantic-weight 0.3
```

### Database Info
```bash
./hybrid-search-poc info
```
- Shows content count, vector dimensions, SQLite version
- Displays go-ann index statistics
- Shows currently configured embedding provider

### Reset Database
```bash
./hybrid-search-poc reset
```
- Deletes the database file (`hybrid_search_poc.db`) and all data
- Requires confirmation before deletion
- Useful for starting fresh or switching providers

## Configuration

### Ollama Setup
1. Install and run Ollama server
2. Ensure the embedding model is available (default: `nomic-embed-text`)
3. Ollama server should be accessible at `http://localhost:11434`

### OpenRouter Setup
1. Set the `OPENROUTER_API_KEY` environment variable
2. Default model: `text-embedding-3-small`

### Mock Provider
- Uses random embeddings for testing
- No external dependencies required
- Consistent results for the same input

## Implementation Details

### Hybrid Search Algorithm

The hybrid search uses **Reciprocal Rank Fusion (RRF)** to combine keyword and semantic results:

1. Execute both keyword and semantic searches independently
2. Rank results from each search method
3. Calculate RRF score: `score = keywordWeight * (1/(k + keywordRank)) + semanticWeight * (1/(k + semanticRank))`
4. Merge and re-rank combined results

### Vector Storage

- Embeddings stored as JSON in SQLite `content_embeddings` table
- Vectors loaded into memory and indexed using go-ann's MRPT algorithm
- MRPT uses 5 trees with depth 10 for balanced performance/accuracy

### Database Schema

```sql
CREATE TABLE content (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE content_embeddings (
    content_id INTEGER PRIMARY KEY,
    embedding TEXT NOT NULL,
    FOREIGN KEY (content_id) REFERENCES content(id)
);
```

## Performance Considerations

- **Single Binary Distribution**: All dependencies are pure Go, enabling single binary deployment
- **Memory Usage**: All vectors loaded into memory for fast ANN search
- **Index Rebuilding**: ANN index rebuilt after each content addition (optimize for production use)
- **Embedding Generation**: Network calls for Ollama/OpenRouter add latency

## Sample Data Topics

The bulk-add command includes content covering:

- Programming languages (Go, Python, Rust)
- Machine learning and AI concepts
- Database technologies
- Search and information retrieval
- Software architecture patterns
- Development tools and practices

## Next Steps for Production

1. **Batch Index Updates**: Rebuild ANN index periodically rather than after each addition
2. **Embedding Caching**: Cache embeddings to avoid regeneration
3. **Persistent ANN Index**: Serialize/deserialize ANN index to disk
4. **Advanced Scoring**: Implement TF-IDF or BM25 for better keyword scoring
5. **Configuration Management**: External config file for embedding models and parameters
6. **Monitoring**: Add metrics for search performance and accuracy

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `modernc.org/sqlite` - Pure Go SQLite driver
- `github.com/rikonor/go-ann` - Approximate nearest neighbor search
- `github.com/tmc/langchaingo` - LLM and embedding integrations
- `github.com/charmbracelet/lipgloss` - Terminal styling

## Building

```bash
go build -o hybrid-search-poc
```

The resulting binary is self-contained with no external dependencies (except for embedding providers).