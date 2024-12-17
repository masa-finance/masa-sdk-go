// Package x_test provides integration tests for the X (formerly Twitter) Masa Protocol API functionality.
package x_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/masa-finance/masa-sdk-go/pkg/x"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test constants for the search parameters
const (
	testQuery    = "Bitcoin"
	testCount    = 1
	testFromDate = "2024-01-01"
	testToDate   = "2024-03-20"
)

// TestSearch verifies the functionality of the X API search endpoint
var _ = Describe("Search", func() {
	// TestSearchX tests the SearchX function specifically
	Context("SearchX", func() {
		// TestSearchXSingleTweet verifies that SearchX returns exactly one tweet
		// when configured with the test parameters
		It("should return one tweet", func() {
			params := x.SearchParams{
				Query: testQuery,
				Count: testCount,
				AdditionalProps: map[string]interface{}{
					"fromDate": testFromDate,
					"toDate":   testToDate,
				},
			}

			response, err := x.SearchX("", "", params)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).NotTo(BeNil())
			Expect(response.Data).To(HaveLen(1))

			// Pretty print the response
			jsonData, err := json.MarshalIndent(response, "", "    ")
			Expect(err).NotTo(HaveOccurred())

			// Save to file
			testDataPath := filepath.Join("testdata", "search_response.json")
			err = os.MkdirAll(filepath.Dir(testDataPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(testDataPath, jsonData, 0644)
			Expect(err).NotTo(HaveOccurred())

			fmt.Printf("Response saved to: %s\n", testDataPath)
		})
	})
})
