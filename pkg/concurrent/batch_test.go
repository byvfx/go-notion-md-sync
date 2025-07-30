package concurrent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

func TestDefaultBatchConfig(t *testing.T) {
	config := DefaultBatchConfig()

	if config.BatchSize != 20 {
		t.Errorf("Expected BatchSize 20, got %d", config.BatchSize)
	}
	if config.MaxConcurrency != 5 {
		t.Errorf("Expected MaxConcurrency 5, got %d", config.MaxConcurrency)
	}
	if config.RetryAttempts != 3 {
		t.Errorf("Expected RetryAttempts 3, got %d", config.RetryAttempts)
	}
	if config.EnableCaching != true {
		t.Errorf("Expected EnableCaching true, got %v", config.EnableCaching)
	}
}

func TestAdvancedBatchProcessor_ProcessBatch_Empty(t *testing.T) {
	config := DefaultBatchConfig()
	processor := NewAdvancedBatchProcessor(config)

	ctx := context.Background()
	result, err := processor.ProcessBatch(ctx, []BatchOperation{})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Success != 0 || result.Failed != 0 {
		t.Errorf("Expected empty result, got Success=%d, Failed=%d", result.Success, result.Failed)
	}
}

func TestAdvancedBatchProcessor_ProcessBatch_Success(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 5
	processor := NewAdvancedBatchProcessor(config)

	operations := []BatchOperation{
		{ID: "op1", Type: "page_sync", Payload: map[string]interface{}{"test": "data"}},
		{ID: "op2", Type: "block_sync", Payload: map[string]interface{}{"test": "data"}},
		{ID: "op3", Type: "database_sync", Payload: map[string]interface{}{"test": "data"}},
	}

	ctx := context.Background()
	result, err := processor.ProcessBatch(ctx, operations)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Success != 3 {
		t.Errorf("Expected 3 successful operations, got %d", result.Success)
	}

	if result.Failed != 0 {
		t.Errorf("Expected 0 failed operations, got %d", result.Failed)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %d errors", len(result.Errors))
	}

	// Check metadata
	if result.Metadata["batches_processed"] != 1 {
		t.Errorf("Expected 1 batch processed, got %v", result.Metadata["batches_processed"])
	}
}

func TestAdvancedBatchProcessor_ProcessBatch_LargeBatch(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 3
	processor := NewAdvancedBatchProcessor(config)

	// Create 10 operations, should be divided into 4 batches (3+3+3+1)
	operations := make([]BatchOperation, 10)
	for i := 0; i < 10; i++ {
		operations[i] = BatchOperation{
			ID:   fmt.Sprintf("op%d", i),
			Type: "page_sync",
			Payload: map[string]interface{}{
				"index": i,
			},
		}
	}

	ctx := context.Background()
	result, err := processor.ProcessBatch(ctx, operations)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Success != 10 {
		t.Errorf("Expected 10 successful operations, got %d", result.Success)
	}

	if result.Failed != 0 {
		t.Errorf("Expected 0 failed operations, got %d", result.Failed)
	}

	// Check that multiple batches were processed
	if result.Metadata["batches_processed"] != 4 {
		t.Errorf("Expected 4 batches processed, got %v", result.Metadata["batches_processed"])
	}
}

func TestAdvancedBatchProcessor_ProcessBatch_WithFailures(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 5
	processor := NewAdvancedBatchProcessor(config)

	// Create operations with unknown types (will fail)
	operations := []BatchOperation{
		{ID: "op1", Type: "page_sync", Payload: map[string]interface{}{"test": "data"}},
		{ID: "op2", Type: "unknown_type", Payload: map[string]interface{}{"test": "data"}},
		{ID: "op3", Type: "block_sync", Payload: map[string]interface{}{"test": "data"}},
	}

	ctx := context.Background()
	result, err := processor.ProcessBatch(ctx, operations)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Success != 2 {
		t.Errorf("Expected 2 successful operations, got %d", result.Success)
	}

	if result.Failed != 1 {
		t.Errorf("Expected 1 failed operation, got %d", result.Failed)
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d errors", len(result.Errors))
	}
}

func TestAdvancedBatchProcessor_ProcessBatch_ContextCancellation(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 5
	processor := NewAdvancedBatchProcessor(config)

	operations := []BatchOperation{
		{ID: "op1", Type: "page_sync", Payload: map[string]interface{}{"test": "data"}},
		{ID: "op2", Type: "block_sync", Payload: map[string]interface{}{"test": "data"}},
	}

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := processor.ProcessBatch(ctx, operations)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Operations should fail due to context cancellation
	if result.Failed == 0 {
		t.Error("Expected some operations to fail due to context cancellation")
	}
}

func TestAdvancedBatchProcessor_DivideBatches(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 3
	processor := NewAdvancedBatchProcessor(config)

	operations := make([]BatchOperation, 10)
	for i := 0; i < 10; i++ {
		operations[i] = BatchOperation{ID: fmt.Sprintf("op%d", i)}
	}

	batches := processor.divideBatches(operations)

	if len(batches) != 4 {
		t.Errorf("Expected 4 batches, got %d", len(batches))
	}

	// Check batch sizes
	expectedSizes := []int{3, 3, 3, 1}
	for i, batch := range batches {
		if len(batch) != expectedSizes[i] {
			t.Errorf("Batch %d: expected size %d, got %d", i, expectedSizes[i], len(batch))
		}
	}
}

func TestBulkSyncManager_BulkSyncPages(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 3

	// Create mock client and converter
	client := &mockNotionClient{}
	converter := &mockConverter{}

	manager := NewBulkSyncManager(client, converter, config)

	pageIDs := []string{"page1", "page2", "page3", "page4", "page5"}
	ctx := context.Background()

	result, err := manager.BulkSyncPages(ctx, pageIDs, "/tmp/output")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Success != 5 {
		t.Errorf("Expected 5 successful page syncs, got %d", result.Success)
	}

	if result.Failed != 0 {
		t.Errorf("Expected 0 failed page syncs, got %d", result.Failed)
	}
}

func TestBulkSyncManager_BulkSyncBlocks(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 2

	// Create mock client and converter
	client := &mockNotionClient{}
	converter := &mockConverter{}

	manager := NewBulkSyncManager(client, converter, config)

	pageIDs := []string{"page1", "page2", "page3"}
	ctx := context.Background()

	result, err := manager.BulkSyncBlocks(ctx, pageIDs)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Success != 3 {
		t.Errorf("Expected 3 successful block syncs, got %d", result.Success)
	}

	if result.Failed != 0 {
		t.Errorf("Expected 0 failed block syncs, got %d", result.Failed)
	}
}

func TestBulkSyncManager_BulkSyncDatabases(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 2

	// Create mock client and converter
	client := &mockNotionClient{}
	converter := &mockConverter{}

	manager := NewBulkSyncManager(client, converter, config)

	databaseIDs := []string{"db1", "db2", "db3"}
	ctx := context.Background()

	result, err := manager.BulkSyncDatabases(ctx, databaseIDs, "/tmp/output")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Success != 3 {
		t.Errorf("Expected 3 successful database syncs, got %d", result.Success)
	}

	if result.Failed != 0 {
		t.Errorf("Expected 0 failed database syncs, got %d", result.Failed)
	}
}

func TestOptimizedBatch_ScheduleOperation(t *testing.T) {
	config := DefaultBatchConfig()
	batch := NewOptimizedBatch(config)

	op1 := BatchOperation{ID: "op1", Type: "page_sync"}
	op2 := BatchOperation{ID: "op2", Type: "block_sync"}
	op3 := BatchOperation{ID: "op3", Type: "page_sync"}

	batch.ScheduleOperation(op1)
	batch.ScheduleOperation(op2)
	batch.ScheduleOperation(op3)

	stats := batch.GetQueueStats()

	if stats["page_sync"] != 2 {
		t.Errorf("Expected 2 page_sync operations, got %d", stats["page_sync"])
	}

	if stats["block_sync"] != 1 {
		t.Errorf("Expected 1 block_sync operation, got %d", stats["block_sync"])
	}
}

func TestOptimizedBatch_ProcessScheduledBatches(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 5
	batch := NewOptimizedBatch(config)

	// Schedule some operations
	operations := []BatchOperation{
		{ID: "op1", Type: "page_sync"},
		{ID: "op2", Type: "block_sync"},
		{ID: "op3", Type: "database_sync"},
		{ID: "op4", Type: "page_sync"},
	}

	for _, op := range operations {
		batch.ScheduleOperation(op)
	}

	// Check initial queue stats
	stats := batch.GetQueueStats()
	if len(stats) != 3 {
		t.Errorf("Expected 3 operation types, got %d", len(stats))
	}

	// Process scheduled batches
	ctx := context.Background()
	result, err := batch.ProcessScheduledBatches(ctx)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Success != 4 {
		t.Errorf("Expected 4 successful operations, got %d", result.Success)
	}

	// Check that queues are cleared after processing
	stats = batch.GetQueueStats()
	for opType, count := range stats {
		if count != 0 {
			t.Errorf("Expected queue for %s to be empty, got %d operations", opType, count)
		}
	}
}

func TestOptimizedBatch_GetQueueStats(t *testing.T) {
	config := DefaultBatchConfig()
	batch := NewOptimizedBatch(config)

	// Initially empty
	stats := batch.GetQueueStats()
	if len(stats) != 0 {
		t.Errorf("Expected empty stats, got %d entries", len(stats))
	}

	// Add operations
	batch.ScheduleOperation(BatchOperation{ID: "op1", Type: "page_sync"})
	batch.ScheduleOperation(BatchOperation{ID: "op2", Type: "page_sync"})
	batch.ScheduleOperation(BatchOperation{ID: "op3", Type: "block_sync"})

	stats = batch.GetQueueStats()
	if stats["page_sync"] != 2 {
		t.Errorf("Expected 2 page_sync operations, got %d", stats["page_sync"])
	}
	if stats["block_sync"] != 1 {
		t.Errorf("Expected 1 block_sync operation, got %d", stats["block_sync"])
	}
}

func BenchmarkAdvancedBatchProcessor_ProcessBatch(b *testing.B) {
	config := DefaultBatchConfig()
	config.BatchSize = 10
	processor := NewAdvancedBatchProcessor(config)

	operations := make([]BatchOperation, 50)
	for i := 0; i < 50; i++ {
		operations[i] = BatchOperation{
			ID:   fmt.Sprintf("op%d", i),
			Type: "page_sync",
			Payload: map[string]interface{}{
				"index": i,
			},
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.ProcessBatch(ctx, operations)
	}
}

func BenchmarkOptimizedBatch_ScheduleAndProcess(b *testing.B) {
	config := DefaultBatchConfig()
	config.BatchSize = 10
	batch := NewOptimizedBatch(config)

	operations := make([]BatchOperation, 50)
	for i := 0; i < 50; i++ {
		operations[i] = BatchOperation{
			ID:   fmt.Sprintf("op%d", i),
			Type: "page_sync",
			Payload: map[string]interface{}{
				"index": i,
			},
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Schedule operations
		for _, op := range operations {
			batch.ScheduleOperation(op)
		}

		// Process batch
		_, _ = batch.ProcessScheduledBatches(ctx)
	}
}

func TestAdvancedBatchProcessor_RetryLogic(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 5
	config.RetryAttempts = 2
	config.RetryDelay = 1 * time.Millisecond
	processor := NewAdvancedBatchProcessor(config)

	// Create operation with unknown type (will fail)
	operations := []BatchOperation{
		{ID: "op1", Type: "unknown_type", Payload: map[string]interface{}{"test": "data"}},
	}

	ctx := context.Background()
	start := time.Now()
	result, err := processor.ProcessBatch(ctx, operations)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Failed != 1 {
		t.Errorf("Expected 1 failed operation, got %d", result.Failed)
	}

	// Should have taken at least 2 retry delays (2 * 1ms = 2ms)
	if elapsed < 2*time.Millisecond {
		t.Errorf("Expected at least 2ms for retries, got %v", elapsed)
	}
}

func TestAdvancedBatchProcessor_Timeout(t *testing.T) {
	config := DefaultBatchConfig()
	config.BatchSize = 5
	config.Timeout = 50 * time.Millisecond
	processor := NewAdvancedBatchProcessor(config)

	// Mock operation that completes quickly
	operations := []BatchOperation{
		{ID: "op1", Type: "page_sync", Payload: map[string]interface{}{"test": "data"}},
	}

	ctx := context.Background()
	result, err := processor.ProcessBatch(ctx, operations)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// All operations should complete within timeout (they're very fast)
	if result.Success != 1 {
		t.Errorf("Expected 1 successful operation, got %d", result.Success)
	}
}

// Mock implementations for testing
type mockNotionClient struct{}

func (m *mockNotionClient) GetPage(ctx context.Context, pageID string) (*notion.Page, error) {
	return &notion.Page{ID: pageID}, nil
}

func (m *mockNotionClient) GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, error) {
	return []notion.Block{{ID: "block-1"}}, nil
}

func (m *mockNotionClient) GetDatabase(ctx context.Context, databaseID string) (*notion.Database, error) {
	return &notion.Database{ID: databaseID}, nil
}

func (m *mockNotionClient) StreamDescendantPages(ctx context.Context, parentID string) *notion.PageStream {
	return notion.NewPageStream()
}

func (m *mockNotionClient) StreamDatabaseRows(ctx context.Context, databaseID string) *notion.DatabaseRowStream {
	return notion.NewDatabaseRowStream()
}

func (m *mockNotionClient) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*notion.Page, error) {
	return nil, nil
}

func (m *mockNotionClient) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	return nil
}

func (m *mockNotionClient) DeletePage(ctx context.Context, pageID string) error {
	return nil
}

func (m *mockNotionClient) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*notion.Page, error) {
	return nil, nil
}

func (m *mockNotionClient) SearchPages(ctx context.Context, query string) ([]notion.Page, error) {
	return nil, nil
}

func (m *mockNotionClient) GetChildPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	return nil, nil
}

func (m *mockNotionClient) GetAllDescendantPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	return nil, nil
}

func (m *mockNotionClient) QueryDatabase(ctx context.Context, databaseID string, request *notion.DatabaseQueryRequest) (*notion.DatabaseQueryResponse, error) {
	return nil, nil
}

func (m *mockNotionClient) CreateDatabase(ctx context.Context, request *notion.CreateDatabaseRequest) (*notion.Database, error) {
	return nil, nil
}

func (m *mockNotionClient) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return nil, nil
}

func (m *mockNotionClient) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return nil, nil
}

type mockConverter struct{}

func (m *mockConverter) MarkdownToBlocks(content string) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (m *mockConverter) BlocksToMarkdown(blocks []notion.Block) (string, error) {
	return "# Test Content", nil
}
