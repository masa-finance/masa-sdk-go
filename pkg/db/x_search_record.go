package db

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lib/pq"
	"github.com/masa-finance/masa-sdk-go/pkg/x"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JSONB represents a PostgreSQL JSONB column
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("invalid scan source")
	}
	return json.Unmarshal(bytes, &j)
}

// Video represents a video attachment in a tweet
type Video struct {
	ID      string `json:"ID"`
	HLSURL  string `json:"HLSURL"`
	Preview string `json:"Preview"`
	URL     string `json:"URL"`
}

// XSearchRecord represents a database record of an X (Twitter) search result from scheduled searches
type XSearchRecord struct {
	gorm.Model
	SearchID       string         `gorm:"index;not null"`         // ID of the scheduled search
	Query          string         `gorm:"not null"`               // Search query used
	TweetID        string         `gorm:"uniqueIndex;not null"`   // Unique ID of the tweet
	UserID         string         `gorm:"index;not null"`         // X user ID who posted
	Username       string         `gorm:"index;not null"`         // X username who posted
	Name           string         `gorm:"not null"`               // Display name of the user
	Text           string         `gorm:"type:text;not null"`     // Tweet content
	HTML           string         `gorm:"type:text"`              // HTML formatted content
	PostedAt       time.Time      `gorm:"index;not null"`         // When tweet was posted
	PermanentURL   string         `gorm:"not null"`               // Permanent link to tweet
	ConversationID string         `gorm:"index"`                  // ID of the conversation thread
	InReplyToID    string         `gorm:"index"`                  // ID of tweet being replied to
	IsReply        bool           `gorm:"not null;default:false"` // Whether tweet is a reply
	IsRetweet      bool           `gorm:"not null;default:false"` // Whether tweet is a retweet
	IsQuoted       bool           `gorm:"not null;default:false"` // Whether tweet is a quote
	IsSelfThread   bool           `gorm:"not null;default:false"` // Whether part of self thread
	IsPin          bool           `gorm:"not null;default:false"` // Whether tweet is pinned
	IsSensitive    bool           `gorm:"not null;default:false"` // Whether content is sensitive
	LikeCount      int            `gorm:"not null;default:0"`     // Number of likes
	RetweetCount   int            `gorm:"not null;default:0"`     // Number of retweets
	ReplyCount     int            `gorm:"not null;default:0"`     // Number of replies
	ViewCount      int            `gorm:"not null;default:0"`     // Number of views
	Hashtags       pq.StringArray `gorm:"type:text[]"`            // Array of hashtags
	URLs           pq.StringArray `gorm:"type:text[]"`            // Array of URLs in tweet
	PhotoURLs      pq.StringArray `gorm:"type:text[]"`            // Array of photo URLs
	VideoURLs      pq.StringArray `gorm:"type:text[]"`            // Array of video URLs
	VideoHLSURLs   pq.StringArray `gorm:"type:text[]"`            // Array of HLS video URLs
	VideoPreviews  pq.StringArray `gorm:"type:text[]"`            // Array of video preview URLs
	GifURLs        pq.StringArray `gorm:"type:text[]"`            // Array of GIF URLs
	Mentions       JSONB          `gorm:"type:jsonb"`             // Full mention data including IDs and names
	Place          JSONB          `gorm:"type:jsonb"`             // Place data if available
	QuotedTweetID  string         `gorm:"index"`                  // ID of quoted tweet
	RetweetedID    string         `gorm:"index"`                  // ID of retweeted tweet
	ThreadID       string         `gorm:"index"`                  // ID of thread if part of one
	Cycle          int            `gorm:"not null"`               // Search cycle number
	SearchedAt     time.Time      `gorm:"index;not null"`         // When search was executed
}

// Add this helper function
func safeTime(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}

	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		// Try parsing with RFC3339 format
		parsed, err := time.Parse(time.RFC3339, t)
		if err == nil {
			return parsed
		}
		// Try Unix timestamp format
		if ts, err := strconv.ParseInt(t, 10, 64); err == nil {
			return time.Unix(ts, 0)
		}
	}
	return time.Time{}
}

// convertToStringSlice safely converts an interface{} to []string
func convertToStringSlice(v interface{}) []string {
	if v == nil {
		return []string{} // Return empty slice instead of nil
	}

	switch t := v.(type) {
	case []interface{}:
		result := make([]string, len(t))
		for i, item := range t {
			if str, ok := item.(string); ok {
				result[i] = str
			}
		}
		return result
	case []string:
		return t
	default:
		return []string{} // Return empty slice for unsupported types
	}
}

// PrepareXSearchRecords converts scheduler search responses into database records
func PrepareXSearchRecords(responses []interface{}, searchID string, query string) ([]XSearchRecord, error) {
	var records []XSearchRecord

	for _, resp := range responses {
		searchResp, ok := resp.(*x.SearchResponse)
		if !ok {
			fmt.Printf("Response is not *x.SearchResponse: %T\n", resp)
			continue
		}

		for _, tweetMap := range searchResp.Data {
			tweet, ok := tweetMap["Tweet"].(map[string]interface{})
			if !ok {
				continue
			}

			// Handle arrays properly
			hashtags := make([]string, 0)
			if hashtagsRaw, exists := tweet["Hashtags"]; exists && hashtagsRaw != nil {
				if hashtagArray, ok := hashtagsRaw.([]interface{}); ok {
					for _, h := range hashtagArray {
						if hashStr, ok := h.(string); ok {
							hashtags = append(hashtags, hashStr)
						}
					}
				}
			}

			// Create record with properly handled arrays
			record := XSearchRecord{
				SearchID:       searchID,
				Query:          query,
				TweetID:        safeString(tweet["ID"]),
				UserID:         safeString(tweet["UserID"]),
				Username:       safeString(tweet["Username"]),
				Name:           safeString(tweet["Name"]),
				Text:           safeString(tweet["Text"]),
				HTML:           safeString(tweet["HTML"]),
				PostedAt:       safeTime(tweet["TimeParsed"]), // Use safeTime instead of direct assertion
				PermanentURL:   safeString(tweet["PermanentURL"]),
				ConversationID: safeString(tweet["ConversationID"]),
				InReplyToID:    safeString(tweet["InReplyToStatusID"]),
				IsReply:        safeBool(tweet["IsReply"]),
				IsRetweet:      safeBool(tweet["IsRetweet"]),
				IsQuoted:       safeBool(tweet["IsQuoted"]),
				IsSelfThread:   safeBool(tweet["IsSelfThread"]),
				IsPin:          safeBool(tweet["IsPin"]),
				IsSensitive:    safeBool(tweet["SensitiveContent"]),
				LikeCount:      safeInt(tweet["Likes"]),
				RetweetCount:   safeInt(tweet["Retweets"]),
				ReplyCount:     safeInt(tweet["Replies"]),
				ViewCount:      safeInt(tweet["Views"]),
				Hashtags:       hashtags,
				URLs:           convertToStringSlice(tweet["URLs"]),
				PhotoURLs:      convertToStringSlice(tweet["Photos"]),
				VideoURLs:      convertToStringSlice(tweet["Videos"]),
				VideoHLSURLs:   convertToStringSlice(tweet["VideoHLSURLs"]),
				VideoPreviews:  convertToStringSlice(tweet["VideoPreviews"]),
				GifURLs:        convertToStringSlice(tweet["GIFs"]),
			}

			records = append(records, record)
		}
	}

	if len(records) == 0 {
		return nil, errors.New("empty slice found")
	}

	return records, nil
}

// AutoMigrateXSearchRecords creates or updates the necessary database tables for X search records
func AutoMigrateXSearchRecords(db *gorm.DB) error {
	return db.AutoMigrate(&XSearchRecord{})
}

// SaveXSearchRecords saves the prepared X search records to the database
func SaveXSearchRecords(db *gorm.DB, records []XSearchRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Use upsert operation with update on conflict
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tweet_id"}},
		UpdateAll: true, // Update all columns if record exists
	}).CreateInBatches(records, 100).Error
}

func safeString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func safeBool(v interface{}) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func safeInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
