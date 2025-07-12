package concurrent

import (
	"context"
	"fmt"
	"sync"
)

// Job represents a unit of work to be processed
type Job interface {
	// Execute performs the job and returns an error if it fails
	Execute(ctx context.Context) error
	// ID returns a unique identifier for the job
	ID() string
}

// Result represents the outcome of a job execution
type Result struct {
	JobID string
	Error error
}

// WorkerPool manages a pool of workers for concurrent job execution
type WorkerPool struct {
	workers      int
	jobQueue     chan Job
	results      chan Result
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	maxRetries   int
	shutdownOnce sync.Once
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}
	if queueSize <= 0 {
		queueSize = workers * 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workers:    workers,
		jobQueue:   make(chan Job, queueSize),
		results:    make(chan Result, queueSize),
		ctx:        ctx,
		cancel:     cancel,
		maxRetries: 3,
	}
}

// SetMaxRetries sets the maximum number of retries for failed jobs
func (wp *WorkerPool) SetMaxRetries(retries int) {
	if retries < 0 {
		retries = 0
	}
	wp.maxRetries = retries
}

// Start initiates the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Submit adds a job to the queue
func (wp *WorkerPool) Submit(job Job) error {
	select {
	case wp.jobQueue <- job:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	}
}

// Results returns the results channel for reading job outcomes
func (wp *WorkerPool) Results() <-chan Result {
	return wp.results
}

// Shutdown gracefully stops the worker pool
func (wp *WorkerPool) Shutdown() {
	wp.shutdownOnce.Do(func() {
		close(wp.jobQueue)
		wp.wg.Wait()
		wp.cancel()
		close(wp.results)
	})
}

// ShutdownNow immediately stops the worker pool
func (wp *WorkerPool) ShutdownNow() {
	wp.shutdownOnce.Do(func() {
		wp.cancel()
		close(wp.jobQueue)
		wp.wg.Wait()
		close(wp.results)
	})
}

// worker is the main worker routine
func (wp *WorkerPool) worker(_ int) {
	defer wp.wg.Done()

	for {
		select {
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}

			// Execute job with retry logic
			err := wp.executeWithRetry(job)

			// Send result
			select {
			case wp.results <- Result{JobID: job.ID(), Error: err}:
			case <-wp.ctx.Done():
				return
			}

		case <-wp.ctx.Done():
			return
		}
	}
}

// executeWithRetry executes a job with retry logic
func (wp *WorkerPool) executeWithRetry(job Job) error {
	var lastErr error

	for attempt := 0; attempt <= wp.maxRetries; attempt++ {
		// Check if context is cancelled before attempting
		if wp.ctx.Err() != nil {
			return fmt.Errorf("job %s cancelled: %w", job.ID(), wp.ctx.Err())
		}

		// Execute the job with the pool's context
		if err := job.Execute(wp.ctx); err != nil {
			lastErr = err
			// Check if context was cancelled
			if wp.ctx.Err() != nil {
				return fmt.Errorf("job %s cancelled: %w", job.ID(), wp.ctx.Err())
			}
			// Continue retrying if we haven't exceeded max attempts
			if attempt < wp.maxRetries {
				continue
			}
		} else {
			// Job succeeded
			return nil
		}
	}

	return fmt.Errorf("job %s failed after %d attempts: %w", job.ID(), wp.maxRetries+1, lastErr)
}

// BatchProcessor provides batch processing capabilities using a worker pool
type BatchProcessor struct {
	workers int
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(workers int) *BatchProcessor {
	return &BatchProcessor{
		workers: workers,
	}
}

// ProcessBatch processes a batch of jobs and waits for all to complete
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, jobs []Job) ([]Result, error) {
	if len(jobs) == 0 {
		return []Result{}, nil
	}

	// Create a fresh pool for this batch
	pool := NewWorkerPool(bp.workers, bp.workers*2)
	pool.Start()
	defer pool.Shutdown()

	// Submit all jobs
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("batch processing cancelled: %w", ctx.Err())
		default:
			if err := pool.Submit(job); err != nil {
				return nil, fmt.Errorf("failed to submit job %s: %w", job.ID(), err)
			}
		}
	}

	// Collect results
	results := make([]Result, 0, len(jobs))
	for i := 0; i < len(jobs); i++ {
		select {
		case result := <-pool.Results():
			results = append(results, result)
		case <-ctx.Done():
			pool.ShutdownNow()
			return results, fmt.Errorf("batch processing cancelled: %w", ctx.Err())
		}
	}

	return results, nil
}
