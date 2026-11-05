package concurrent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// withTimeout fails the test if fn does not return within d.
func withTimeout(t *testing.T, d time.Duration, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not finish within %v (deadlock)", name, d)
	}
}

// TestBatchProcessor_ManyJobsNoDeadlock is a regression test: all jobs
// were submitted before any result was read, which deadlocked once the
// queue and results buffers filled up.
func TestBatchProcessor_ManyJobsNoDeadlock(t *testing.T) {
	tests := []struct {
		workers int
		jobs    int
	}{
		{2, 20},
		{1, 100},
		{4, 500},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("workers=%d/jobs=%d", tt.workers, tt.jobs), func(t *testing.T) {
			jobs := make([]Job, tt.jobs)
			for i := range jobs {
				jobs[i] = &testJob{id: fmt.Sprintf("job-%d", i)}
			}

			var results []Result
			var err error
			withTimeout(t, 5*time.Second, "ProcessBatch", func() {
				results, err = NewBatchProcessor(tt.workers).ProcessBatch(context.Background(), jobs)
			})
			if err != nil {
				t.Fatalf("ProcessBatch() error = %v", err)
			}
			if len(results) != tt.jobs {
				t.Errorf("got %d results, want %d", len(results), tt.jobs)
			}
		})
	}
}

// TestBatchProcessor_PassesCallerContext ensures jobs see the caller's
// context values and cancellation.
func TestBatchProcessor_PassesCallerContext(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	var seen sync.Map
	jobs := []Job{&testJob{
		id: "ctx-job",
		work: func(jobCtx context.Context) error {
			seen.Store("value", jobCtx.Value(ctxKey{}))
			return nil
		},
	}}

	if _, err := NewBatchProcessor(1).ProcessBatch(ctx, jobs); err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	if v, _ := seen.Load("value"); v != "marker" {
		t.Errorf("job context value = %v, want marker", v)
	}
}

// TestWorkerPool_SubmitDuringShutdown is a regression test: Submit racing
// with Shutdown panicked with "send on closed channel".
func TestWorkerPool_SubmitDuringShutdown(t *testing.T) {
	for iter := 0; iter < 50; iter++ {
		pool := NewWorkerPool(2, 2)
		pool.Start()

		var wg sync.WaitGroup
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				for i := 0; i < 20; i++ {
					err := pool.Submit(&testJob{id: fmt.Sprintf("%d-%d", g, i)})
					if err != nil && !errors.Is(err, ErrPoolShutdown) {
						t.Errorf("Submit() unexpected error = %v", err)
						return
					}
				}
			}(g)
		}

		withTimeout(t, 5*time.Second, "Shutdown", pool.Shutdown)
		wg.Wait()

		if err := pool.Submit(&testJob{id: "late"}); !errors.Is(err, ErrPoolShutdown) {
			t.Fatalf("Submit after Shutdown = %v, want ErrPoolShutdown", err)
		}
	}
}

// TestWorkerPool_ShutdownWithUnreadResults is a regression test: Shutdown
// waited for workers that were blocked sending results nobody read.
func TestWorkerPool_ShutdownWithUnreadResults(t *testing.T) {
	pool := NewWorkerPool(2, 2)
	pool.Start()

	const n = 50
	go func() {
		for i := 0; i < n; i++ {
			if err := pool.Submit(&testJob{id: fmt.Sprintf("job-%d", i)}); err != nil {
				return
			}
		}
	}()
	time.Sleep(50 * time.Millisecond)

	withTimeout(t, 5*time.Second, "Shutdown", pool.Shutdown)

	// Results submitted before shutdown are still readable afterwards and
	// the channel is closed at the end.
	count := 0
	withTimeout(t, 5*time.Second, "draining results", func() {
		for range pool.Results() {
			count++
		}
	})
	if count == 0 {
		t.Error("expected buffered results to be readable after Shutdown")
	}
}

func TestWorkerPool_ShutdownNowClosesResults(t *testing.T) {
	pool := NewWorkerPool(1, 1)
	pool.Start()
	for i := 0; i < 3; i++ {
		if err := pool.Submit(&testJob{id: fmt.Sprintf("job-%d", i)}); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}
	withTimeout(t, 5*time.Second, "ShutdownNow", pool.ShutdownNow)
	withTimeout(t, 5*time.Second, "results closed", func() {
		for range pool.Results() {
		}
	})
}

func TestWorkerPool_ShutdownWithoutStart(t *testing.T) {
	pool := NewWorkerPool(1, 1)
	withTimeout(t, 2*time.Second, "Shutdown", pool.Shutdown)
	withTimeout(t, 2*time.Second, "results closed", func() {
		for range pool.Results() {
		}
	})
}

// TestSyncOrchestrator_SyncPagesTwice is a regression test: the second call
// panicked because the pool's job queue was already closed.
func TestSyncOrchestrator_SyncPagesTwice(t *testing.T) {
	client := &benchmarkNotionClient{}
	ids := []string{"p1", "p2", "p3", "p4", "p5"}

	orchestrator := NewSyncOrchestrator(client, &benchmarkConverter{}, &OrchestratorConfig{
		Workers:    2,
		QueueSize:  2,
		MaxRetries: 0,
		BatchSize:  0, // must fall back to a default instead of looping
		OutputDir:  t.TempDir(),
	})

	for call := 1; call <= 2; call++ {
		var results []Result
		var err error
		withTimeout(t, 5*time.Second, "SyncPages", func() {
			results, err = orchestrator.SyncPages(context.Background(), ids)
		})
		if err != nil {
			t.Fatalf("call %d: SyncPages() error = %v", call, err)
		}
		if len(results) != len(ids) {
			t.Fatalf("call %d: got %d results, want %d", call, len(results), len(ids))
		}
	}
}

// TestDivideBatches_ZeroBatchSize is a regression test: a BatchSize of 0
// looped forever.
func TestDivideBatches_ZeroBatchSize(t *testing.T) {
	tests := []struct {
		batchSize int
		ops       int
		want      int
	}{
		{0, 45, 3},  // default batch size 20
		{-1, 10, 1}, // default batch size 20
		{5, 12, 3},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("size=%d", tt.batchSize), func(t *testing.T) {
			config := DefaultBatchConfig()
			config.BatchSize = tt.batchSize
			processor := NewAdvancedBatchProcessor(config)

			ops := make([]BatchOperation, tt.ops)
			var batches [][]BatchOperation
			withTimeout(t, time.Second, "divideBatches", func() {
				batches = processor.divideBatches(ops)
			})
			if len(batches) != tt.want {
				t.Errorf("got %d batches, want %d", len(batches), tt.want)
			}
		})
	}
}

// TestOptimizedBatch_ScheduledOrder is a regression test: operation types
// without a configured priority were silently dropped, and priorities were
// iterated in random map order.
func TestOptimizedBatch_ScheduledOrder(t *testing.T) {
	ob := NewOptimizedBatch(DefaultBatchConfig())
	callerMeta := map[string]interface{}{"k": "v"}
	ops := []BatchOperation{
		{ID: "u2", Type: "zeta_custom"},
		{ID: "d1", Type: "database_sync"},
		{ID: "u1", Type: "alpha_custom", Metadata: callerMeta},
		{ID: "b1", Type: "block_sync"},
		{ID: "p1", Type: "page_sync"},
		{ID: "p2", Type: "page_sync"},
	}
	for _, op := range ops {
		ob.ScheduleOperation(op)
	}

	got := ob.takeScheduled()

	var ids []string
	for _, op := range got {
		ids = append(ids, op.ID)
	}
	want := []string{"p1", "p2", "b1", "d1", "u1", "u2"}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Errorf("order = %v, want %v", ids, want)
	}
	if got[4].Metadata["priority"] != 4 || got[4].Metadata["k"] != "v" {
		t.Errorf("unknown type metadata = %v", got[4].Metadata)
	}
	if _, modified := callerMeta["priority"]; modified {
		t.Error("caller's metadata map was modified")
	}
	if stats := ob.GetQueueStats(); len(stats) != 0 {
		t.Errorf("queues not cleared: %v", stats)
	}
}

func TestOptimizedBatch_ProcessesUnknownTypes(t *testing.T) {
	ob := NewOptimizedBatch(DefaultBatchConfig())
	ob.ScheduleOperation(BatchOperation{ID: "p1", Type: "page_sync"})
	ob.ScheduleOperation(BatchOperation{ID: "x1", Type: "custom_type"})

	result, err := ob.ProcessScheduledBatches(context.Background())
	if err != nil {
		t.Fatalf("ProcessScheduledBatches() error = %v", err)
	}
	// The custom type is processed (and fails as unknown) instead of being
	// dropped.
	if result.Success+result.Failed != 2 {
		t.Errorf("processed %d operations, want 2 (success=%d failed=%d)", result.Success+result.Failed, result.Success, result.Failed)
	}
}
