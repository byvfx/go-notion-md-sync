package concurrent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/cache"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	syncpkg "github.com/byvfx/go-notion-md-sync/pkg/sync"
)

// BatchConfig holds configuration for batch processing operations
type BatchConfig struct {
	BatchSize         int           // Number of items to process in each batch
	MaxConcurrency    int           // Maximum number of concurrent batches
	RetryAttempts     int           // Number of retry attempts for failed operations
	RetryDelay        time.Duration // Delay between retry attempts
	Timeout           time.Duration // Timeout for individual operations
	EnableCaching     bool          // Whether to enable caching
	CacheSize         int           // Size of the cache
	CacheTTL          time.Duration // Time-to-live for cache entries
	EnableCompression bool          // Whether to enable compression for large payloads
}

// DefaultBatchConfig returns a default batch configuration
func DefaultBatchConfig() *BatchConfig {
	return &BatchConfig{
		BatchSize:         20,
		MaxConcurrency:    5,
		RetryAttempts:     3,
		RetryDelay:        100 * time.Millisecond,
		Timeout:           30 * time.Second,
		EnableCaching:     true,
		CacheSize:         1000,
		CacheTTL:          15 * time.Minute,
		EnableCompression: false,
	}
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Success  int                    // Number of successful operations
	Failed   int                    // Number of failed operations
	Errors   []error                // List of errors encountered
	Duration time.Duration          // Total duration of the batch
	Metadata map[string]interface{} // Additional metadata
}

// BatchOperation represents a single operation in a batch
type BatchOperation struct {
	ID       string
	Type     string
	Payload  interface{}
	Metadata map[string]interface{}
}

// AdvancedBatchProcessor handles batch processing of multiple operations
type AdvancedBatchProcessor struct {
	config    *BatchConfig
	cache     cache.NotionCache
	processor *WorkerPool
}

// NewAdvancedBatchProcessor creates a new advanced batch processor with the given configuration
func NewAdvancedBatchProcessor(config *BatchConfig) *AdvancedBatchProcessor {
	if config == nil {
		config = DefaultBatchConfig()
	}

	bp := &AdvancedBatchProcessor{
		config:    config,
		processor: NewWorkerPool(config.MaxConcurrency, config.BatchSize*2),
	}

	// Initialize cache if enabled
	if config.EnableCaching {
		bp.cache = cache.NewNotionCache(config.CacheSize, config.CacheTTL)
	}

	return bp
}

// ProcessBatch processes a batch of operations
func (bp *AdvancedBatchProcessor) ProcessBatch(ctx context.Context, operations []BatchOperation) (*BatchResult, error) {
	if len(operations) == 0 {
		return &BatchResult{}, nil
	}

	startTime := time.Now()
	result := &BatchResult{
		Metadata: make(map[string]interface{}),
	}

	// Divide operations into batches
	batches := bp.divideBatches(operations)

	// Process each batch
	var wg sync.WaitGroup
	resultChan := make(chan BatchResult, len(batches))
	errorChan := make(chan error, len(batches))

	for i, batch := range batches {
		wg.Add(1)
		go func(batchID int, ops []BatchOperation) {
			defer wg.Done()

			batchResult, err := bp.processSingleBatch(ctx, ops)
			if err != nil {
				errorChan <- fmt.Errorf("batch %d failed: %w", batchID, err)
				return
			}

			resultChan <- *batchResult
		}(i, batch)
	}

	// Wait for all batches to complete
	wg.Wait()
	close(resultChan)
	close(errorChan)

	// Collect results
	for batchResult := range resultChan {
		result.Success += batchResult.Success
		result.Failed += batchResult.Failed
		result.Errors = append(result.Errors, batchResult.Errors...)
	}

	// Collect errors
	for err := range errorChan {
		result.Errors = append(result.Errors, err)
	}

	result.Duration = time.Since(startTime)
	result.Metadata["batches_processed"] = len(batches)
	result.Metadata["operations_per_batch"] = bp.config.BatchSize

	return result, nil
}

// divideBatches divides operations into smaller batches
func (bp *AdvancedBatchProcessor) divideBatches(operations []BatchOperation) [][]BatchOperation {
	var batches [][]BatchOperation

	for i := 0; i < len(operations); i += bp.config.BatchSize {
		end := i + bp.config.BatchSize
		if end > len(operations) {
			end = len(operations)
		}
		batches = append(batches, operations[i:end])
	}

	return batches
}

// processSingleBatch processes a single batch of operations
func (bp *AdvancedBatchProcessor) processSingleBatch(ctx context.Context, operations []BatchOperation) (*BatchResult, error) {
	result := &BatchResult{
		Metadata: make(map[string]interface{}),
	}

	// Create timeout context for this batch
	batchCtx, cancel := context.WithTimeout(ctx, bp.config.Timeout)
	defer cancel()

	// Process operations with retry logic
	for _, op := range operations {
		if err := bp.processOperationWithRetry(batchCtx, op); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("operation %s failed: %w", op.ID, err))
		} else {
			result.Success++
		}
	}

	return result, nil
}

// processOperationWithRetry processes a single operation with retry logic
func (bp *AdvancedBatchProcessor) processOperationWithRetry(ctx context.Context, op BatchOperation) error {
	var lastErr error

	for attempt := 0; attempt <= bp.config.RetryAttempts; attempt++ {
		// Check if context is cancelled
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}

		// Process the operation
		if err := bp.processOperation(ctx, op); err != nil {
			lastErr = err

			// If this is the last attempt, return the error
			if attempt == bp.config.RetryAttempts {
				break
			}

			// Wait before retrying
			select {
			case <-time.After(bp.config.RetryDelay):
				continue
			case <-ctx.Done():
				return fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
			}
		} else {
			// Operation succeeded
			return nil
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", bp.config.RetryAttempts+1, lastErr)
}

// processOperation processes a single operation
func (bp *AdvancedBatchProcessor) processOperation(ctx context.Context, op BatchOperation) error {
	// This is a placeholder for the actual operation processing
	// In a real implementation, this would handle different operation types
	switch op.Type {
	case "page_sync":
		return bp.processPageSync(ctx, op)
	case "block_sync":
		return bp.processBlockSync(ctx, op)
	case "database_sync":
		return bp.processDatabaseSync(ctx, op)
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// processPageSync processes a page synchronization operation
func (bp *AdvancedBatchProcessor) processPageSync(ctx context.Context, op BatchOperation) error {
	// Placeholder implementation
	// In a real implementation, this would sync a page
	time.Sleep(10 * time.Millisecond) // Simulate work
	return nil
}

// processBlockSync processes a block synchronization operation
func (bp *AdvancedBatchProcessor) processBlockSync(ctx context.Context, op BatchOperation) error {
	// Placeholder implementation
	// In a real implementation, this would sync blocks
	time.Sleep(5 * time.Millisecond) // Simulate work
	return nil
}

// processDatabaseSync processes a database synchronization operation
func (bp *AdvancedBatchProcessor) processDatabaseSync(ctx context.Context, op BatchOperation) error {
	// Placeholder implementation
	// In a real implementation, this would sync a database
	time.Sleep(15 * time.Millisecond) // Simulate work
	return nil
}

// BulkSyncManager manages bulk synchronization operations
type BulkSyncManager struct {
	client    notion.Client
	converter syncpkg.Converter
	processor *AdvancedBatchProcessor
	cache     cache.NotionCache
}

// NewBulkSyncManager creates a new bulk sync manager
func NewBulkSyncManager(client notion.Client, converter syncpkg.Converter, config *BatchConfig) *BulkSyncManager {
	processor := NewAdvancedBatchProcessor(config)

	var notionCache cache.NotionCache
	if config.EnableCaching {
		notionCache = cache.NewNotionCache(config.CacheSize, config.CacheTTL)
	}

	return &BulkSyncManager{
		client:    client,
		converter: converter,
		processor: processor,
		cache:     notionCache,
	}
}

// BulkSyncPages synchronizes multiple pages concurrently
func (bsm *BulkSyncManager) BulkSyncPages(ctx context.Context, pageIDs []string, outputDir string) (*BatchResult, error) {
	operations := make([]BatchOperation, len(pageIDs))
	for i, pageID := range pageIDs {
		operations[i] = BatchOperation{
			ID:   pageID,
			Type: "page_sync",
			Payload: map[string]interface{}{
				"page_id":    pageID,
				"output_dir": outputDir,
			},
		}
	}

	return bsm.processor.ProcessBatch(ctx, operations)
}

// BulkSyncBlocks synchronizes blocks for multiple pages
func (bsm *BulkSyncManager) BulkSyncBlocks(ctx context.Context, pageIDs []string) (*BatchResult, error) {
	operations := make([]BatchOperation, len(pageIDs))
	for i, pageID := range pageIDs {
		operations[i] = BatchOperation{
			ID:   fmt.Sprintf("blocks-%s", pageID),
			Type: "block_sync",
			Payload: map[string]interface{}{
				"page_id": pageID,
			},
		}
	}

	return bsm.processor.ProcessBatch(ctx, operations)
}

// BulkSyncDatabases synchronizes multiple databases
func (bsm *BulkSyncManager) BulkSyncDatabases(ctx context.Context, databaseIDs []string, outputDir string) (*BatchResult, error) {
	operations := make([]BatchOperation, len(databaseIDs))
	for i, dbID := range databaseIDs {
		operations[i] = BatchOperation{
			ID:   dbID,
			Type: "database_sync",
			Payload: map[string]interface{}{
				"database_id": dbID,
				"output_dir":  outputDir,
			},
		}
	}

	return bsm.processor.ProcessBatch(ctx, operations)
}

// OptimizedBatch provides optimized batch processing with intelligent scheduling
type OptimizedBatch struct {
	config    *BatchConfig
	scheduler *BatchScheduler
}

// BatchScheduler manages the scheduling of batch operations
type BatchScheduler struct {
	mu         sync.RWMutex
	queues     map[string][]BatchOperation // Priority queues for different operation types
	priorities map[string]int              // Priority levels for operation types
}

// NewOptimizedBatch creates a new optimized batch processor
func NewOptimizedBatch(config *BatchConfig) *OptimizedBatch {
	scheduler := &BatchScheduler{
		queues:     make(map[string][]BatchOperation),
		priorities: make(map[string]int),
	}

	// Set default priorities
	scheduler.priorities["page_sync"] = 1
	scheduler.priorities["block_sync"] = 2
	scheduler.priorities["database_sync"] = 3

	return &OptimizedBatch{
		config:    config,
		scheduler: scheduler,
	}
}

// ScheduleOperation schedules an operation for batch processing
func (ob *OptimizedBatch) ScheduleOperation(op BatchOperation) {
	ob.scheduler.mu.Lock()
	defer ob.scheduler.mu.Unlock()

	if _, exists := ob.scheduler.queues[op.Type]; !exists {
		ob.scheduler.queues[op.Type] = make([]BatchOperation, 0)
	}

	ob.scheduler.queues[op.Type] = append(ob.scheduler.queues[op.Type], op)
}

// ProcessScheduledBatches processes all scheduled operations
func (ob *OptimizedBatch) ProcessScheduledBatches(ctx context.Context) (*BatchResult, error) {
	ob.scheduler.mu.Lock()
	defer ob.scheduler.mu.Unlock()

	// Sort operations by priority
	var allOperations []BatchOperation
	for opType, priority := range ob.scheduler.priorities {
		if operations, exists := ob.scheduler.queues[opType]; exists {
			// Add priority information to metadata
			for _, op := range operations {
				if op.Metadata == nil {
					op.Metadata = make(map[string]interface{})
				}
				op.Metadata["priority"] = priority
				allOperations = append(allOperations, op)
			}
		}
	}

	// Clear queues
	ob.scheduler.queues = make(map[string][]BatchOperation)

	// Process the batch
	processor := NewAdvancedBatchProcessor(ob.config)
	return processor.ProcessBatch(ctx, allOperations)
}

// GetQueueStats returns statistics about the current queues
func (ob *OptimizedBatch) GetQueueStats() map[string]int {
	ob.scheduler.mu.RLock()
	defer ob.scheduler.mu.RUnlock()

	stats := make(map[string]int)
	for opType, queue := range ob.scheduler.queues {
		stats[opType] = len(queue)
	}

	return stats
}
