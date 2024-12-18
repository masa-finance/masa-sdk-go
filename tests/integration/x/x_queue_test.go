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

var _ = Describe("RequestQueue", func() {
	var queue *x.RequestQueue

	BeforeEach(func() {
		queue = x.NewRequestQueue(5)
		queue.Start()
	})

	AfterEach(func() {
		queue.Stop()
	})

	Context("Mixed Queue Processing", func() {
		It("should process and save multiple search and profile requests", func() {
			searchResponses := make([]interface{}, 0)
			profileResponses := make([]interface{}, 0)

			// Add search requests
			searchTerms := []string{
				"web3", "blockchain", "crypto", "ethereum",
				"bitcoin", "defi", "nft", "dao",
				"metaverse", "tokenization",
			}

			searchChannels := make([]chan interface{}, len(searchTerms))
			for i, term := range searchTerms {
				searchChannels[i] = queue.AddRequest(x.SearchRequest, map[string]interface{}{
					"query": term,
					"count": 10,
					"additionalProps": map[string]interface{}{
						"fromDate": "2024-01-01",
						"toDate":   "2024-03-20",
					},
				}, i+1)
			}

			// Add profile requests
			profiles := []string{
				"elonmusk", "vitalikbuterin",
				"brendanplayford", "cz_binance",
				"aantonop",
			}

			profileChannels := make([]chan interface{}, len(profiles))
			for i, username := range profiles {
				profileChannels[i] = queue.AddRequest(x.ProfileRequest, map[string]interface{}{
					"username": username,
				}, i+1)
			}

			// Collect search responses
			for i, ch := range searchChannels {
				var response interface{}
				Eventually(ch, 30*time.Second).Should(Receive(&response))
				if _, ok := response.(error); !ok {
					searchResponses = append(searchResponses, map[string]interface{}{
						"query":    searchTerms[i],
						"response": response,
					})
				}
			}

			// Collect profile responses
			for i, ch := range profileChannels {
				var response interface{}
				Eventually(ch, 30*time.Second).Should(Receive(&response))
				if _, ok := response.(error); !ok {
					profileResponses = append(profileResponses, map[string]interface{}{
						"username": profiles[i],
						"response": response,
					})
				}
			}

			// Save search responses
			searchJSON, err := json.MarshalIndent(searchResponses, "", "    ")
			Expect(err).NotTo(HaveOccurred())

			testDataPath := filepath.Join("testdata", "queue_search_responses.json")
			err = os.MkdirAll(filepath.Dir(testDataPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(testDataPath, searchJSON, 0644)
			Expect(err).NotTo(HaveOccurred())

			// Save profile responses
			profileJSON, err := json.MarshalIndent(profileResponses, "", "    ")
			Expect(err).NotTo(HaveOccurred())

			testDataPath = filepath.Join("testdata", "queue_profile_responses.json")
			err = os.WriteFile(testDataPath, profileJSON, 0644)
			Expect(err).NotTo(HaveOccurred())

			// Verify queue is empty
			Eventually(func() int {
				return queue.GetQueueLength(x.SearchRequest)
			}, 30*time.Second, 1*time.Second).Should(Equal(0))

			Eventually(func() int {
				return queue.GetQueueLength(x.ProfileRequest)
			}, 30*time.Second, 1*time.Second).Should(Equal(0))
		})
	})
})
