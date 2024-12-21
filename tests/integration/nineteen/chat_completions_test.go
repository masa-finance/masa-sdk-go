package nineteen_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/masa-finance/masa-sdk-go/pkg/nineteen"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Nineteen AI Client", func() {
	var (
		client *nineteen.Client
		apiKey string
	)

	BeforeEach(func() {
		err := godotenv.Load("../../../.env")
		Expect(err).NotTo(HaveOccurred())

		apiKey = os.Getenv("NINETEEN_API_KEY")
		Expect(apiKey).NotTo(BeEmpty(), "NINETEEN_API_KEY must be set in .env")

		client = nineteen.NewClient(apiKey)
	})

	Context("Non-streaming Chat Completion", func() {
		It("should successfully complete a chat request", func() {
			messages := []nineteen.Message{
				{
					Role:    "user",
					Content: "What is the capital of France?",
				},
			}

			req := nineteen.NewChatCompletionRequest(messages)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			response, err := client.CreateChatCompletion(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).NotTo(BeNil())
			Expect(response.Choices).To(HaveLen(1))
			Expect(response.Choices[0].Message.Content).To(ContainSubstring("Paris"))
		})
	})

	Context("Streaming Chat Completion", func() {
		It("should successfully stream chat responses", func() {
			messages := []nineteen.Message{
				{
					Role:    "user",
					Content: "Please respond with only numbers, counting from 1 to 3.",
				},
			}

			req := nineteen.NewChatCompletionRequest(messages)
			req.Stream = true
			req.MaxTokens = 50
			req.Temperature = 0.1

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			chunkChan, errChan := client.CreateChatCompletionStream(ctx, req)

			var fullResponse string
			done := make(chan bool)

			go func() {
				defer close(done)
				for {
					select {
					case chunk, ok := <-chunkChan:
						if !ok {
							return
						}
						for _, choice := range chunk.Choices {
							content := choice.Message.Content
							if choice.Delta != nil {
								content = choice.Delta.Content
							}
							if content != "" {
								fullResponse += content
								fmt.Printf("Chunk content: %s\n", content)
							}
						}
					case err := <-errChan:
						if err != nil {
							fmt.Printf("Stream error: %v\n", err)
						}
						return
					case <-ctx.Done():
						return
					}
				}
			}()

			Eventually(done, 30*time.Second).Should(BeClosed())
			fmt.Printf("\nFull response: %s\n", fullResponse)

			Expect(fullResponse).NotTo(BeEmpty(), "Expected non-empty response")

			// More flexible number checking
			containsNumbers := false
			for _, num := range []string{"1", "2", "3"} {
				if strings.Contains(fullResponse, num) {
					containsNumbers = true
					break
				}
			}
			Expect(containsNumbers).To(BeTrue(), "Expected response to contain at least one number")
		})
	})

	Context("Rate Limiting", func() {
		It("should respect rate limits", func() {
			// Create a client with a very low rate limit for testing
			client := nineteen.NewClient(apiKey, nineteen.WithRateLimit(2.0))

			messages := []nineteen.Message{
				{
					Role:    "user",
					Content: "Hello",
				},
			}
			req := nineteen.NewChatCompletionRequest(messages)

			// Make multiple requests in quick succession
			start := time.Now()
			for i := 0; i < 3; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				resp, err := client.CreateChatCompletion(ctx, req)
				cancel()

				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
			}

			// With a rate limit of 2 RPS, 3 requests should take at least 1 second
			duration := time.Since(start)
			Expect(duration).To(BeNumerically(">=", 1*time.Second))
		})
	})
})
