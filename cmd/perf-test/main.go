package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/cache"
	"github.com/byvfx/go-notion-md-sync/pkg/concurrent"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/spf13/cobra"
	"strings"
)

var (
	parentPageID string
	pageCount    int
	workers      int
	enableCache  bool
	cacheSize    int
	batchSize    int
	outputDir    string
	verbose      bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "perf-test",
		Short: "Performance testing tool for notion-md-sync",
		Long:  "Measures real-world performance of Notion API operations with various optimization settings",
		Run:   runPerfTest,
	}

	rootCmd.Flags().StringVarP(&parentPageID, "parent", "p", "", "Parent page ID to test under (required)")
	rootCmd.Flags().IntVarP(&pageCount, "pages", "n", 10, "Number of test pages to create")
	rootCmd.Flags().IntVarP(&workers, "workers", "w", 1, "Number of concurrent workers")
	rootCmd.Flags().BoolVarP(&enableCache, "cache", "c", false, "Enable caching")
	rootCmd.Flags().IntVar(&cacheSize, "cache-size", 1000, "Cache size (entries)")
	rootCmd.Flags().IntVar(&batchSize, "batch-size", 20, "Batch size for operations")
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "./perf-test-output", "Output directory for test files")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	if err := rootCmd.MarkFlagRequired("parent"); err != nil {
		log.Fatal(err)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runPerfTest(cmd *cobra.Command, args []string) {
	// Get Notion token from environment
	token := os.Getenv("NOTION_MD_SYNC_NOTION_TOKEN")
	if token == "" {
		log.Fatal("Notion token not configured. Set NOTION_MD_SYNC_NOTION_TOKEN environment variable")
	}

	// Create Notion client
	client := notion.NewClient(token)
	ctx := context.Background()

	// Verify parent page exists
	parentPage, err := client.GetPage(ctx, parentPageID)
	if err != nil {
		log.Fatalf("Failed to access parent page %s: %v", parentPageID, err)
	}

	fmt.Printf("Performance Test Configuration:\n")
	fmt.Printf("- Parent Page: %s\n", getPageTitle(parentPage))
	fmt.Printf("- Test Pages: %d\n", pageCount)
	fmt.Printf("- Workers: %d\n", workers)
	fmt.Printf("- Cache: %v (size: %d)\n", enableCache, cacheSize)
	fmt.Printf("- Batch Size: %d\n", batchSize)
	fmt.Printf("\n")

	// Create test pages
	fmt.Println("Creating test pages...")
	pageIDs, createTime := createTestPages(ctx, client, parentPageID, pageCount)
	fmt.Printf("Created %d pages in %v (%.2f pages/sec)\n\n",
		len(pageIDs), createTime, float64(len(pageIDs))/createTime.Seconds())

	// Wait a bit for Notion to index
	time.Sleep(2 * time.Second)

	// Test 1: Sequential Pull
	fmt.Println("Test 1: Sequential Pull")
	seq1Time := testSequentialPull(ctx, client, pageIDs)

	// Test 2: Concurrent Pull (no cache)
	fmt.Printf("\nTest 2: Concurrent Pull (%d workers, no cache)\n", workers)
	conc1Time := testConcurrentPull(ctx, client, pageIDs, workers, false)

	// Test 3: Sequential Pull with Cache
	fmt.Println("\nTest 3: Sequential Pull with Cache")
	seqCacheTime := testSequentialPullWithCache(ctx, client, pageIDs)

	// Test 4: Concurrent Pull with Cache
	fmt.Printf("\nTest 4: Concurrent Pull (%d workers, with cache)\n", workers)
	concCacheTime := testConcurrentPull(ctx, client, pageIDs, workers, true)

	// Test 5: Batch Processing
	fmt.Printf("\nTest 5: Batch Processing (batch size: %d)\n", batchSize)
	batchTime := testBatchProcessing(ctx, client, pageIDs)

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("PERFORMANCE SUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Sequential baseline:           %v\n", seq1Time)
	fmt.Printf("Concurrent (%d workers):       %v (%.2fx improvement)\n",
		workers, conc1Time, seq1Time.Seconds()/conc1Time.Seconds())
	fmt.Printf("Sequential with cache:         %v (%.2fx improvement)\n",
		seqCacheTime, seq1Time.Seconds()/seqCacheTime.Seconds())
	fmt.Printf("Concurrent with cache:         %v (%.2fx improvement)\n",
		concCacheTime, seq1Time.Seconds()/concCacheTime.Seconds())
	fmt.Printf("Batch processing:              %v (%.2fx improvement)\n",
		batchTime, seq1Time.Seconds()/batchTime.Seconds())

	// Cleanup
	fmt.Printf("\nCleaning up test pages...")
	cleanupTime := cleanupTestPages(ctx, client, pageIDs)
	fmt.Printf(" done in %v\n", cleanupTime)
}

func createTestPages(ctx context.Context, client notion.Client, parentID string, count int) ([]string, time.Duration) {
	start := time.Now()
	pageIDs := make([]string, 0, count)

	for i := 0; i < count; i++ {
		properties := map[string]interface{}{
			"title": []interface{}{
				map[string]interface{}{
					"text": map[string]interface{}{
						"content": fmt.Sprintf("Perf Test Page %d - %s", i+1, time.Now().Format("15:04:05")),
					},
				},
			},
		}

		// Add some content blocks
		blocks := []map[string]interface{}{
			{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": map[string]interface{}{
								"content": fmt.Sprintf("This is test content for page %d. Created for performance testing.", i+1),
							},
						},
					},
				},
			},
			{
				"object": "block",
				"type":   "heading_2",
				"heading_2": map[string]interface{}{
					"rich_text": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": map[string]interface{}{
								"content": "Test Section",
							},
						},
					},
				},
			},
			{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": map[string]interface{}{
								"content": "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
							},
						},
					},
				},
			},
		}

		page, err := client.RecreatePageWithBlocks(ctx, parentID, properties, blocks)
		if err != nil {
			log.Printf("Failed to create test page %d: %v", i+1, err)
			continue
		}

		pageIDs = append(pageIDs, page.ID)
		if verbose {
			fmt.Printf("Created page %d/%d\n", i+1, count)
		}
	}

	return pageIDs, time.Since(start)
}

func testSequentialPull(ctx context.Context, client notion.Client, pageIDs []string) time.Duration {
	start := time.Now()
	converter := sync.NewConverter()

	for i, pageID := range pageIDs {
		// Get page
		_, err := client.GetPage(ctx, pageID)
		if err != nil {
			log.Printf("Failed to get page: %v", err)
			continue
		}

		// Get blocks
		blocks, err := client.GetPageBlocks(ctx, pageID)
		if err != nil {
			log.Printf("Failed to get blocks: %v", err)
			continue
		}

		// Convert to markdown
		_, err = converter.BlocksToMarkdown(blocks)
		if err != nil {
			log.Printf("Failed to convert blocks: %v", err)
			continue
		}

		if verbose && (i+1)%10 == 0 {
			fmt.Printf("  Processed %d/%d pages\n", i+1, len(pageIDs))
		}
	}

	duration := time.Since(start)
	fmt.Printf("  Completed in %v (%.2f pages/sec)\n",
		duration, float64(len(pageIDs))/duration.Seconds())
	return duration
}

func testSequentialPullWithCache(ctx context.Context, client notion.Client, pageIDs []string) time.Duration {
	// Create cached client
	notionCache := cache.NewNotionCache(cacheSize, 15*time.Minute)
	cachedClient := cache.NewCachedNotionClient(client, notionCache)

	// First run to warm cache
	fmt.Println("  Warming cache...")
	testSequentialPull(ctx, cachedClient, pageIDs)

	// Get cache stats
	stats := notionCache.Stats()
	fmt.Printf("  Cache stats after warm-up: %d hits, %d misses (%.1f%% hit rate)\n",
		stats.Hits, stats.Misses, float64(stats.Hits)/float64(stats.Hits+stats.Misses)*100)

	// Second run with warm cache
	fmt.Println("  Testing with warm cache...")
	start := time.Now()
	converter := sync.NewConverter()

	for i, pageID := range pageIDs {
		// Get page (should hit cache)
		_, err := cachedClient.GetPage(ctx, pageID)
		if err != nil {
			log.Printf("Failed to get page: %v", err)
			continue
		}

		// Get blocks (should hit cache)
		blocks, err := cachedClient.GetPageBlocks(ctx, pageID)
		if err != nil {
			log.Printf("Failed to get blocks: %v", err)
			continue
		}

		// Convert to markdown
		_, err = converter.BlocksToMarkdown(blocks)
		if err != nil {
			log.Printf("Failed to convert blocks: %v", err)
			continue
		}

		if verbose && (i+1)%10 == 0 {
			fmt.Printf("  Processed %d/%d pages\n", i+1, len(pageIDs))
		}
	}

	duration := time.Since(start)

	// Final cache stats
	finalStats := notionCache.Stats()
	fmt.Printf("  Completed in %v (%.2f pages/sec)\n",
		duration, float64(len(pageIDs))/duration.Seconds())
	fmt.Printf("  Final cache stats: %d hits, %d misses (%.1f%% hit rate)\n",
		finalStats.Hits, finalStats.Misses,
		float64(finalStats.Hits)/float64(finalStats.Hits+finalStats.Misses)*100)

	return duration
}

func testConcurrentPull(ctx context.Context, client notion.Client, pageIDs []string, workerCount int, useCache bool) time.Duration {
	converter := sync.NewConverter()

	// Optionally wrap with cache
	actualClient := client
	if useCache {
		notionCache := cache.NewNotionCache(cacheSize, 15*time.Minute)
		actualClient = cache.NewCachedNotionClient(client, notionCache)
	}

	config := &concurrent.OrchestratorConfig{
		Workers:    workerCount,
		QueueSize:  workerCount * 2,
		MaxRetries: 1,
		BatchSize:  batchSize,
		OutputDir:  outputDir,
	}

	orchestrator := concurrent.NewSyncOrchestrator(actualClient, converter, config)

	start := time.Now()
	results, err := orchestrator.SyncPages(ctx, pageIDs)
	if err != nil {
		log.Printf("Orchestrator error: %v", err)
	}

	duration := time.Since(start)

	// Count successes and failures
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	fmt.Printf("  Completed in %v (%.2f pages/sec)\n",
		duration, float64(len(pageIDs))/duration.Seconds())
	fmt.Printf("  Success: %d/%d pages\n", successCount, len(pageIDs))

	return duration
}

func testBatchProcessing(ctx context.Context, client notion.Client, pageIDs []string) time.Duration {
	converter := sync.NewConverter()

	config := &concurrent.BatchConfig{
		BatchSize:      batchSize,
		MaxConcurrency: workers,
		RetryAttempts:  1,
		RetryDelay:     100 * time.Millisecond,
		Timeout:        30 * time.Second,
		EnableCaching:  enableCache,
		CacheSize:      cacheSize,
		CacheTTL:       15 * time.Minute,
	}

	manager := concurrent.NewBulkSyncManager(client, converter, config)

	start := time.Now()
	result, err := manager.BulkSyncPages(ctx, pageIDs, outputDir)
	if err != nil {
		log.Printf("Batch processing error: %v", err)
	}

	duration := time.Since(start)

	fmt.Printf("  Completed in %v (%.2f pages/sec)\n",
		duration, float64(len(pageIDs))/duration.Seconds())
	fmt.Printf("  Success: %d, Failed: %d\n", result.Success, result.Failed)

	return duration
}

func cleanupTestPages(ctx context.Context, client notion.Client, pageIDs []string) time.Duration {
	start := time.Now()

	for _, pageID := range pageIDs {
		err := client.DeletePage(ctx, pageID)
		if err != nil && verbose {
			log.Printf("Failed to delete page %s: %v", pageID, err)
		}
	}

	return time.Since(start)
}

func getPageTitle(page *notion.Page) string {
	if page == nil || page.Properties == nil {
		return "Unknown"
	}

	if titleProp, ok := page.Properties["title"].(map[string]interface{}); ok {
		if titleArray, ok := titleProp["title"].([]interface{}); ok && len(titleArray) > 0 {
			if firstTitle, ok := titleArray[0].(map[string]interface{}); ok {
				if text, ok := firstTitle["text"].(map[string]interface{}); ok {
					if content, ok := text["content"].(string); ok {
						return content
					}
				}
			}
		}
	}

	return "Untitled"
}
