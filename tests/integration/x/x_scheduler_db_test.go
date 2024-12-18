package x_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/masa-finance/masa-sdk-go/pkg/db"
	"github.com/masa-finance/masa-sdk-go/pkg/x"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var _ = FDescribe("SearchScheduler with Database", func() {
	var (
		queue     *x.RequestQueue
		scheduler *x.SearchScheduler
		database  *gorm.DB
		responses []interface{}
	)

	BeforeEach(func() {
		var err error
		dsn := "host=localhost user=postgres password=postgres dbname=test_db port=5432 sslmode=disable"
		database, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		Expect(err).NotTo(HaveOccurred())

		// Execute migration SQL
		migrationSQL := `
		-- Create x_search_records table
		CREATE TABLE IF NOT EXISTS x_search_records (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMP WITH TIME ZONE,
			updated_at TIMESTAMP WITH TIME ZONE,
			deleted_at TIMESTAMP WITH TIME ZONE,
			search_id VARCHAR NOT NULL,
			query VARCHAR NOT NULL,
			tweet_id VARCHAR NOT NULL,
			user_id VARCHAR NOT NULL,
			username VARCHAR NOT NULL,
			name VARCHAR NOT NULL,
			text TEXT NOT NULL,
			html TEXT,
			posted_at TIMESTAMP WITH TIME ZONE NOT NULL,
			permanent_url VARCHAR NOT NULL,
			conversation_id VARCHAR,
			in_reply_to_id VARCHAR,
			is_reply BOOLEAN NOT NULL DEFAULT FALSE,
			is_retweet BOOLEAN NOT NULL DEFAULT FALSE,
			is_quoted BOOLEAN NOT NULL DEFAULT FALSE,
			is_self_thread BOOLEAN NOT NULL DEFAULT FALSE,
			is_pin BOOLEAN NOT NULL DEFAULT FALSE,
			is_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
			like_count INTEGER NOT NULL DEFAULT 0,
			retweet_count INTEGER NOT NULL DEFAULT 0,
			reply_count INTEGER NOT NULL DEFAULT 0,
			view_count INTEGER NOT NULL DEFAULT 0,
			hashtags TEXT[],
			urls TEXT[],
			photo_urls TEXT[],
			video_urls TEXT[],
			video_hls_urls TEXT[],
			video_previews TEXT[],
			gif_urls TEXT[],
			mentions JSONB,
			place JSONB,
			quoted_tweet_id VARCHAR,
			retweeted_id VARCHAR,
			thread_id VARCHAR,
			cycle INTEGER NOT NULL,
			searched_at TIMESTAMP WITH TIME ZONE NOT NULL
		);

		-- Create indexes
		CREATE INDEX IF NOT EXISTS idx_x_search_records_search_id ON x_search_records(search_id);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_x_search_records_tweet_id ON x_search_records(tweet_id);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_user_id ON x_search_records(user_id);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_username ON x_search_records(username);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_posted_at ON x_search_records(posted_at);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_conversation_id ON x_search_records(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_in_reply_to_id ON x_search_records(in_reply_to_id);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_quoted_tweet_id ON x_search_records(quoted_tweet_id);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_retweeted_id ON x_search_records(retweeted_id);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_thread_id ON x_search_records(thread_id);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_searched_at ON x_search_records(searched_at);
		CREATE INDEX IF NOT EXISTS idx_x_search_records_deleted_at ON x_search_records(deleted_at);
		`
		err = database.Exec(migrationSQL).Error
		Expect(err).NotTo(HaveOccurred())

		// Initialize queue with debug logging
		queue = x.NewRequestQueue(5)
		fmt.Printf("Queue initialized\n")
		queue.Start()

		// Create scheduler with queue
		scheduler = x.NewSearchScheduler(queue)
		responses = make([]interface{}, 0)
	})

	AfterEach(func() {
		scheduler.Stop()
		queue.Stop()

		// Drop the table in cleanup
		err := database.Exec(`DROP TABLE IF EXISTS x_search_records CASCADE;`).Error
		Expect(err).NotTo(HaveOccurred())

		// Cleanup database connection
		sqlDB, err := database.DB()
		Expect(err).NotTo(HaveOccurred())
		err = sqlDB.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Bitcoin Search Monitoring with Database Storage", func() {
		It("should execute scheduled bitcoin searches and store results in database", func() {
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
				fmt.Printf("\nResponse type: %T\n", response)

				if _, ok := response.(error); !ok {
					// Debug the response structure
					fmt.Printf("\nProcessing response structure:\n")
					if respMap, ok := response.(map[string]interface{}); ok {
						fmt.Printf("Top level keys: %v\n", reflect.ValueOf(respMap).MapKeys())
						if data, ok := respMap["Data"].([]interface{}); ok {
							fmt.Printf("Number of tweets: %d\n", len(data))
							if len(data) > 0 {
								fmt.Printf("First tweet structure: %+v\n", data[0])
							}
						}
					}

					responses = append(responses, map[string]interface{}{
						"cycle":    1,
						"response": response,
					})

					searchRecords, err := db.PrepareXSearchRecords([]interface{}{response}, "bitcoin-monitor", "bitcoin")
					if err != nil {
						fmt.Printf("PrepareXSearchRecords error: %v\n", err)
					}
					Expect(err).NotTo(HaveOccurred())

					// Debug the prepared records
					fmt.Printf("Number of prepared records: %d\n", len(searchRecords))
					if len(searchRecords) > 0 {
						fmt.Printf("First record: %+v\n", searchRecords[0])
					}

					// Store records in database
					err = db.SaveXSearchRecords(database, searchRecords)
					Expect(err).NotTo(HaveOccurred())

					// Verify records were stored
					var count int64
					result := database.Model(&db.XSearchRecord{}).Count(&count)
					Expect(result.Error).NotTo(HaveOccurred())
					Expect(count).To(BeNumerically(">", 0))
				}
				time.Sleep(15 * time.Second)
			}

			// Save responses to testdata
			jsonData, err := json.MarshalIndent(responses, "", "    ")
			Expect(err).NotTo(HaveOccurred())

			testDataPath := filepath.Join("testdata", "scheduler_search_db_responses.json")
			err = os.MkdirAll(filepath.Dir(testDataPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(testDataPath, jsonData, 0644)
			Expect(err).NotTo(HaveOccurred())

			// Verify the queue processes the searches
			Eventually(func() int {
				return queue.GetQueueLength(x.SearchRequest)
			}, 15*time.Second, 1*time.Second).Should(Equal(0))

			// Verify database records
			var records []db.XSearchRecord
			result := database.Where("search_id = ?", "bitcoin-monitor").Find(&records)
			Expect(result.Error).NotTo(HaveOccurred())
			Expect(len(records)).To(BeNumerically(">", 0))

			// Verify record fields
			for _, record := range records {
				Expect(record.SearchID).To(Equal("bitcoin-monitor"))
				Expect(record.Query).To(Equal("bitcoin"))
				Expect(record.TweetID).NotTo(BeEmpty())
				Expect(record.UserID).NotTo(BeEmpty())
				Expect(record.Username).NotTo(BeEmpty())
				Expect(record.Text).NotTo(BeEmpty())
			}
		})
	})
})
