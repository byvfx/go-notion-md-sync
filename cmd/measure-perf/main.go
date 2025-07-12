package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/spf13/cobra"
	"strings"
)

var (
	pageID   string
	requests int
	verbose  bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "measure-perf",
		Short: "Measure Notion API performance",
		Long:  "Simple tool to measure real Notion API response times without creating test data",
		Run:   measurePerf,
	}

	rootCmd.Flags().StringVarP(&pageID, "page", "p", "", "Page ID to test with (required)")
	rootCmd.Flags().IntVarP(&requests, "requests", "n", 10, "Number of requests to make")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	if err := rootCmd.MarkFlagRequired("page"); err != nil {
		log.Fatal(err)
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func measurePerf(cmd *cobra.Command, args []string) {
	// Get Notion token from environment
	token := os.Getenv("NOTION_MD_SYNC_NOTION_TOKEN")
	if token == "" {
		log.Fatal("Notion token not configured. Set NOTION_MD_SYNC_NOTION_TOKEN environment variable")
	}

	// Create Notion client
	client := notion.NewClient(token)
	ctx := context.Background()

	fmt.Println("Notion API Performance Measurement")
	fmt.Println("==================================")
	fmt.Printf("Testing with page: %s\n", pageID)
	fmt.Printf("Requests: %d\n\n", requests)

	// Test GetPage performance
	fmt.Println("Testing GetPage API:")
	getPageTimes := make([]time.Duration, 0, requests)

	for i := 0; i < requests; i++ {
		start := time.Now()
		_, err := client.GetPage(ctx, pageID)
		duration := time.Since(start)

		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
			continue
		}

		getPageTimes = append(getPageTimes, duration)
		if verbose {
			fmt.Printf("  Request %d: %v\n", i+1, duration)
		}

		// Small delay to avoid hitting rate limits
		time.Sleep(350 * time.Millisecond)
	}

	// Test GetPageBlocks performance
	fmt.Println("\nTesting GetPageBlocks API:")
	getBlocksTimes := make([]time.Duration, 0, requests)

	for i := 0; i < requests; i++ {
		start := time.Now()
		blocks, err := client.GetPageBlocks(ctx, pageID)
		duration := time.Since(start)

		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
			continue
		}

		getBlocksTimes = append(getBlocksTimes, duration)
		if verbose {
			fmt.Printf("  Request %d: %v (%d blocks)\n", i+1, duration, len(blocks))
		}

		// Small delay to avoid hitting rate limits
		time.Sleep(350 * time.Millisecond)
	}

	// Calculate statistics
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("RESULTS")
	fmt.Println(strings.Repeat("=", 50))

	if len(getPageTimes) > 0 {
		fmt.Println("\nGetPage API:")
		printStats(getPageTimes)
	}

	if len(getBlocksTimes) > 0 {
		fmt.Println("\nGetPageBlocks API:")
		printStats(getBlocksTimes)
	}

	// Combined statistics
	if len(getPageTimes) > 0 && len(getBlocksTimes) > 0 {
		fmt.Println("\nCombined (GetPage + GetPageBlocks):")
		combinedTimes := make([]time.Duration, 0)
		minLen := len(getPageTimes)
		if len(getBlocksTimes) < minLen {
			minLen = len(getBlocksTimes)
		}
		for i := 0; i < minLen; i++ {
			combinedTimes = append(combinedTimes, getPageTimes[i]+getBlocksTimes[i])
		}
		printStats(combinedTimes)
	}

	// Performance projections
	if len(getPageTimes) > 0 && len(getBlocksTimes) > 0 {
		avgCombined := average(getPageTimes) + average(getBlocksTimes)
		fmt.Println("\nPerformance Projections (based on average):")
		fmt.Printf("  10 pages:   ~%.1f seconds (sequential)\n", avgCombined.Seconds()*10)
		fmt.Printf("  100 pages:  ~%.1f seconds (sequential)\n", avgCombined.Seconds()*100)
		fmt.Printf("  1000 pages: ~%.1f minutes (sequential)\n", avgCombined.Seconds()*1000/60)

		fmt.Println("\nWith 10 concurrent workers (theoretical):")
		fmt.Printf("  100 pages:  ~%.1f seconds\n", avgCombined.Seconds()*100/10)
		fmt.Printf("  1000 pages: ~%.1f seconds\n", avgCombined.Seconds()*1000/10)

		fmt.Println("\nNote: Actual concurrent performance will be limited by:")
		fmt.Println("  - Notion API rate limits (3 req/sec sustained)")
		fmt.Println("  - Network conditions")
		fmt.Println("  - Server load")
	}
}

func printStats(times []time.Duration) {
	if len(times) == 0 {
		fmt.Println("  No successful requests")
		return
	}

	avg := average(times)
	min := minimum(times)
	max := maximum(times)

	fmt.Printf("  Successful requests: %d\n", len(times))
	fmt.Printf("  Average: %v\n", avg)
	fmt.Printf("  Min:     %v\n", min)
	fmt.Printf("  Max:     %v\n", max)

	// Calculate percentiles
	if len(times) >= 10 {
		p50 := percentile(times, 50)
		p90 := percentile(times, 90)
		p95 := percentile(times, 95)

		fmt.Printf("  P50:     %v\n", p50)
		fmt.Printf("  P90:     %v\n", p90)
		fmt.Printf("  P95:     %v\n", p95)
	}
}

func average(times []time.Duration) time.Duration {
	if len(times) == 0 {
		return 0
	}

	var sum time.Duration
	for _, t := range times {
		sum += t
	}
	return sum / time.Duration(len(times))
}

func minimum(times []time.Duration) time.Duration {
	if len(times) == 0 {
		return 0
	}

	min := times[0]
	for _, t := range times[1:] {
		if t < min {
			min = t
		}
	}
	return min
}

func maximum(times []time.Duration) time.Duration {
	if len(times) == 0 {
		return 0
	}

	max := times[0]
	for _, t := range times[1:] {
		if t > max {
			max = t
		}
	}
	return max
}

func percentile(times []time.Duration, p int) time.Duration {
	if len(times) == 0 {
		return 0
	}

	// Simple percentile calculation (not exact but good enough)
	index := len(times) * p / 100
	if index >= len(times) {
		index = len(times) - 1
	}

	// Sort times
	sorted := make([]time.Duration, len(times))
	copy(sorted, times)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted[index]
}
