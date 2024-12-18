package x_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/x"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SearchScheduler", func() {
	var (
		queue     *x.RequestQueue
		scheduler *x.SearchScheduler
		responses []interface{}
	)

	BeforeEach(func() {
		// Initialize queue with 5 workers
		queue = x.NewRequestQueue(5)
		queue.Start()

		// Create scheduler with queue
		scheduler = x.NewSearchScheduler(queue)
		responses = make([]interface{}, 0)
	})

	AfterEach(func() {
		scheduler.Stop()
		queue.Stop()
	})

	Context("Bitcoin Search Monitoring", func() {
		It("should execute scheduled bitcoin searches every 15 seconds", func() {
			// Add the bitcoin search configuration
			scheduler.AddScheduledSearch(
				"bitcoin-monitor",
				"bitcoin",
				100,            // Get up to 100 results
				15*time.Second, // Set 15-second interval
			)

			// Start the scheduler
			scheduler.Start()

			// Collect responses for 2 cycles
			for i := 0; i < 2; i++ {
				responseChan := queue.AddRequest(x.SearchRequest, map[string]interface{}{
					"query": "bitcoin",
					"count": 100,
				}, x.DefaultPriority)

				response := <-responseChan
				if _, ok := response.(error); !ok {
					responses = append(responses, map[string]interface{}{
						"cycle":    i + 1,
						"response": response,
					})
				}
				time.Sleep(15 * time.Second)
			}

			// Save responses to testdata
			jsonData, err := json.MarshalIndent(responses, "", "    ")
			Expect(err).NotTo(HaveOccurred())

			testDataPath := filepath.Join("testdata", "scheduler_search_responses.json")
			err = os.MkdirAll(filepath.Dir(testDataPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(testDataPath, jsonData, 0644)
			Expect(err).NotTo(HaveOccurred())

			// Verify the queue processes the searches
			Eventually(func() int {
				return queue.GetQueueLength(x.SearchRequest)
			}, 15*time.Second, 1*time.Second).Should(Equal(0))
		})
	})
})
