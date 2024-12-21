package x_test

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/masa-finance/masa-sdk-go/pkg/db"
	"github.com/masa-finance/masa-sdk-go/pkg/nineteen"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Helper function to print current directory
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("error getting current directory: %v", err)
	}
	return dir
}

var _ = Describe("TweetSelector", func() {
	var (
		database *gorm.DB
		aiClient *nineteen.Client
	)

	BeforeEach(func() {
		// Load .env file with explicit error handling
		if err := godotenv.Load("../../../.env"); err != nil {
			fmt.Printf("Error loading .env file: %v\n", err)
			fmt.Printf("Current working directory: %s\n", getCurrentDir())
		}

		// Database connection
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"),
			os.Getenv("DB_PORT"),
		)

		var err error
		database, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		Expect(err).NotTo(HaveOccurred())

		// Initialize AI client
		aiClient = nineteen.NewClient(os.Getenv("NINETEEN_API_KEY"))
	})

	FIt("should select an appropriate tweet for engagement", func() {
		var count int64
		err := database.Model(&db.XSearchRecord{}).Count(&count).Error
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(BeNumerically(">", 0))

		// Add debug logging
		fmt.Printf("Found %d tweets in database\n", count)

		selectedTweet, selection, err := db.SelectTweetForEngagement(database, aiClient)
		if err != nil {
			fmt.Printf("Tweet Selection Error: %+v\n", err)
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(selectedTweet).NotTo(BeNil())
		Expect(selection).NotTo(BeNil())

		// Verify the response structure with string-based tweet ID
		Expect(selection.SelectedTweetID).To(Equal(selectedTweet.TweetID))
		Expect(selection.SelectedTweetText).To(Equal(selectedTweet.Text))
		Expect(selection.SelectedTweetAuthor).To(Equal(selectedTweet.Username))
		Expect(selection.Reason).NotTo(BeEmpty())
		Expect(selection.SuggestedReplyTone).NotTo(BeEmpty())

		// Print selected tweet details for verification
		fmt.Printf("\nSelected Tweet Details:\n")
		fmt.Printf("ID: %s\n", selection.SelectedTweetID)
		fmt.Printf("Author: @%s\n", selection.SelectedTweetAuthor)
		fmt.Printf("Text: %s\n", selection.SelectedTweetText)
		fmt.Printf("Reason: %s\n", selection.Reason)
		fmt.Printf("Suggested Tone: %s\n", selection.SuggestedReplyTone)

		// Additional verification based on actual data
		Expect(selectedTweet.IsRetweet).To(BeFalse())
		Expect(selectedTweet.IsSensitive).To(BeFalse())
		Expect(selectedTweet.Text).NotTo(BeEmpty())

		// Verify the tweet exists in our sample data
		var foundTweet db.XSearchRecord
		err = database.Where("tweet_id = ?", selectedTweet.TweetID).First(&foundTweet).Error
		Expect(err).NotTo(HaveOccurred())
	})
})
