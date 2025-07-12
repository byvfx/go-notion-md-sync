package concurrent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
)

// PageSyncJob represents a job to sync a single Notion page
type PageSyncJob struct {
	PageID    string
	PageTitle string
	Client    notion.Client
	Converter sync.Converter
	FilePath  string
}

// Execute implements the Job interface
func (psj *PageSyncJob) Execute(ctx context.Context) error {
	// Verify the page exists
	_, err := psj.Client.GetPage(ctx, psj.PageID)
	if err != nil {
		return fmt.Errorf("failed to fetch page %s: %w", psj.PageID, err)
	}

	// Get page blocks
	blocks, err := psj.Client.GetPageBlocks(ctx, psj.PageID)
	if err != nil {
		return fmt.Errorf("failed to fetch blocks for page %s: %w", psj.PageID, err)
	}

	// Convert blocks to markdown
	markdown, err := psj.Converter.BlocksToMarkdown(blocks)
	if err != nil {
		return fmt.Errorf("failed to convert page %s to markdown: %w", psj.PageID, err)
	}

	// Write to file using os package
	if err := writeFile(psj.FilePath, []byte(markdown)); err != nil {
		return fmt.Errorf("failed to write markdown file for page %s: %w", psj.PageID, err)
	}

	return nil
}

// ID implements the Job interface
func (psj *PageSyncJob) ID() string {
	return psj.PageID
}

// BlockFetchJob represents a job to fetch blocks for a page
type BlockFetchJob struct {
	PageID string
	Client notion.Client
	Result *[]notion.Block // Pointer to store result
}

// Execute implements the Job interface
func (bfj *BlockFetchJob) Execute(ctx context.Context) error {
	blocks, err := bfj.Client.GetPageBlocks(ctx, bfj.PageID)
	if err != nil {
		return fmt.Errorf("failed to fetch blocks: %w", err)
	}

	*bfj.Result = blocks
	return nil
}

// ID implements the Job interface
func (bfj *BlockFetchJob) ID() string {
	return fmt.Sprintf("blocks-%s", bfj.PageID)
}

// MarkdownWriteJob represents a job to write markdown to disk
type MarkdownWriteJob struct {
	FilePath string
	Content  string
	PageID   string
}

// Execute implements the Job interface
func (mwj *MarkdownWriteJob) Execute(ctx context.Context) error {
	// Ensure directory exists
	dir := filepath.Dir(mwj.FilePath)
	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := writeFile(mwj.FilePath, []byte(mwj.Content)); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ID implements the Job interface
func (mwj *MarkdownWriteJob) ID() string {
	return fmt.Sprintf("write-%s", mwj.PageID)
}

// DatabaseExportJob represents a job to export a Notion database
type DatabaseExportJob struct {
	DatabaseID string
	OutputPath string
	Client     notion.Client
	Syncer     sync.DatabaseSync
}

// Execute implements the Job interface
func (dej *DatabaseExportJob) Execute(ctx context.Context) error {
	return dej.Syncer.SyncNotionDatabaseToCSV(ctx, dej.DatabaseID, dej.OutputPath)
}

// ID implements the Job interface
func (dej *DatabaseExportJob) ID() string {
	return fmt.Sprintf("db-export-%s", dej.DatabaseID)
}

// SyncOrchestrator manages concurrent sync operations
type SyncOrchestrator struct {
	pool      *WorkerPool
	client    notion.Client
	converter sync.Converter
	config    *OrchestratorConfig
}

// OrchestratorConfig holds configuration for the sync orchestrator
type OrchestratorConfig struct {
	Workers    int
	QueueSize  int
	MaxRetries int
	BatchSize  int
	OutputDir  string
}

// DefaultOrchestratorConfig returns a default configuration
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		Workers:    5,
		QueueSize:  20,
		MaxRetries: 3,
		BatchSize:  10,
		OutputDir:  ".",
	}
}

// NewSyncOrchestrator creates a new sync orchestrator
func NewSyncOrchestrator(client notion.Client, converter sync.Converter, config *OrchestratorConfig) *SyncOrchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	pool := NewWorkerPool(config.Workers, config.QueueSize)
	pool.SetMaxRetries(config.MaxRetries)

	return &SyncOrchestrator{
		pool:      pool,
		client:    client,
		converter: converter,
		config:    config,
	}
}

// SyncPages synchronizes multiple pages concurrently
func (so *SyncOrchestrator) SyncPages(ctx context.Context, pageIDs []string) ([]Result, error) {
	if len(pageIDs) == 0 {
		return []Result{}, nil
	}

	// Start the pool
	so.pool.Start()
	defer so.pool.Shutdown()

	// Create jobs
	jobs := make([]Job, len(pageIDs))
	for i, pageID := range pageIDs {
		jobs[i] = &PageSyncJob{
			PageID:    pageID,
			Client:    so.client,
			Converter: so.converter,
			FilePath:  filepath.Join(so.config.OutputDir, fmt.Sprintf("%s.md", pageID)),
		}
	}

	// Process in batches
	var allResults []Result
	for i := 0; i < len(jobs); i += so.config.BatchSize {
		end := i + so.config.BatchSize
		if end > len(jobs) {
			end = len(jobs)
		}

		batch := jobs[i:end]

		// Submit batch
		for _, job := range batch {
			if err := so.pool.Submit(job); err != nil {
				return allResults, fmt.Errorf("failed to submit job: %w", err)
			}
		}

		// Collect batch results
		for j := 0; j < len(batch); j++ {
			select {
			case result := <-so.pool.Results():
				allResults = append(allResults, result)
			case <-ctx.Done():
				return allResults, fmt.Errorf("sync cancelled: %w", ctx.Err())
			}
		}
	}

	return allResults, nil
}

// Helper functions
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func writeFile(path string, data []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := ensureDir(dir); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
