package x

import (
	"container/heap"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/logger"
)

const (
	defaultStateDir = ".masa-sdk"
	queueStateFile  = "queue_state.json"
)

// QueueState represents the serializable state of the queue
type QueueState struct {
	LastSaved time.Time                          `json:"last_saved"`
	Queues    map[RequestType][]RequestDataState `json:"queues"`
}

// RequestDataState is a serializable version of RequestData
type RequestDataState struct {
	Type     RequestType            `json:"type"`
	Priority int                    `json:"priority"`
	Data     map[string]interface{} `json:"data"`
}

// getQueueState extracts the current state from the queue
func (rq *RequestQueue) getQueueState() QueueState {
	state := QueueState{
		LastSaved: time.Now(),
		Queues:    make(map[RequestType][]RequestDataState),
	}

	// Convert current queue items to serializable format
	for reqType, queue := range rq.queues {
		items := make([]RequestDataState, queue.Len())
		tempQueue := *queue
		for i := 0; i < queue.Len(); i++ {
			item := tempQueue[i]
			items[i] = RequestDataState{
				Type:     item.data.Type,
				Priority: item.data.Priority,
				Data:     item.data.Data,
			}
		}
		state.Queues[reqType] = items
	}

	return state
}

// GetCurrentState returns the current queue state without saving to disk
func (rq *RequestQueue) GetCurrentState() QueueState {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	return rq.getQueueState()
}

// SaveState saves the current queue state to disk
func (rq *RequestQueue) SaveState() error {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	state := rq.getQueueState()

	// Ensure directory exists
	stateDir := getStateDir()
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return err
	}

	// Write state to file
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	statePath := filepath.Join(stateDir, queueStateFile)
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return err
	}

	logger.Debugf("Queue state saved to %s", statePath)
	return nil
}

// LoadState loads the queue state from disk
func (rq *RequestQueue) LoadState() error {
	statePath := filepath.Join(getStateDir(), queueStateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debugf("No existing queue state found at %s", statePath)
			return nil
		}
		return err
	}

	var state QueueState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	rq.mu.Lock()
	defer rq.mu.Unlock()

	// Restore items to queues
	for reqType, items := range state.Queues {
		queue := rq.queues[reqType]
		for _, item := range items {
			responseChan := make(chan interface{}, 1)
			heap.Push(queue, &PriorityItem{
				data: RequestData{
					Type:         item.Type,
					Priority:     item.Priority,
					Data:         item.Data,
					ResponseChan: responseChan,
				},
				priority: item.Priority,
			})
		}
	}

	logger.Infof("Queue state loaded from %s (last saved: %s)",
		statePath,
		state.LastSaved.Format(time.RFC3339))
	return nil
}

// getStateDir returns the directory path for state storage
func getStateDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, defaultStateDir)
}
