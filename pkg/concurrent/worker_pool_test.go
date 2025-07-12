package concurrent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testJob is a simple job implementation for testing
type testJob struct {
	id        string
	work      func(ctx context.Context) error
	execCount int32
}

func (tj *testJob) Execute(ctx context.Context) error {
	atomic.AddInt32(&tj.execCount, 1)
	if tj.work != nil {
		return tj.work(ctx)
	}
	return nil
}

func (tj *testJob) ID() string {
	return tj.id
}

func (tj *testJob) ExecutionCount() int32 {
	return atomic.LoadInt32(&tj.execCount)
}

func TestNewWorkerPool(t *testing.T) {
	tests := []struct {
		name        string
		workers     int
		queueSize   int
		wantWorkers int
		wantQueue   int
	}{
		{
			name:        "valid parameters",
			workers:     5,
			queueSize:   10,
			wantWorkers: 5,
			wantQueue:   10,
		},
		{
			name:        "zero workers",
			workers:     0,
			queueSize:   10,
			wantWorkers: 1,
			wantQueue:   10,
		},
		{
			name:        "negative workers",
			workers:     -5,
			queueSize:   10,
			wantWorkers: 1,
			wantQueue:   10,
		},
		{
			name:        "zero queue size",
			workers:     5,
			queueSize:   0,
			wantWorkers: 5,
			wantQueue:   10, // workers * 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewWorkerPool(tt.workers, tt.queueSize)
			if pool.workers != tt.wantWorkers {
				t.Errorf("NewWorkerPool() workers = %v, want %v", pool.workers, tt.wantWorkers)
			}
			if cap(pool.jobQueue) != tt.wantQueue {
				t.Errorf("NewWorkerPool() queue size = %v, want %v", cap(pool.jobQueue), tt.wantQueue)
			}
		})
	}
}

func TestWorkerPool_Submit(t *testing.T) {
	pool := NewWorkerPool(2, 5)
	pool.Start()
	defer pool.Shutdown()

	job := &testJob{id: "test-1"}

	err := pool.Submit(job)
	if err != nil {
		t.Errorf("Submit() error = %v, want nil", err)
	}
}

func TestWorkerPool_ProcessJobs(t *testing.T) {
	pool := NewWorkerPool(3, 10)
	pool.Start()
	defer pool.Shutdown()

	// Create 10 jobs
	jobs := make([]*testJob, 10)
	for i := 0; i < 10; i++ {
		jobs[i] = &testJob{
			id: fmt.Sprintf("job-%d", i),
			work: func(ctx context.Context) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		}
	}

	// Submit all jobs
	for _, job := range jobs {
		if err := pool.Submit(job); err != nil {
			t.Fatalf("Failed to submit job: %v", err)
		}
	}

	// Collect results
	results := make(map[string]Result)
	for i := 0; i < len(jobs); i++ {
		result := <-pool.Results()
		results[result.JobID] = result
	}

	// Verify all jobs completed
	if len(results) != len(jobs) {
		t.Errorf("Got %d results, want %d", len(results), len(jobs))
	}

	// Verify no errors
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("Job %s failed: %v", result.JobID, result.Error)
		}
	}
}

func TestWorkerPool_Retry(t *testing.T) {
	pool := NewWorkerPool(1, 5)
	pool.SetMaxRetries(2)
	pool.Start()
	defer pool.Shutdown()

	attempts := int32(0)
	job := &testJob{
		id: "retry-job",
		work: func(ctx context.Context) error {
			count := atomic.AddInt32(&attempts, 1)
			if count < 3 {
				return errors.New("temporary failure")
			}
			return nil
		},
	}

	if err := pool.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	result := <-pool.Results()

	// Should succeed after retries
	if result.Error != nil {
		t.Errorf("Job failed after retries: %v", result.Error)
	}

	// Should have attempted 3 times (initial + 2 retries)
	if attempts != 3 {
		t.Errorf("Job attempted %d times, want 3", attempts)
	}
}

func TestWorkerPool_MaxRetryExceeded(t *testing.T) {
	pool := NewWorkerPool(1, 5)
	pool.SetMaxRetries(1)
	pool.Start()
	defer pool.Shutdown()

	job := &testJob{
		id: "failing-job",
		work: func(ctx context.Context) error {
			return errors.New("permanent failure")
		},
	}

	if err := pool.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	result := <-pool.Results()

	// Should fail after max retries
	if result.Error == nil {
		t.Error("Expected job to fail, but it succeeded")
	}

	// Should have attempted 2 times (initial + 1 retry)
	if job.ExecutionCount() != 2 {
		t.Errorf("Job executed %d times, want 2", job.ExecutionCount())
	}
}

func TestWorkerPool_Shutdown(t *testing.T) {
	pool := NewWorkerPool(2, 5)
	pool.Start()

	// Submit a job
	job := &testJob{
		id: "test-job",
		work: func(ctx context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	}

	if err := pool.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Shutdown should wait for job to complete
	done := make(chan bool)
	go func() {
		pool.Shutdown()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Error("Shutdown took too long")
	}
}

func TestWorkerPool_ShutdownNow(t *testing.T) {
	pool := NewWorkerPool(1, 5)
	pool.Start()

	// Submit a long-running job
	job := &testJob{
		id: "long-job",
		work: func(ctx context.Context) error {
			select {
			case <-time.After(5 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	if err := pool.Submit(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// ShutdownNow should cancel running jobs
	done := make(chan bool)
	go func() {
		pool.ShutdownNow()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("ShutdownNow took too long")
	}
}

func TestBatchProcessor_ProcessBatch(t *testing.T) {
	bp := NewBatchProcessor(3)

	// Create batch of jobs
	jobs := make([]Job, 10)
	for i := 0; i < 10; i++ {
		jobs[i] = &testJob{
			id: fmt.Sprintf("batch-job-%d", i),
			work: func(ctx context.Context) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			},
		}
	}

	ctx := context.Background()
	results, err := bp.ProcessBatch(ctx, jobs)

	if err != nil {
		t.Errorf("ProcessBatch() error = %v, want nil", err)
	}

	if len(results) != len(jobs) {
		t.Errorf("ProcessBatch() returned %d results, want %d", len(results), len(jobs))
	}

	// Verify all jobs succeeded
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("Job %s failed: %v", result.JobID, result.Error)
		}
	}
}

func TestBatchProcessor_EmptyBatch(t *testing.T) {
	bp := NewBatchProcessor(3)

	ctx := context.Background()
	results, err := bp.ProcessBatch(ctx, []Job{})

	if err != nil {
		t.Errorf("ProcessBatch() error = %v, want nil", err)
	}

	if len(results) != 0 {
		t.Errorf("ProcessBatch() returned %d results, want 0", len(results))
	}
}

func TestBatchProcessor_ContextCancellation(t *testing.T) {
	bp := NewBatchProcessor(1)

	// Create jobs that take time but respect context cancellation
	jobs := make([]Job, 5)
	for i := 0; i < 5; i++ {
		jobs[i] = &testJob{
			id: fmt.Sprintf("slow-job-%d", i),
			work: func(ctx context.Context) error {
				select {
				case <-time.After(100 * time.Millisecond):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	results, err := bp.ProcessBatch(ctx, jobs)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected context cancellation error")
	}

	// Should have processed some but not all jobs
	if len(results) >= len(jobs) {
		t.Errorf("Processed too many jobs: %d", len(results))
	}

	// Should have finished quickly after context cancellation
	if elapsed > 250*time.Millisecond {
		t.Errorf("Took too long to cancel: %v", elapsed)
	}
}

func TestWorkerPool_Concurrency(t *testing.T) {
	workers := 5
	pool := NewWorkerPool(workers, 20)
	pool.Start()
	defer pool.Shutdown()

	// Track concurrent executions
	var concurrent int32
	maxConcurrent := int32(0)
	mu := sync.Mutex{}

	// Create jobs that run concurrently
	jobs := make([]*testJob, 20)
	for i := 0; i < 20; i++ {
		jobs[i] = &testJob{
			id: fmt.Sprintf("concurrent-job-%d", i),
			work: func(ctx context.Context) error {
				// Increment concurrent counter
				current := atomic.AddInt32(&concurrent, 1)

				// Track max concurrent
				mu.Lock()
				if current > maxConcurrent {
					maxConcurrent = current
				}
				mu.Unlock()

				// Simulate work
				time.Sleep(50 * time.Millisecond)

				// Decrement counter
				atomic.AddInt32(&concurrent, -1)
				return nil
			},
		}
	}

	// Submit all jobs
	for _, job := range jobs {
		if err := pool.Submit(job); err != nil {
			t.Fatalf("Failed to submit job: %v", err)
		}
	}

	// Collect results
	for i := 0; i < len(jobs); i++ {
		<-pool.Results()
	}

	// Verify concurrency was achieved
	if maxConcurrent < 2 {
		t.Errorf("Max concurrent executions = %d, expected at least 2", maxConcurrent)
	}

	// Verify it didn't exceed worker count
	if maxConcurrent > int32(workers) {
		t.Errorf("Max concurrent executions = %d, exceeded worker count %d", maxConcurrent, workers)
	}
}
