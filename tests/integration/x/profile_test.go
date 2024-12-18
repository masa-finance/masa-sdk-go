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

// Test constants for the profile parameters
const (
	testUsername = "elonmusk"
)

var _ = Describe("Profile", func() {
	Context("GetXProfile", func() {
		It("should return profile data for a valid username", func() {
			response, err := x.GetXProfile("", "", testUsername, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).NotTo(BeNil())
			Expect(response.Data).NotTo(BeEmpty())
			Expect(response.RecordCount).To(Equal(1))

			// Pretty print the response
			jsonData, err := json.MarshalIndent(response, "", "    ")
			Expect(err).NotTo(HaveOccurred())

			// Save to file
			testDataPath := filepath.Join("testdata", "profile_response.json")
			err = os.MkdirAll(filepath.Dir(testDataPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(testDataPath, jsonData, 0644)
			Expect(err).NotTo(HaveOccurred())

			fmt.Printf("Response saved to: %s\n", testDataPath)
		})
	})
})
