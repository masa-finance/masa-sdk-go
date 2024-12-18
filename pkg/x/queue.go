package x

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

const (
	DefaultMaxConcurrentRequests = 5
	DefaultAPIRequestsPerSecond  = 20
	DefaultRetries               = 10
	DefaultPriority              = 100
	BackoffBaseSleep             = 1 * time.Second
)

// RequestType defines the type of request
type RequestType string

const (
	SearchRequest  RequestType = "search"
	ProfileRequest RequestType = "profile"
)

// RequestData holds the data for a request
type RequestData struct {
	Type     RequestType
	Priority int
	Data     map[string]interface{}
}

// PriorityItem represents an item in the priority queue
type PriorityItem struct {
	data     RequestData
	priority int
	index    int
}

// PriorityQueue implements heap.Interface
type PriorityQueue []*PriorityItem

// Queue methods implementation
func (pq PriorityQueue) Len() int           { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool { return pq[i].priority < pq[j].priority }
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PriorityItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// Worker represents a request processing worker
type Worker struct {
	id         int
	jobChannel chan RequestData
	quit       chan bool
	rateLimit  *time.Ticker
	queue      *RequestQueue
}

// RequestQueue manages request processing
type RequestQueue struct {
	queues            map[RequestType]*PriorityQueue
	workers           []*Worker
	jobChannel        chan RequestData
	maxWorkers        int
	requestsPerSecond float64
	mu                sync.Mutex
}

// NewRequestQueue creates a new RequestQueue instance
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

// AddRequest adds a new request to the queue
func (rq *RequestQueue) AddRequest(reqType RequestType, data map[string]interface{}, priority int) {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if queue, ok := rq.queues[reqType]; ok {
		item := &PriorityItem{
			data: RequestData{
				Type:     reqType,
				Priority: priority,
				Data:     data,
			},
			priority: priority,
		}
		heap.Push(queue, item)
		logger.Debugf("Added request to %s queue with priority %d", reqType, priority)
	}
}

// newWorker creates a new worker
func (rq *RequestQueue) newWorker(id int) *Worker {
	return &Worker{
		id:         id,
		jobChannel: rq.jobChannel,
		quit:       make(chan bool),
		rateLimit:  time.NewTicker(time.Second / time.Duration(rq.requestsPerSecond)),
		queue:      rq,
	}
}

// Start begins processing requests with a worker pool
func (rq *RequestQueue) Start() {
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

// processQueue continuously checks for items in the priority queues
func (rq *RequestQueue) processQueue() {
	ticker := time.NewTicker(100 * time.Millisecond)
	for range ticker.C {
		rq.mu.Lock()
		for _, queue := range rq.queues {
			if queue.Len() > 0 {
				item := heap.Pop(queue).(*PriorityItem)
				rq.jobChannel <- item.data
			}
		}
		rq.mu.Unlock()
	}
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

// processRequest handles a single request with retries
func (w *Worker) processRequest(data RequestData) {
	var err error
	for attempt := 0; attempt < DefaultRetries; attempt++ {
		if attempt > 0 {
			retryDelay := GetRetryDelay(err)
			time.Sleep(retryDelay)
			logger.Debugf("Worker %d: Retrying request after %v delay", w.id, retryDelay)
		}

		switch data.Type {
		case ProfileRequest:
			username, _ := data.Data["username"].(string)
			_, err = GetXProfile("", "", username, nil)
			if err != nil {
				// Wrap generic errors in our custom error types
				if apiErr, ok := err.(*APIError); ok {
					switch apiErr.StatusCode {
					case StatusRateLimit:
						err = NewRateLimitError(0, fmt.Sprintf("worker-%d", w.id))
					case StatusWorkerLimit:
						err = &WorkerRateLimitError{RetryAfter: DefaultRateLimitDelay}
					case StatusGatewayTimeout:
						err = &TimeoutError{
							Operation: "profile_fetch",
							Duration:  TimeoutRetryDelay,
						}
					case StatusServiceDown:
						err = &ConnectionError{Err: fmt.Errorf("service unavailable: %s", apiErr.Message)}
					}
				}
			}

		case SearchRequest:
			query, _ := data.Data["query"].(string)
			count, _ := data.Data["count"].(int)
			if count == 0 {
				count = 10
			}
			resp, err := SearchX("", "", SearchParams{
				Query: query,
				Count: count,
			})
			if err != nil {
				// Wrap generic errors in our custom error types
				if apiErr, ok := err.(*APIError); ok {
					switch apiErr.StatusCode {
					case StatusRateLimit:
						err = NewRateLimitError(0, fmt.Sprintf("worker-%d", w.id))
					case StatusWorkerLimit:
						err = &WorkerRateLimitError{RetryAfter: DefaultRateLimitDelay}
					case StatusGatewayTimeout:
						err = &TimeoutError{
							Operation: "search_query",
							Duration:  TimeoutRetryDelay,
						}
					case StatusServiceDown:
						err = &ConnectionError{Err: fmt.Errorf("service unavailable: %s", apiErr.Message)}
					}
				}

				// Handle empty response case
				if resp != nil && resp.RecordCount == 0 {
					err = &EmptyResponseError{Query: query}
				}
			}
		}

		if err == nil {
			logger.Debugf("Request processed successfully by worker %d", w.id)
			return
		}

		// Check if the error is retryable
		if !IsRetryable(err) {
			logger.Errorf("Worker %d: Non-retryable error encountered: %v", w.id, err)
			return
		}

		// Log the error with more context based on error type
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
		default:
			logger.Warnf("Worker %d: Request failed: %v (attempt %d/%d)",
				w.id, err, attempt+1, DefaultRetries)
		}
	}

	logger.Errorf("Worker %d: Request failed permanently after %d attempts", w.id, DefaultRetries)
}

// Stop gracefully stops all workers and the queue
func (rq *RequestQueue) Stop() {
	logger.Infof("Stopping request queue...")
	for _, worker := range rq.workers {
		worker.quit <- true
	}
	close(rq.jobChannel)
	logger.Infof("Request queue stopped")
}

// GetQueueLength returns the current length of a specific queue
func (rq *RequestQueue) GetQueueLength(reqType RequestType) int {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if queue, ok := rq.queues[reqType]; ok {
		return queue.Len()
	}
	return 0
}

// GetActiveWorkers returns the number of currently active workers
func (rq *RequestQueue) GetActiveWorkers() int {
	return rq.maxWorkers
}
