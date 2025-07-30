package concurrent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/cache"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

// BenchmarkPageSyncSequential benchmarks sequential page synchronization (baseline)
func BenchmarkPageSyncSequential(b *testing.B) {
	client := &benchmarkNotionClient{}
	converter := &benchmarkConverter{}

	pageIDs := generatePageIDs(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, pageID := range pageIDs {
			// Simulate sequential sync
			page, _ := client.GetPage(context.Background(), pageID)
			blocks, _ := client.GetPageBlocks(context.Background(), pageID)
			_, _ = converter.BlocksToMarkdown(blocks)
			_ = page // Use page to avoid compiler optimization
		}
	}
}

// BenchmarkPageSyncConcurrent benchmarks concurrent page synchronization using worker pools
func BenchmarkPageSyncConcurrent(b *testing.B) {
	client := &benchmarkNotionClient{}
	converter := &benchmarkConverter{}

	pageIDs := generatePageIDs(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		orchestrator := NewSyncOrchestrator(client, converter, &OrchestratorConfig{
			Workers:    10,
			QueueSize:  50,
			MaxRetries: 1,
			BatchSize:  20,
			OutputDir:  "/tmp",
		})

		_, _ = orchestrator.SyncPages(context.Background(), pageIDs)
	}
}

// BenchmarkPageSyncWithCache benchmarks page sync with caching enabled
func BenchmarkPageSyncWithCache(b *testing.B) {
	client := &benchmarkNotionClient{}
	notionCache := cache.NewNotionCache(1000, 15*time.Minute)
	cachedClient := cache.NewCachedNotionClient(client, notionCache)
	converter := &benchmarkConverter{}

	pageIDs := generatePageIDs(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, pageID := range pageIDs {
			// First call hits API, subsequent calls hit cache
			page, _ := cachedClient.GetPage(context.Background(), pageID)
			blocks, _ := cachedClient.GetPageBlocks(context.Background(), pageID)
			_, _ = converter.BlocksToMarkdown(blocks)
			_ = page // Use page to avoid compiler optimization
		}
	}
}

// BenchmarkPageSyncWithCacheAndConcurrency benchmarks the full optimized stack
func BenchmarkPageSyncWithCacheAndConcurrency(b *testing.B) {
	client := &benchmarkNotionClient{}
	notionCache := cache.NewNotionCache(1000, 15*time.Minute)
	cachedClient := cache.NewCachedNotionClient(client, notionCache)
	converter := &benchmarkConverter{}

	pageIDs := generatePageIDs(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		orchestrator := NewSyncOrchestrator(cachedClient, converter, &OrchestratorConfig{
			Workers:    10,
			QueueSize:  50,
			MaxRetries: 1,
			BatchSize:  20,
			OutputDir:  "/tmp",
		})

		_, _ = orchestrator.SyncPages(context.Background(), pageIDs)
	}
}

// BenchmarkBatchProcessing benchmarks the advanced batch processor
func BenchmarkBatchProcessing(b *testing.B) {
	config := &BatchConfig{
		BatchSize:      50,
		MaxConcurrency: 10,
		RetryAttempts:  1,
		RetryDelay:     1 * time.Millisecond,
		Timeout:        5 * time.Second,
		EnableCaching:  false,
	}

	processor := NewAdvancedBatchProcessor(config)
	operations := generateBatchOperations(500)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = processor.ProcessBatch(context.Background(), operations)
	}
}

// BenchmarkBatchProcessingWithCache benchmarks batch processing with caching
func BenchmarkBatchProcessingWithCache(b *testing.B) {
	config := &BatchConfig{
		BatchSize:      50,
		MaxConcurrency: 10,
		RetryAttempts:  1,
		RetryDelay:     1 * time.Millisecond,
		Timeout:        5 * time.Second,
		EnableCaching:  true,
		CacheSize:      1000,
		CacheTTL:       15 * time.Minute,
	}

	processor := NewAdvancedBatchProcessor(config)
	operations := generateBatchOperations(500)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = processor.ProcessBatch(context.Background(), operations)
	}
}

// BenchmarkOptimizedBatchScheduling benchmarks the optimized batch scheduler
func BenchmarkOptimizedBatchScheduling(b *testing.B) {
	config := DefaultBatchConfig()
	batch := NewOptimizedBatch(config)
	operations := generateBatchOperations(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Schedule operations
		for _, op := range operations {
			batch.ScheduleOperation(op)
		}

		// Process batch
		_, _ = batch.ProcessScheduledBatches(context.Background())
	}
}

// BenchmarkWorkerPoolScaling benchmarks worker pool performance with different worker counts
func BenchmarkWorkerPoolScaling(b *testing.B) {
	workerCounts := []int{1, 2, 5, 10, 20, 50}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers-%d", workers), func(b *testing.B) {
			pool := NewWorkerPool(workers, workers*2)
			pool.Start()
			defer pool.Shutdown()

			jobs := generateJobs(100)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Submit jobs
				for _, job := range jobs {
					_ = pool.Submit(job)
				}

				// Collect results
				for j := 0; j < len(jobs); j++ {
					<-pool.Results()
				}
			}
		})
	}
}

// BenchmarkCachePerformance benchmarks cache operations
func BenchmarkCachePerformance(b *testing.B) {
	notionCache := cache.NewNotionCache(10000, 1*time.Hour)
	pages := generatePages(1000)

	// Populate cache
	for i, page := range pages {
		notionCache.SetPage(fmt.Sprintf("page-%d", i), page, 0)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 90% hits, 10% misses (realistic cache scenario)
		pageID := fmt.Sprintf("page-%d", i%900)
		notionCache.GetPage(context.Background(), pageID)
	}
}

// BenchmarkCacheMissPerformance benchmarks cache miss scenarios
func BenchmarkCacheMissPerformance(b *testing.B) {
	notionCache := cache.NewNotionCache(1000, 1*time.Hour)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// All cache misses
		pageID := fmt.Sprintf("page-%d", i)
		notionCache.GetPage(context.Background(), pageID)
	}
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	config := DefaultBatchConfig()
	processor := NewAdvancedBatchProcessor(config)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		operations := generateBatchOperations(100)
		_, _ = processor.ProcessBatch(context.Background(), operations)
	}
}

// BenchmarkThroughputComparison benchmarks throughput with different configurations
func BenchmarkThroughputComparison(b *testing.B) {
	configurations := []struct {
		name      string
		workers   int
		batchSize int
		caching   bool
	}{
		{"Sequential", 1, 1, false},
		{"LowConcurrency", 2, 10, false},
		{"MediumConcurrency", 5, 20, false},
		{"HighConcurrency", 10, 50, false},
		{"CachedMedium", 5, 20, true},
		{"CachedHigh", 10, 50, true},
	}

	for _, config := range configurations {
		b.Run(config.name, func(b *testing.B) {
			var client notion.Client = &benchmarkNotionClient{}
			converter := &benchmarkConverter{}

			if config.caching {
				notionCache := cache.NewNotionCache(1000, 15*time.Minute)
				client = cache.NewCachedNotionClient(client, notionCache)
			}

			orchestrator := NewSyncOrchestrator(client, converter, &OrchestratorConfig{
				Workers:    config.workers,
				QueueSize:  config.batchSize * 2,
				MaxRetries: 1,
				BatchSize:  config.batchSize,
				OutputDir:  "/tmp",
			})

			pageIDs := generatePageIDs(200)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = orchestrator.SyncPages(context.Background(), pageIDs)
			}
		})
	}
}

// Helper functions for benchmarks

func generatePageIDs(count int) []string {
	pageIDs := make([]string, count)
	for i := 0; i < count; i++ {
		pageIDs[i] = fmt.Sprintf("page-%d", i)
	}
	return pageIDs
}

func generateBatchOperations(count int) []BatchOperation {
	operations := make([]BatchOperation, count)
	types := []string{"page_sync", "block_sync", "database_sync"}

	for i := 0; i < count; i++ {
		operations[i] = BatchOperation{
			ID:   fmt.Sprintf("op-%d", i),
			Type: types[i%len(types)],
			Payload: map[string]interface{}{
				"index": i,
			},
		}
	}
	return operations
}

func generateJobs(count int) []Job {
	jobs := make([]Job, count)
	for i := 0; i < count; i++ {
		jobs[i] = &benchmarkJob{
			id: fmt.Sprintf("job-%d", i),
			work: func(ctx context.Context) error {
				// Simulate lightweight work
				time.Sleep(100 * time.Microsecond)
				return nil
			},
		}
	}
	return jobs
}

func generatePages(count int) []*notion.Page {
	pages := make([]*notion.Page, count)
	for i := 0; i < count; i++ {
		pages[i] = &notion.Page{
			ID: fmt.Sprintf("page-%d", i),
			Properties: map[string]interface{}{
				"title": fmt.Sprintf("Page %d", i),
			},
		}
	}
	return pages
}

// Benchmark-specific mock implementations

type benchmarkNotionClient struct {
	apiCallDelay time.Duration
	mu           sync.RWMutex
	callCount    int
}

func (c *benchmarkNotionClient) GetPage(ctx context.Context, pageID string) (*notion.Page, error) {
	c.mu.Lock()
	c.callCount++
	c.mu.Unlock()

	// Simulate API latency
	if c.apiCallDelay > 0 {
		time.Sleep(c.apiCallDelay)
	} else {
		time.Sleep(1 * time.Millisecond) // Default minimal delay
	}

	return &notion.Page{
		ID: pageID,
		Properties: map[string]interface{}{
			"title": fmt.Sprintf("Page %s", pageID),
		},
	}, nil
}

func (c *benchmarkNotionClient) GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, error) {
	c.mu.Lock()
	c.callCount++
	c.mu.Unlock()

	// Simulate API latency
	if c.apiCallDelay > 0 {
		time.Sleep(c.apiCallDelay)
	} else {
		time.Sleep(1 * time.Millisecond) // Default minimal delay
	}

	return []notion.Block{
		{ID: fmt.Sprintf("block-1-%s", pageID), Type: "paragraph"},
		{ID: fmt.Sprintf("block-2-%s", pageID), Type: "heading_1"},
		{ID: fmt.Sprintf("block-3-%s", pageID), Type: "paragraph"},
	}, nil
}

func (c *benchmarkNotionClient) GetDatabase(ctx context.Context, databaseID string) (*notion.Database, error) {
	c.mu.Lock()
	c.callCount++
	c.mu.Unlock()

	time.Sleep(2 * time.Millisecond) // Databases are slower

	return &notion.Database{
		ID: databaseID,
		Title: []notion.RichText{
			{PlainText: fmt.Sprintf("Database %s", databaseID)},
		},
	}, nil
}

func (c *benchmarkNotionClient) GetCallCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.callCount
}

// Implement remaining interface methods as no-ops for benchmarking
func (c *benchmarkNotionClient) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*notion.Page, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	return nil
}

func (c *benchmarkNotionClient) DeletePage(ctx context.Context, pageID string) error {
	return nil
}

func (c *benchmarkNotionClient) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*notion.Page, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) SearchPages(ctx context.Context, query string) ([]notion.Page, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) GetChildPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) GetAllDescendantPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) QueryDatabase(ctx context.Context, databaseID string, request *notion.DatabaseQueryRequest) (*notion.DatabaseQueryResponse, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) CreateDatabase(ctx context.Context, request *notion.CreateDatabaseRequest) (*notion.Database, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return nil, nil
}

func (c *benchmarkNotionClient) StreamDescendantPages(ctx context.Context, parentID string) *notion.PageStream {
	return notion.NewPageStream()
}

func (c *benchmarkNotionClient) StreamDatabaseRows(ctx context.Context, databaseID string) *notion.DatabaseRowStream {
	return notion.NewDatabaseRowStream()
}

type benchmarkConverter struct{}

func (c *benchmarkConverter) MarkdownToBlocks(content string) ([]map[string]interface{}, error) {
	// Simulate conversion work
	time.Sleep(100 * time.Microsecond)
	return []map[string]interface{}{
		{"type": "paragraph", "content": content},
	}, nil
}

func (c *benchmarkConverter) BlocksToMarkdown(blocks []notion.Block) (string, error) {
	// Simulate conversion work
	time.Sleep(50 * time.Microsecond)
	return fmt.Sprintf("# Content with %d blocks", len(blocks)), nil
}

type benchmarkJob struct {
	id   string
	work func(ctx context.Context) error
}

func (j *benchmarkJob) Execute(ctx context.Context) error {
	if j.work != nil {
		return j.work(ctx)
	}
	return nil
}

func (j *benchmarkJob) ID() string {
	return j.id
}

// Benchmark performance analysis functions

// BenchmarkAnalysis runs a comprehensive performance analysis
func BenchmarkAnalysis(b *testing.B) {
	if !testing.Short() {
		b.Run("FullStack", func(b *testing.B) {
			client := &benchmarkNotionClient{apiCallDelay: 5 * time.Millisecond}
			notionCache := cache.NewNotionCache(1000, 15*time.Minute)
			cachedClient := cache.NewCachedNotionClient(client, notionCache)
			converter := &benchmarkConverter{}

			orchestrator := NewSyncOrchestrator(cachedClient, converter, &OrchestratorConfig{
				Workers:    20,
				QueueSize:  100,
				MaxRetries: 1,
				BatchSize:  50,
				OutputDir:  "/tmp",
			})

			pageIDs := generatePageIDs(1000)

			b.ResetTimer()
			b.ReportAllocs()

			start := time.Now()
			for i := 0; i < b.N; i++ {
				_, _ = orchestrator.SyncPages(context.Background(), pageIDs)
			}
			elapsed := time.Since(start)

			b.ReportMetric(float64(len(pageIDs)*b.N)/elapsed.Seconds(), "pages/sec")
			b.ReportMetric(float64(client.GetCallCount()), "api_calls")
		})
	}
}
