//go:build cgo && (darwin || (linux && amd64))

package embedding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/backends"
	"github.com/knights-analytics/hugot/pipelines"
	"github.com/sirupsen/logrus"
)

const (
	// ModelRepo is the Hugging Face model repository
	ModelRepo = "sentence-transformers/all-MiniLM-L6-v2"
	// EmbeddingDimension is the dimension of the embedding vectors
	EmbeddingDimension = 384
	// DefaultModelDir is the default directory for model files
	DefaultModelDir = ".mcp-devtools/models"
)

// Config holds configuration for the embedding engine
type Config struct {
	ModelPath    string
	RuntimePath  string
	BatchSize    int
	MaxSeqLength int
}

// Engine provides embedding generation using a local ONNX model via hugot
type Engine struct {
	config         Config
	logger         *logrus.Logger
	session        *hugot.Session
	pipeline       *pipelines.FeatureExtractionPipeline
	modelLoaded    bool
	runtimeLoaded  bool
	modelPath      string
	runtimeVersion string
	mu             sync.RWMutex
}

// NewEngine creates a new embedding engine
func NewEngine(config Config, logger *logrus.Logger) (*Engine, error) {
	// Set defaults
	if config.ModelPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.ModelPath = filepath.Join(homeDir, DefaultModelDir)
	}

	if config.BatchSize <= 0 {
		config.BatchSize = 32
	}
	if config.MaxSeqLength <= 0 {
		config.MaxSeqLength = 512
	}

	engine := &Engine{
		config: config,
		logger: logger,
	}

	return engine, nil
}

// EnsureReady ensures the model and runtime are downloaded and loaded
func (e *Engine) EnsureReady(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.modelLoaded && e.runtimeLoaded {
		return nil
	}

	// Create hugot session - uses Go backend (no external ONNX runtime required)
	e.logger.Info("Initialising embedding engine...")
	session, err := hugot.NewGoSession()
	if err != nil {
		return fmt.Errorf("failed to create hugot session: %w", err)
	}
	e.session = session
	e.runtimeLoaded = true
	e.runtimeVersion = "gomlx"

	// Download model if not present
	if err := e.ensureModel(ctx); err != nil {
		return fmt.Errorf("failed to ensure model: %w", err)
	}

	// Create feature extraction pipeline
	if err := e.loadModel(); err != nil {
		return fmt.Errorf("failed to load model: %w", err)
	}

	return nil
}

// ensureModel downloads the embedding model if not present
func (e *Engine) ensureModel(_ context.Context) error {
	modelDir := filepath.Join(e.config.ModelPath, "sentence-transformers_all-MiniLM-L6-v2")
	modelFile := filepath.Join(modelDir, "onnx", "model.onnx")

	// Check if model exists
	if _, err := os.Stat(modelFile); err == nil {
		e.modelPath = modelDir
		e.logger.Debug("Embedding model already present")
		return nil
	}

	e.logger.Info("Downloading embedding model (this may take a moment)...")

	// Create model directory
	if err := os.MkdirAll(e.config.ModelPath, 0700); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	// Download model from Hugging Face using hugot
	// Specify the standard onnx model file (not optimised variants)
	downloadOptions := hugot.NewDownloadOptions()
	downloadOptions.OnnxFilePath = "onnx/model.onnx"

	downloadedPath, err := hugot.DownloadModel(ModelRepo, e.config.ModelPath, downloadOptions)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}

	e.modelPath = downloadedPath
	e.logger.WithField("path", downloadedPath).Info("Embedding model downloaded")

	return nil
}

// loadModel creates the feature extraction pipeline
func (e *Engine) loadModel() error {
	if e.session == nil {
		return fmt.Errorf("session not initialised")
	}

	e.logger.Debug("Loading feature extraction pipeline...")

	// Create feature extraction pipeline configuration
	config := backends.PipelineConfig[*pipelines.FeatureExtractionPipeline]{
		ModelPath: e.modelPath,
		Name:      "code-search-embeddings",
	}

	pipeline, err := hugot.NewPipeline[*pipelines.FeatureExtractionPipeline](e.session, config)
	if err != nil {
		return fmt.Errorf("failed to create feature extraction pipeline: %w", err)
	}

	e.pipeline = pipeline
	e.modelLoaded = true
	e.logger.Info("Embedding model loaded successfully")

	return nil
}

// Embed generates an embedding for the given text
func (e *Engine) Embed(ctx context.Context, text string) ([]float32, error) {
	if !e.modelLoaded {
		if err := e.EnsureReady(ctx); err != nil {
			return nil, err
		}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.pipeline == nil {
		return nil, fmt.Errorf("pipeline not initialised")
	}

	// Run feature extraction on single text
	result, err := e.pipeline.RunPipeline([]string{text})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Extract embedding from result
	if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("no embedding generated")
	}

	return result.Embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *Engine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	if !e.modelLoaded {
		if err := e.EnsureReady(ctx); err != nil {
			return nil, err
		}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.pipeline == nil {
		return nil, fmt.Errorf("pipeline not initialised")
	}

	// Process in batches for memory efficiency
	batchSize := e.config.BatchSize
	if batchSize <= 0 {
		batchSize = runtime.NumCPU()
	}

	allEmbeddings := make([][]float32, len(texts))

	for start := 0; start < len(texts); start += batchSize {
		end := min(start+batchSize, len(texts))
		batch := texts[start:end]

		result, err := e.pipeline.RunPipeline(batch)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embeddings for batch: %w", err)
		}

		for i, emb := range result.Embeddings {
			allEmbeddings[start+i] = emb
		}
	}

	return allEmbeddings, nil
}

// IsLoaded returns whether the model is loaded
func (e *Engine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.modelLoaded
}

// IsRuntimeLoaded returns whether the runtime is loaded
func (e *Engine) IsRuntimeLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.runtimeLoaded
}

// ModelPath returns the path to the loaded model
func (e *Engine) ModelPath() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.modelPath
}

// RuntimeVersion returns the runtime version
func (e *Engine) RuntimeVersion() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.runtimeVersion
}

// Dimension returns the embedding dimension
func (e *Engine) Dimension() int {
	return EmbeddingDimension
}

// Close releases resources
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.session != nil {
		if err := e.session.Destroy(); err != nil {
			e.logger.WithError(err).Warn("Failed to destroy hugot session")
		}
		e.session = nil
	}

	e.pipeline = nil
	e.modelLoaded = false
	e.runtimeLoaded = false

	return nil
}
