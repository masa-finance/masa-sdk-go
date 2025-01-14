package x_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/x"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test users and configuration
const (
	// Users to search for
	UserCornelia    = "CorneliaRo15670"
	UserJustin      = "Justinhk1208"
	UserWaifuOracle = "_WaifuOracle_"

	// Search parameters
	SearchSinceDate  = "2025-01-12"
	SearchTweetCount = 450
)

var _ = FDescribe("User Search Queue", func() {
	var queue *x.RequestQueue

	BeforeEach(func() {
		GinkgoWriter.Printf("Initializing queue...\n")
		queue = x.NewRequestQueue(3)

		startChan := make(chan struct{})
		go func() {
			GinkgoWriter.Printf("Starting queue...\n")
			queue.Start()
			GinkgoWriter.Printf("Queue start completed\n")
			close(startChan)
		}()

		select {
		case <-startChan:
			GinkgoWriter.Printf("Queue started successfully\n")
		case <-time.After(5 * time.Second):
			GinkgoWriter.Printf("Queue start timed out\n")
			Fail("Timeout waiting for queue to start")
		}
	})

	AfterEach(func() {
		queue.Stop()
	})

	Context("Specific User Searches", func() {
		It("should fetch tweets from specific users", func() {
			users := []string{
				UserCornelia,
				UserJustin,
				UserWaifuOracle,
			}

			responses := make([]map[string]interface{}, 0)

			// Process each user search
			for _, username := range users {
				query := "from:" + username + " since:" + SearchSinceDate
				GinkgoWriter.Printf("Processing search request for user: %s\n", username)

				responseChan := queue.AddRequest(x.SearchRequest, map[string]interface{}{
					"query": query,
					"count": SearchTweetCount,
				}, x.DefaultPriority)

				// Add timeout context for response handling
				var response interface{}
				select {
				case resp := <-responseChan:
					response = resp
					GinkgoWriter.Printf("Received response for user %s\n", username)
				case <-time.After(30 * time.Second):
					Fail(fmt.Sprintf("Timeout waiting for response for user %s", username))
				}

				if err, ok := response.(error); ok {
					GinkgoWriter.Printf("Error fetching tweets for user %s: %v\n", username, err)
					continue
				}

				responses = append(responses, map[string]interface{}{
					"username": username,
					"query":    query,
					"response": response,
				})

				// Add delay between requests to respect rate limits
				GinkgoWriter.Printf("Waiting 5 seconds before next request...\n")
				time.Sleep(5 * time.Second)
			}

			// Save responses to testdata file
			jsonData, err := json.MarshalIndent(responses, "", "    ")
			Expect(err).NotTo(HaveOccurred())

			testDataPath := filepath.Join("testdata", "user_search_responses.json")
			err = os.MkdirAll(filepath.Dir(testDataPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(testDataPath, jsonData, 0644)
			Expect(err).NotTo(HaveOccurred())

			// Verify responses were collected
			Expect(responses).To(HaveLen(3))

			// Verify queue is empty after processing
			Eventually(func() int {
				return queue.GetQueueLength(x.SearchRequest)
			}, 30*time.Second, 1*time.Second).Should(Equal(0))

			// Additional response validation
			for _, resp := range responses {
				response := resp["response"].(map[string]interface{})
				Expect(response).To(HaveKey("data"))
				Expect(response).To(HaveKey("recordCount"))

				// Verify data is present
				data := response["data"].([]interface{})
				Expect(data).NotTo(BeEmpty())
			}
		})
	})
})
