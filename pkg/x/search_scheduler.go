package x

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

const (
	DefaultSearchInterval = 15 * time.Minute
	DefaultDateRange      = 24 * time.Hour
)

// SearchScheduler manages scheduled X searches
type SearchScheduler struct {
	queue         *RequestQueue
	searchConfigs map[string]*ScheduledSearch
	interval      time.Duration
	mu            sync.RWMutex
	stop          chan struct{}
}

// ScheduledSearch represents a configured periodic search
type ScheduledSearch struct {
	ID          string
	Query       string
	Count       int
	DateRange   time.Duration
	LastRunTime time.Time
}

// NewSearchScheduler creates a new search scheduler
func NewSearchScheduler(queue *RequestQueue) *SearchScheduler {
	return &SearchScheduler{
		queue:         queue,
		searchConfigs: make(map[string]*ScheduledSearch),
		interval:      DefaultSearchInterval,
		stop:          make(chan struct{}),
	}
}

// AddScheduledSearch adds a new search to be executed periodically
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

// RemoveScheduledSearch removes a scheduled search by ID
func (s *SearchScheduler) RemoveScheduledSearch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.searchConfigs, id)
	logger.Debugf("Removed scheduled search: %s", id)
}

// Start begins the scheduler
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

// Stop halts the scheduler
func (s *SearchScheduler) Stop() {
	close(s.stop)
	logger.Infof("Search scheduler stopped")
}

// executeScheduledSearches runs all configured searches
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

// InitializeSearches sets up multiple searches in the scheduler
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
