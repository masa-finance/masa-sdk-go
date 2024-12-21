package x_test

import (
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/x"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("X Mentions", func() {
	var monitor *x.MentionMonitor

	BeforeEach(func() {
		monitor = x.NewMentionMonitor(5)
	})

	AfterEach(func() {
		if monitor != nil {
			monitor.Stop()
			monitor = nil
		}
	})

	FIt("should get mentions for a username", func() {
		username := "getmasafi"
		resultChan := make(chan *x.SearchResponse, 1)

		monitor.OnMentionsReceived(func(user string, response *x.SearchResponse) {
			if user == username {
				resultChan <- response
			}
		})

		// Start monitoring with a 5-second interval for testing
		monitor.MonitorMentionsWithInterval(username, 5*time.Second)

		// Wait for the initial response
		var response *x.SearchResponse
		Eventually(resultChan, 30*time.Second).Should(Receive(&response))

		Expect(response).NotTo(BeNil())
		Expect(response.Data).NotTo(BeEmpty())

		// Verify tweets contain @ mentions
		for _, tweetData := range response.Data {
			tweet, ok := tweetData["Tweet"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Tweet field should be a map[string]interface{}")
			Expect(tweet).NotTo(BeNil())

			text, ok := tweet["Text"].(string)
			Expect(ok).To(BeTrue(), "Text field should be a string")
			Expect(text).To(ContainSubstring("@" + username))
		}
	})
})
