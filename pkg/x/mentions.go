// Package x provides functionality for monitoring and managing X (formerly Twitter) mentions
// and interactions through scheduled searches.
//
// The package implements a concurrent monitoring system that periodically checks for
// mentions and replies to specific usernames using Masa's X search API through SN42 or the Masa Protocol. It uses a worker
// pool pattern to handle multiple search requests efficiently.
//
// Example usage:
//
//	// Create a new mention monitor with 5 concurrent workers
//	monitor := x.NewMentionMonitor(5)
//
//	// Start monitoring mentions for a specific username
//	monitor.MonitorMentions("johndoe")
//
//	// The monitor will now check for mentions every 15 minutes
//	// To stop monitoring:
//	monitor.Stop()
//
// The package uses a scheduler and request queue system to manage periodic searches:
//
//	// The scheduler can be used independently for custom search patterns
//	queue := x.NewRequestQueue(3)
//	scheduler := x.NewSearchScheduler(queue)
//
//	scheduler.AddScheduledSearch(
//		"custom-search",
//		"search query",
//		100,
//		15 * time.Minute,
//	)
//
// The monitoring system is designed to be thread-safe and can handle multiple
// concurrent monitoring tasks for different usernames.
package x

import (
	"fmt"
	"time"
)

const (
	// MentionsInterval is the time between mention checks (15 minutes)
	MentionsInterval   = 15 * time.Minute
	DefaultResultCount = 100
)

// MentionMonitor manages scheduled monitoring of X mentions for specific usernames
type MentionMonitor struct {
	scheduler *SearchScheduler
	queue     *RequestQueue
	handlers  []MentionHandler
}

type MentionHandler func(string, *SearchResponse)

func (m *MentionMonitor) OnMentionsReceived(handler MentionHandler) {
	m.handlers = append(m.handlers, handler)
}

// NewMentionMonitor creates a new monitor instance for tracking mentions
func NewMentionMonitor(maxWorkers int) *MentionMonitor {
	queue := NewRequestQueue(maxWorkers)
	return &MentionMonitor{
		scheduler: NewSearchScheduler(queue),
		queue:     queue,
	}
}

func (m *MentionMonitor) formatQuery(username string) string {
	return fmt.Sprintf("(to:%s) OR (@%s)", username, username)
}

func (m *MentionMonitor) getMonitorID(username string) string {
	return fmt.Sprintf("mentions-%s", username)
}

func (m *MentionMonitor) handleSearchResponse(username string, responseChan chan interface{}) {
	if response := <-responseChan; response != nil {
		if searchResp, ok := response.(*SearchResponse); ok {
			for _, handler := range m.handlers {
				handler(username, searchResp)
			}
		}
	}
}

// MonitorMentions starts monitoring mentions for a specific username
func (m *MentionMonitor) MonitorMentions(username string) {
	m.MonitorMentionsWithInterval(username, MentionsInterval)
}

// MonitorMentionsWithInterval starts monitoring mentions for a specific username with a custom interval
func (m *MentionMonitor) MonitorMentionsWithInterval(username string, interval time.Duration) {
	query := m.formatQuery(username)
	monitorID := m.getMonitorID(username)

	m.queue.Start()

	params := SearchParams{
		Query: query,
		Count: DefaultResultCount,
	}
	responseChan := m.queue.AddRequest(SearchRequest, params.ToMap(), DefaultPriority)

	go m.handleSearchResponse(username, responseChan)

	m.scheduler.AddScheduledSearch(
		monitorID,
		query,
		DefaultResultCount,
		interval,
	)

	m.scheduler.Start()
}

// Stop halts the mention monitoring
func (m *MentionMonitor) Stop() {
	m.scheduler.Stop()
	m.queue.Stop()
}
