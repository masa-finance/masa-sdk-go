// Package x provides functionality for interacting with the Masa Protocol X (formerly Twitter) API.
package x

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

const (
	// DefaultSearchInterval is the default time between scheduled search executions (15 minutes)
	DefaultSearchInterval = 15 * time.Minute

	// DefaultDateRange is the default time range to search within (24 hours)
	DefaultDateRange = 24 * time.Hour
)

// SearchScheduler manages scheduled X searches by maintaining a queue of search configurations
// and executing them periodically based on a configured interval. It handles concurrent access
// to search configurations and provides methods to add, remove and execute searches safely.
type SearchScheduler struct {
	queue         *RequestQueue               // Queue for processing search requests
	searchConfigs map[string]*ScheduledSearch // Map of search configurations indexed by ID
	interval      time.Duration               // Time between search executions
	mu            sync.RWMutex                // Mutex for thread-safe access to configs
	stop          chan struct{}               // Channel for stopping the scheduler
}

// ScheduledSearch represents a configured periodic search with parameters and metadata.
// Each search is uniquely identified and tracks its last execution time.
type ScheduledSearch struct {
	ID          string        // Unique identifier for the search
	Query       string        // X search query string
	Count       int           // Number of results to return
	DateRange   time.Duration // Time range to search within
	LastRunTime time.Time     // When the search was last executed
}

// NewSearchScheduler creates a new search scheduler with the provided request queue.
// It initializes the scheduler with default interval and empty search configurations.
//
// Parameters:
//   - queue: RequestQueue instance for processing search requests
//
// Returns:
//   - *SearchScheduler: New scheduler instance
func NewSearchScheduler(queue *RequestQueue) *SearchScheduler {
	return &SearchScheduler{
		queue:         queue,
		searchConfigs: make(map[string]*ScheduledSearch),
		interval:      DefaultSearchInterval,
		stop:          make(chan struct{}),
	}
}

// AddScheduledSearch adds a new search configuration to be executed periodically.
// If dateRange is 0, it uses the DefaultDateRange (24h).
//
// Parameters:
//   - id: Unique identifier for the search
//   - query: X search query string
//   - count: Number of results to return
//   - dateRange: Time range to search within
func (s *SearchScheduler) AddScheduledSearch(id, query string, count int, dateRange time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if dateRange == 0 {
		dateRange = DefaultDateRange
	}

	s.searchConfigs[id] = &ScheduledSearch{
		ID:        id,
		Query:     query,
		Count:     count,
		DateRange: dateRange,
	}
	logger.Debugf("Added scheduled search: %s with query: %s", id, query)
}

// RemoveScheduledSearch removes a scheduled search configuration by its ID.
//
// Parameters:
//   - id: ID of the search to remove
func (s *SearchScheduler) RemoveScheduledSearch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.searchConfigs, id)
	logger.Debugf("Removed scheduled search: %s", id)
}

// Start begins the scheduler, executing searches at the configured interval.
// Runs in a separate goroutine until Stop is called.
func (s *SearchScheduler) Start() {
	ticker := time.NewTicker(s.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.executeScheduledSearches()
			case <-s.stop:
				ticker.Stop()
				return
			}
		}
	}()
	logger.Infof("Search scheduler started with interval: %v", s.interval)
}

// Stop halts the scheduler, preventing further search executions.
func (s *SearchScheduler) Stop() {
	close(s.stop)
	logger.Infof("Search scheduler stopped")
}

// executeScheduledSearches runs all configured searches, adding date filters
// if not already present in the query. Each search is executed asynchronously
// through the request queue.
func (s *SearchScheduler) executeScheduledSearches() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	for _, search := range s.searchConfigs {
		// Format dates in YYYY-MM-DD format
		endTime := now.Format("2006-01-02")
		startTime := now.Add(-search.DateRange).Format("2006-01-02")

		// Check if query already contains date filters
		hasDateFilter := strings.Contains(search.Query, "until:") || strings.Contains(search.Query, "since:")

		var finalQuery string
		if hasDateFilter {
			finalQuery = search.Query
		} else {
			finalQuery = fmt.Sprintf("%s until:%s since:%s",
				search.Query,
				endTime,
				startTime,
			)
		}

		params := SearchParams{
			Query: finalQuery,
			Count: search.Count,
		}

		// Add to queue with default priority
		responseChan := s.queue.AddRequest(SearchRequest, params.ToMap(), DefaultPriority)

		// Handle response asynchronously
		go func(searchID string) {
			response := <-responseChan
			if err, ok := response.(error); ok {
				logger.Errorf("Scheduled search %s failed: %v", searchID, err)
				return
			}
			logger.Debugf("Scheduled search %s completed successfully", searchID)
		}(search.ID)

		search.LastRunTime = now
	}
}

// InitializeSearches sets up multiple searches in the scheduler at once.
// Each search uses the default 24h date range.
//
// Parameters:
//   - searches: Slice of search configurations containing ID, query and count
func (s *SearchScheduler) InitializeSearches(searches []struct {
	ID    string
	Query string
	Count int
}) {
	for _, search := range searches {
		s.AddScheduledSearch(
			search.ID,
			search.Query,
			search.Count,
			DefaultDateRange, // Using the default 24h range
		)
	}
	logger.Infof("Initialized %d scheduled searches", len(searches))
}
