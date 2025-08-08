package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tmc/langchaingo/llms/openai"
)

// EmbeddingProvider interface for generating embeddings
type EmbeddingProvider interface {
	GenerateEmbedding(text string) ([]float32, error)
	GetProviderInfo() string
}

// OllamaEmbeddingProvider implements embeddings via Ollama
type OllamaEmbeddingProvider struct {
	client *openai.LLM
	model  string
	baseURL string
}

// OpenRouterEmbeddingProvider implements embeddings via OpenRouter
type OpenRouterEmbeddingProvider struct {
	client *openai.LLM
	model  string
}

// NewEmbeddingProvider creates an embedding provider based on the specified type
func NewEmbeddingProvider(provider, model, baseURL string) (EmbeddingProvider, error) {
	switch provider {
	case "ollama":
		return NewOllamaEmbeddingProvider(model, baseURL)
	case "openrouter":
		return NewOpenRouterEmbeddingProvider(model)
	case "mock":
		return NewMockEmbeddingProvider(1024), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// NewDefaultEmbeddingProvider creates a default embedding provider
// This would typically read from config/env vars
func NewDefaultEmbeddingProvider() (EmbeddingProvider, error) {
	// For the POC, default to mock provider for easy testing
	provider := os.Getenv("EMBEDDING_PROVIDER")
	if provider == "" {
		provider = "mock"
	}
	
	model := os.Getenv("EMBEDDING_MODEL")
	if model == "" {
		model = "nomic-embed-text"
	}
	
	baseURL := os.Getenv("EMBEDDING_BASE_URL")
	
	return NewEmbeddingProvider(provider, model, baseURL)
}

// NewOllamaEmbeddingProvider creates an Ollama-based embedding provider
func NewOllamaEmbeddingProvider(model, baseURL string) (*OllamaEmbeddingProvider, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	}
	
	// Configure LangChainGo for Ollama's OpenAI-compatible API
	client, err := openai.New(
		openai.WithBaseURL(baseURL),
		openai.WithToken("ollama"), // Ollama doesn't require a real token
		openai.WithEmbeddingModel(model),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client: %w", err)
	}
	
	return &OllamaEmbeddingProvider{
		client: client,
		model:  model,
		baseURL: baseURL,
	}, nil
}

// NewOpenRouterEmbeddingProvider creates an OpenRouter-based embedding provider
func NewOpenRouterEmbeddingProvider(model string) (*OpenRouterEmbeddingProvider, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is required")
	}
	
	if model == "" {
		model = "text-embedding-3-small" // Default to OpenAI's efficient embedding model
	}
	
	// Configure LangChainGo for OpenRouter
	client, err := openai.New(
		openai.WithBaseURL("https://openrouter.ai/api/v1"),
		openai.WithToken(apiKey),
		openai.WithEmbeddingModel(model),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenRouter client: %w", err)
	}
	
	return &OpenRouterEmbeddingProvider{
		client: client,
		model:  model,
	}, nil
}

// GenerateEmbedding generates an embedding for the given text using Ollama
func (p *OllamaEmbeddingProvider) GenerateEmbedding(text string) ([]float32, error) {
	ctx := context.Background()
	
	// Generate embedding using LangChainGo
	embeddings, err := p.client.CreateEmbedding(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding via Ollama: %w", err)
	}
	
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned from Ollama")
	}
	
	return embeddings[0], nil
}

// GetProviderInfo returns information about the Ollama provider
func (p *OllamaEmbeddingProvider) GetProviderInfo() string {
	return fmt.Sprintf("Ollama (%s) - Model: %s, URL: %s", p.client, p.model, p.baseURL)
}

// GenerateEmbedding generates an embedding for the given text using OpenRouter
func (p *OpenRouterEmbeddingProvider) GenerateEmbedding(text string) ([]float32, error) {
	ctx := context.Background()
	
	// Generate embedding using LangChainGo
	embeddings, err := p.client.CreateEmbedding(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding via OpenRouter: %w", err)
	}
	
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned from OpenRouter")
	}
	
	return embeddings[0], nil
}

// GetProviderInfo returns information about the OpenRouter provider
func (p *OpenRouterEmbeddingProvider) GetProviderInfo() string {
	return fmt.Sprintf("OpenRouter - Model: %s", p.model)
}

// MockEmbeddingProvider for testing when no real provider is available
type MockEmbeddingProvider struct {
	dimensions int
}

// NewMockEmbeddingProvider creates a mock provider for testing
func NewMockEmbeddingProvider(dimensions int) *MockEmbeddingProvider {
	if dimensions <= 0 {
		dimensions = 1024 // Default dimension
	}
	return &MockEmbeddingProvider{dimensions: dimensions}
}

// GenerateEmbedding generates a mock embedding based on text hash
func (p *MockEmbeddingProvider) GenerateEmbedding(text string) ([]float32, error) {
	// Create a deterministic but varied embedding based on text content
	embedding := make([]float32, p.dimensions)
	
	// Simple hash-based generation for consistent results
	hash := 0
	for _, char := range text {
		hash = int(char) + hash*31
	}
	
	for i := range embedding {
		// Generate values between -1 and 1 based on text and position
		seed := hash + i*7919 // 7919 is a large prime
		embedding[i] = float32((seed%2000 - 1000)) / 1000.0
	}
	
	return embedding, nil
}

// GetProviderInfo returns information about the mock provider
func (p *MockEmbeddingProvider) GetProviderInfo() string {
	return fmt.Sprintf("Mock Provider - Dimensions: %d (deterministic hash-based)", p.dimensions)
}