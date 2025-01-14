// Package x provides functionality for interacting with the Masa Protocol X (formerly Twitter) API.
// This file implements a priority queue-based request processing system with rate limiting and retries.
package x

import (
	"container/heap"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

// Default configuration values for the request queue system
const (
	// DefaultMaxConcurrentRequests is the default number of concurrent worker goroutines
	DefaultMaxConcurrentRequests = 5
	// DefaultAPIRequestsPerSecond is the default rate limit for API requests
	DefaultAPIRequestsPerSecond = 20
	// DefaultRetries is the default number of retry attempts for failed requests
	DefaultRetries = 10
	// DefaultPriority is the default priority level for requests
	DefaultPriority = 100
	// BackoffBaseSleep is the base sleep duration for exponential backoff
	BackoffBaseSleep = 1 * time.Second
)

// RequestType defines the type of request to be processed
type RequestType string

// Supported request types
const (
	SearchRequest  RequestType = "search"  // For X search requests
	ProfileRequest RequestType = "profile" // For X profile requests
)

// RequestData holds the data and metadata for a request to be processed
type RequestData struct {
	Type         RequestType            // Type of the request (search/profile)
	Priority     int                    // Priority level of the request
	Data         map[string]interface{} // Request parameters
	ResponseChan chan interface{}       // Channel for receiving the response
}

// PriorityItem represents an item in the priority queue with its metadata
type PriorityItem struct {
	data     RequestData // The actual request data
	priority int         // Priority level for queue ordering
	index    int         // Index in the heap for efficient operations
}

// PriorityQueue implements heap.Interface for priority-based request processing
type PriorityQueue []*PriorityItem

// Queue methods implementation for the heap.Interface
func (pq PriorityQueue) Len() int           { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool { return pq[i].priority < pq[j].priority }
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push adds an item to the priority queue
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PriorityItem)
	item.index = n
	*pq = append(*pq, item)
}

// Pop removes and returns the highest priority item from the queue
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// Worker represents a request processing worker goroutine
type Worker struct {
	id         int              // Unique identifier for the worker
	jobChannel chan RequestData // Channel for receiving jobs
	quit       chan bool        // Channel for shutdown signaling
	rateLimit  *time.Ticker     // Rate limiter for API requests
	queue      *RequestQueue    // Reference to parent queue
}

// RequestQueue manages the request processing system
type RequestQueue struct {
	queues            map[RequestType]*PriorityQueue // Map of request type to priority queues
	workers           []*Worker                      // Pool of worker goroutines
	jobChannel        chan RequestData               // Channel for distributing jobs
	maxWorkers        int                            // Maximum number of concurrent workers
	requestsPerSecond float64                        // Rate limit for API requests
	mu                sync.Mutex                     // Mutex for thread-safe operations
	paused            bool                           // Pause state for queue processing
	pauseMux          sync.RWMutex                   // Mutex for pause state access
}

// NewRequestQueue creates and initializes a new RequestQueue instance
//
// Parameters:
//   - maxWorkers: Maximum number of concurrent worker goroutines
//
// Returns:
//   - *RequestQueue: Initialized request queue instance
func NewRequestQueue(maxWorkers int) *RequestQueue {
	if maxWorkers <= 0 {
		maxWorkers = DefaultMaxConcurrentRequests
	}

	rq := &RequestQueue{
		queues:            make(map[RequestType]*PriorityQueue),
		maxWorkers:        maxWorkers,
		jobChannel:        make(chan RequestData, maxWorkers*2),
		requestsPerSecond: DefaultAPIRequestsPerSecond,
	}

	// Initialize queues
	searchQueue := make(PriorityQueue, 0)
	profileQueue := make(PriorityQueue, 0)
	heap.Init(&searchQueue)
	heap.Init(&profileQueue)
	rq.queues[SearchRequest] = &searchQueue
	rq.queues[ProfileRequest] = &profileQueue

	return rq
}

// AddRequest adds a new request to the appropriate queue and returns a response channel
//
// Parameters:
//   - reqType: Type of request (search/profile)
//   - data: Request parameters
//   - priority: Priority level for queue ordering
//
// Returns:
//   - chan interface{}: Channel for receiving the request response
func (rq *RequestQueue) AddRequest(reqType RequestType, data map[string]interface{}, priority int) chan interface{} {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	responseChan := make(chan interface{}, 1)

	if queue, ok := rq.queues[reqType]; ok {
		item := &PriorityItem{
			data: RequestData{
				Type:         reqType,
				Priority:     priority,
				Data:         data,
				ResponseChan: responseChan,
			},
			priority: priority,
		}
		heap.Push(queue, item)
		logger.Infof("Queue update - Added %s request (priority: %d). Current sizes - Search: %d, Profile: %d",
			reqType, priority,
			rq.queues[SearchRequest].Len(),
			rq.queues[ProfileRequest].Len())
	}
	return responseChan
}

// newWorker creates and initializes a new worker instance
func (rq *RequestQueue) newWorker(id int) *Worker {
	return &Worker{
		id:         id,
		jobChannel: rq.jobChannel,
		quit:       make(chan bool),
		rateLimit:  time.NewTicker(time.Second / time.Duration(rq.requestsPerSecond)),
		queue:      rq,
	}
}

// Start begins processing requests by initializing workers and starting the queue processor
func (rq *RequestQueue) Start() {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle shutdown signals in a separate goroutine
	go func() {
		sig := <-sigChan
		logger.Infof("Received signal %v, initiating graceful shutdown...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := rq.StopWithContext(ctx); err != nil {
			logger.Errorf("Error during graceful shutdown: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	// Load existing state
	if err := rq.LoadState(); err != nil {
		logger.Warnf("Failed to load queue state: %v", err)
	}

	// Initialize workers
	rq.workers = make([]*Worker, rq.maxWorkers)
	for i := 0; i < rq.maxWorkers; i++ {
		worker := rq.newWorker(i)
		rq.workers[i] = worker
		go worker.start()
	}

	// Start queue processor
	go rq.processQueue()
}

// processQueue continuously checks for and distributes items from the priority queues
func (rq *RequestQueue) processQueue() {
	ticker := time.NewTicker(100 * time.Millisecond)
	logTicker := time.NewTicker(30 * time.Second)

	for {
		select {
		case <-ticker.C:
			rq.pauseMux.RLock()
			if !rq.paused {
				rq.mu.Lock()
				for _, queue := range rq.queues {
					if queue.Len() > 0 {
						item := heap.Pop(queue).(*PriorityItem)
						rq.jobChannel <- item.data
					}
				}
				rq.mu.Unlock()
			}
			rq.pauseMux.RUnlock()

		case <-logTicker.C:
			rq.pauseMux.RLock()
			isPaused := rq.paused
			rq.pauseMux.RUnlock()

			rq.mu.Lock()
			logger.Infof("Queue status - State: %s | Search: %s, Profile: %s",
				getQueueState(isPaused),
				logger.FormatNumber(rq.queues[SearchRequest].Len()),
				logger.FormatNumber(rq.queues[ProfileRequest].Len()))
			rq.mu.Unlock()
		}
	}
}

// getQueueState returns the queue state as a string
func getQueueState(isPaused bool) string {
	if isPaused {
		return "PAUSED"
	}
	return "ACTIVE"
}

// start begins the worker's processing loop
func (w *Worker) start() {
	for {
		select {
		case job := <-w.jobChannel:
			<-w.rateLimit.C // Rate limiting
			w.processRequest(job)
		case <-w.quit:
			return
		}
	}
}

// processRequest handles a single request with retries and error handling
func (w *Worker) processRequest(data RequestData) {
	var err error
	var response interface{}

	for attempt := 0; attempt < DefaultRetries; attempt++ {
		if attempt > 0 {
			retryDelay := GetRetryDelay(err)
			logger.Debugf("Worker %d: Retrying request after %v delay (attempt %d/%d)",
				w.id, retryDelay, attempt+1, DefaultRetries)
			time.Sleep(retryDelay)
		}

		switch data.Type {
		case ProfileRequest:
			username, _ := data.Data["username"].(string)
			response, err = GetXProfile("", "", username, nil)

		case SearchRequest:
			query, _ := data.Data["query"].(string)
			count, _ := data.Data["count"].(int)
			if count == 0 {
				count = 10
			}
			response, err = SearchX("", "", SearchParams{
				Query: query,
				Count: count,
			})
		}

		if err == nil {
			logger.Debugf("Request processed successfully by worker %d", w.id)
			data.ResponseChan <- response
			close(data.ResponseChan)
			return
		}

		// Convert generic errors to specific error types for better handling
		if apiErr, ok := err.(*APIError); ok {
			switch apiErr.StatusCode {
			case 504:
				err = &TimeoutError{
					Operation: string(data.Type),
					Duration:  TimeoutRetryDelay,
				}
			case 503:
				err = &ConnectionError{Err: fmt.Errorf("service unavailable: %s", apiErr.Message)}
			case StatusRateLimit:
				err = NewRateLimitError(DefaultRateLimitDelay, "")
			case StatusWorkerLimit:
				err = &WorkerRateLimitError{RetryAfter: DefaultRateLimitDelay}
			}
		}

		// Log the error with context and continue retrying
		switch e := err.(type) {
		case *RateLimitError:
			logger.Warnf("Worker %d: Rate limit hit, retry after %v (attempt %d/%d)",
				w.id, e.RetryAfter, attempt+1, DefaultRetries)
		case *WorkerRateLimitError:
			logger.Warnf("Worker %d: All workers rate limited, retry after %v (attempt %d/%d)",
				w.id, e.RetryAfter, attempt+1, DefaultRetries)
		case *TimeoutError:
			logger.Warnf("Worker %d: Request timed out after %v (attempt %d/%d)",
				w.id, e.Duration, attempt+1, DefaultRetries)
		case *ConnectionError:
			logger.Warnf("Worker %d: Connection error: %v (attempt %d/%d)",
				w.id, e.Err, attempt+1, DefaultRetries)
		case *EmptyResponseError:
			logger.Warnf("Worker %d: Empty response received: %v (attempt %d/%d)",
				w.id, e.Query, attempt+1, DefaultRetries)
		case *APIError:
			logger.Warnf("Worker %d: API error (status %d): %s (attempt %d/%d)",
				w.id, e.StatusCode, e.Message, attempt+1, DefaultRetries)
		default:
			logger.Warnf("Worker %d: Request failed: %v (attempt %d/%d)",
				w.id, err, attempt+1, DefaultRetries)
		}
	}

	// All retries exhausted
	data.ResponseChan <- err
	close(data.ResponseChan)
	logger.Errorf("Worker %d: Request failed permanently after %d attempts: %v",
		w.id, DefaultRetries, err)
}

// Stop gracefully stops all workers and the queue
func (rq *RequestQueue) Stop() {
	logger.Infof("Stopping request queue...")

	// Set queue to paused state to prevent new items being processed
	rq.Pause()

	// Wait for in-flight requests to complete
	rq.mu.Lock()
	activeRequests := 0
	for _, queue := range rq.queues {
		activeRequests += queue.Len()
	}
	rq.mu.Unlock()

	if activeRequests > 0 {
		logger.Infof("Waiting for %d active requests to complete...", activeRequests)
		// Give some time for in-flight requests to complete
		time.Sleep(5 * time.Second)
	}

	// Save state before stopping
	if err := rq.SaveState(); err != nil {
		logger.Warnf("Failed to save queue state: %v", err)
	}

	// Signal all workers to stop
	for _, worker := range rq.workers {
		worker.quit <- true
	}

	// Wait for workers to finish
	for _, worker := range rq.workers {
		worker.rateLimit.Stop()
	}

	// Close job channel after all workers have stopped
	close(rq.jobChannel)

	logger.Infof("Request queue stopped gracefully")
}

// StopWithContext provides context-aware shutdown with timeout control
func (rq *RequestQueue) StopWithContext(ctx context.Context) error {
	logger.Infof("Initiating graceful shutdown of request queue...")

	// Set queue to paused state
	rq.Pause()

	done := make(chan struct{})
	go func() {
		rq.Stop()
		close(done)
	}()

	select {
	case <-ctx.Done():
		logger.Warnf("Shutdown deadline exceeded, forcing queue stop")
		// Force close channels
		close(rq.jobChannel)
		for _, worker := range rq.workers {
			worker.rateLimit.Stop()
			close(worker.quit)
		}
		return ctx.Err()
	case <-done:
		logger.Infof("Queue shutdown completed successfully")
		return nil
	}
}

// GetQueueLength returns the current length of a specific queue
//
// Parameters:
//   - reqType: Type of queue to check (search/profile)
//
// Returns:
//   - int: Number of items in the queue
func (rq *RequestQueue) GetQueueLength(reqType RequestType) int {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if queue, ok := rq.queues[reqType]; ok {
		return queue.Len()
	}
	return 0
}

// GetActiveWorkers returns the number of currently active workers
//
// Returns:
//   - int: Number of active worker goroutines
func (rq *RequestQueue) GetActiveWorkers() int {
	return rq.maxWorkers
}

// Pause temporarily stops processing new requests
func (rq *RequestQueue) Pause() {
	rq.pauseMux.Lock()
	defer rq.pauseMux.Unlock()
	rq.paused = true
	logger.Infof("Request queue paused")
}

// Resume continues processing requests
func (rq *RequestQueue) Resume() {
	rq.pauseMux.Lock()
	defer rq.pauseMux.Unlock()
	rq.paused = false
	logger.Infof("Request queue resumed")
}
