package concurrent

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrPoolShutdown is returned by Submit once the pool is shutting down.
var ErrPoolShutdown = errors.New("worker pool is shutting down")

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

// WorkerPool manages a pool of workers for concurrent job execution.
//
// Results are buffered internally without limit, so workers never block on
// a slow or absent reader: Shutdown always returns once the queued jobs are
// done. Results not read before Shutdown remain readable from Results()
// until it is closed; ShutdownNow discards them.
type WorkerPool struct {
	workers    int
	jobQueue   chan Job
	results    chan Result
	out        chan Result // workers -> dispatcher
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	maxRetries int

	// mu guards closed and every send on jobQueue, so Submit never sends
	// on a closed channel.
	mu      sync.RWMutex
	closed  bool
	closing chan struct{} // closed when shutdown begins
	abort   chan struct{} // closed by ShutdownNow

	startOnce      sync.Once
	dispatcherOnce sync.Once
	shutdownOnce   sync.Once
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	return NewWorkerPoolWithContext(context.Background(), workers, queueSize)
}

// NewWorkerPoolWithContext creates a worker pool whose jobs run with a
// context derived from parent: cancelling parent cancels running jobs and
// makes Submit fail.
func NewWorkerPoolWithContext(parent context.Context, workers int, queueSize int) *WorkerPool {
	if parent == nil {
		parent = context.Background()
	}
	if workers <= 0 {
		workers = 1
	}
	if queueSize <= 0 {
		queueSize = workers * 2
	}

	ctx, cancel := context.WithCancel(parent)
	return &WorkerPool{
		workers:    workers,
		jobQueue:   make(chan Job, queueSize),
		results:    make(chan Result, queueSize),
		out:        make(chan Result),
		ctx:        ctx,
		cancel:     cancel,
		maxRetries: 3,
		closing:    make(chan struct{}),
		abort:      make(chan struct{}),
	}
}

// SetMaxRetries sets the maximum number of retries for failed jobs
func (wp *WorkerPool) SetMaxRetries(retries int) {
	if retries < 0 {
		retries = 0
	}
	wp.maxRetries = retries
}

// Start initiates the worker pool. Calling it more than once, or after
// shutdown, has no effect.
func (wp *WorkerPool) Start() {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	if wp.closed {
		return
	}
	wp.startOnce.Do(func() {
		wp.startDispatcher()
		for i := 0; i < wp.workers; i++ {
			wp.wg.Add(1)
			go wp.worker(i)
		}
	})
}

func (wp *WorkerPool) startDispatcher() {
	wp.dispatcherOnce.Do(func() {
		go wp.dispatch()
	})
}

// Submit adds a job to the queue. It blocks while the queue is full and
// returns ErrPoolShutdown (never panics) once shutdown has begun or the
// pool's context is cancelled.
func (wp *WorkerPool) Submit(job Job) error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if wp.closed {
		return ErrPoolShutdown
	}

	select {
	case wp.jobQueue <- job:
		return nil
	case <-wp.closing:
		return ErrPoolShutdown
	case <-wp.ctx.Done():
		return fmt.Errorf("%w: %v", ErrPoolShutdown, wp.ctx.Err())
	}
}

// Results returns the results channel for reading job outcomes. It is
// closed after shutdown once every buffered result has been delivered.
func (wp *WorkerPool) Results() <-chan Result {
	return wp.results
}

// Shutdown gracefully stops the worker pool: queued jobs are completed and
// Shutdown waits for them. It does not wait for results to be read.
func (wp *WorkerPool) Shutdown() {
	wp.shutdown(false)
}

// ShutdownNow immediately stops the worker pool, cancelling running jobs,
// dropping queued jobs and discarding unread results.
func (wp *WorkerPool) ShutdownNow() {
	wp.shutdown(true)
}

func (wp *WorkerPool) shutdown(now bool) {
	wp.shutdownOnce.Do(func() {
		// Wake any Submit blocked on a full queue so it releases mu.
		close(wp.closing)
		if now {
			close(wp.abort)
			wp.cancel()
		}

		wp.mu.Lock()
		wp.closed = true
		close(wp.jobQueue)
		wp.mu.Unlock()

		// Ensure Results() is eventually closed even if Start was never
		// called.
		wp.startDispatcher()

		wp.wg.Wait()
		wp.cancel()
		close(wp.out)
	})
}

// worker is the main worker routine
func (wp *WorkerPool) worker(_ int) {
	defer wp.wg.Done()

	for {
		select {
		case <-wp.abort:
			return
		default:
		}

		select {
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}

			// Execute job with retry logic
			err := wp.executeWithRetry(job)

			// Hand the result to the dispatcher, which never blocks for
			// long because it buffers internally.
			select {
			case wp.out <- Result{JobID: job.ID(), Error: err}:
			case <-wp.abort:
				return
			}

		case <-wp.abort:
			return
		}
	}
}

// dispatch forwards results from workers to the public results channel,
// buffering as needed, and closes it when the pool has shut down and every
// result has been delivered (or immediately on ShutdownNow).
func (wp *WorkerPool) dispatch() {
	defer close(wp.results)

	var pending []Result
	in := wp.out
	for in != nil || len(pending) > 0 {
		var send chan<- Result
		var next Result
		if len(pending) > 0 {
			send = wp.results
			next = pending[0]
		}

		select {
		case r, ok := <-in:
			if !ok {
				in = nil
				continue
			}
			pending = append(pending, r)
		case send <- next:
			pending = pending[1:]
		case <-wp.abort:
			// Keep receiving so workers are never stuck, but deliver
			// nothing more.
			if in != nil {
				for range in {
				}
			}
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

// ProcessBatch processes a batch of jobs and waits for all to complete.
// Jobs run with a context derived from ctx. Results are read while jobs are
// still being submitted, so any number of jobs can be processed.
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, jobs []Job) ([]Result, error) {
	if len(jobs) == 0 {
		return []Result{}, nil
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("batch processing cancelled: %w", ctx.Err())
	}

	// Create a fresh pool for this batch
	pool := NewWorkerPoolWithContext(ctx, bp.workers, bp.workers*2)
	pool.Start()

	type submitOutcome struct {
		submitted int
		err       error
	}
	submitDone := make(chan submitOutcome, 1)
	go func() {
		n := 0
		for _, job := range jobs {
			if err := pool.Submit(job); err != nil {
				submitDone <- submitOutcome{submitted: n, err: fmt.Errorf("failed to submit job %s: %w", job.ID(), err)}
				return
			}
			n++
		}
		submitDone <- submitOutcome{submitted: n}
	}()

	results := make([]Result, 0, len(jobs))
	expected := len(jobs)
	var submitErr error
	submitCh := submitDone

	for len(results) < expected {
		select {
		case result, ok := <-pool.Results():
			if !ok {
				pool.Shutdown()
				return results, fmt.Errorf("worker pool closed after %d of %d results", len(results), expected)
			}
			results = append(results, result)

		case outcome := <-submitCh:
			submitCh = nil
			expected = outcome.submitted
			submitErr = outcome.err

		case <-ctx.Done():
			pool.ShutdownNow()
			return results, fmt.Errorf("batch processing cancelled: %w", ctx.Err())
		}
	}

	if submitCh != nil {
		// All results are in; the submitter has finished or will shortly.
		outcome := <-submitCh
		submitErr = outcome.err
	}

	pool.Shutdown()

	if submitErr != nil {
		if ctx.Err() != nil {
			return results, fmt.Errorf("batch processing cancelled: %w", ctx.Err())
		}
		return results, submitErr
	}
	return results, nil
}
