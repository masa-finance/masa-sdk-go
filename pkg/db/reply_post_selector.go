package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/masa-finance/masa-sdk-go/pkg/nineteen"
	"gorm.io/gorm"
)

const (
	maxTweetsToFetch = 15
	systemPrompt     = `You are a tweet selector specializing in TAO and Bittensor ecosystem content. Analyze the provided tweets and select ONE tweet that would be most appropriate to engage with.

Consider these criteria:
- Tweet must be related to TAO, Bittensor, or the broader Bittensor ecosystem
- Tweet has good engagement (likes, retweets, replies)
- Content is relevant and meaningful to the TAO/Bittensor community
- Tweet is not sensitive or controversial
- Preferably not a reply to another tweet
- Recent tweets are preferred

Keywords to look for: TAO, Bittensor, subnet, validator, neuron, meritocracy, AI, decentralized intelligence

IMPORTANT: Return ONLY a valid JSON object with no additional text or prefixes. The JSON must have these fields:
- selected_tweet_id: the ID of the selected tweet (MUST be a string, wrapped in quotes)
- selected_tweet_text: MUST be the EXACT and COMPLETE tweet text, including all emojis and attributions
- selected_tweet_author: the username of the tweet author
- reason: brief explanation of why this tweet was selected and its relevance to TAO/Bittensor
- suggested_reply_tone: suggested tone for the reply (professional, casual, technical, etc)

Example format:
{
  "selected_tweet_id": "1234567890",
  "selected_tweet_text": "🔍 Example tweet with emojis 🚀 Made using Tool by @user",
  "selected_tweet_author": "username",
  "reason": "explanation",
  "suggested_reply_tone": "casual"
}`
)

type TweetSelectionResponse struct {
	SelectedTweetID     string `json:"selected_tweet_id"`
	SelectedTweetText   string `json:"selected_tweet_text"`
	SelectedTweetAuthor string `json:"selected_tweet_author"`
	Reason              string `json:"reason"`
	SuggestedReplyTone  string `json:"suggested_reply_tone"`
}

// UnmarshalJSON implements custom unmarshaling for TweetSelectionResponse
func (t *TweetSelectionResponse) UnmarshalJSON(data []byte) error {
	// Create an auxiliary struct with interface{} for selected_tweet_id
	aux := struct {
		SelectedTweetID     interface{} `json:"selected_tweet_id"`
		SelectedTweetText   string      `json:"selected_tweet_text"`
		SelectedTweetAuthor string      `json:"selected_tweet_author"`
		Reason              string      `json:"reason"`
		SuggestedReplyTone  string      `json:"suggested_reply_tone"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert selected_tweet_id to string regardless of its original type
	switch v := aux.SelectedTweetID.(type) {
	case string:
		t.SelectedTweetID = v
	case float64:
		t.SelectedTweetID = fmt.Sprintf("%.0f", v)
	case int64:
		t.SelectedTweetID = fmt.Sprintf("%d", v)
	case int:
		t.SelectedTweetID = fmt.Sprintf("%d", v)
	default:
		return fmt.Errorf("unexpected type for selected_tweet_id: %T", v)
	}

	// Copy the rest of the fields
	t.SelectedTweetText = aux.SelectedTweetText
	t.SelectedTweetAuthor = aux.SelectedTweetAuthor
	t.Reason = aux.Reason
	t.SuggestedReplyTone = aux.SuggestedReplyTone

	return nil
}

func normalizeText(s string) string {
	// Replace three dots with ellipsis
	s = strings.ReplaceAll(s, "...", "…")

	// Replace multiple spaces/newlines with single space
	s = strings.Join(strings.Fields(s), " ")

	return strings.TrimSpace(s)
}

func SelectTweetForEngagement(db *gorm.DB, aiClient *nineteen.Client) (*XSearchRecord, *TweetSelectionResponse, error) {
	var tweets []XSearchRecord

	// Fetch latest tweets, ordered by posted time
	if err := db.Order("posted_at DESC").
		Limit(maxTweetsToFetch).
		Where("is_sensitive = ?", false).
		Find(&tweets).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to fetch tweets: %w", err)
	}

	if len(tweets) == 0 {
		return nil, nil, fmt.Errorf("no tweets found")
	}

	// Debug logging for fetched tweets
	fmt.Printf("\n=== First 5 Fetched Tweets ===\n")
	for i, tweet := range tweets[:min(5, len(tweets))] {
		fmt.Printf("%d. ID: %s | Author: @%s | Text: %q\n", i+1, tweet.TweetID, tweet.Username, tweet.Text)
	}
	fmt.Printf("==================\n\n")

	// Prepare tweets data for AI analysis
	tweetsData := prepareTweetsForAI(tweets)

	// Create chat completion request
	messages := []nineteen.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: tweetsData},
	}

	req := nineteen.NewChatCompletionRequest(messages)
	// Explicitly set parameters
	req.Model = nineteen.DefaultModel
	req.Temperature = 0.7 // Slightly more creative
	req.MaxTokens = 200   // Shorter response
	req.TopP = 0.9        // More focused on likely responses

	// Add debug logging
	fmt.Printf("Sending request to AI with %d tweets\n", len(tweets))
	fmt.Printf("First tweet: %s\n", tweets[0].Text)

	resp, err := aiClient.CreateChatCompletion(context.Background(), req)
	if err != nil {
		fmt.Printf("Raw AI error: %+v\n", err)
		return nil, nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// Enhanced debug logging
	fmt.Printf("\n=== Raw LLM Response ===\n")
	fmt.Printf("Content: %s\n", resp.Choices[0].Message.Content)
	fmt.Printf("Finish Reason: %s\n", resp.Choices[0].FinishReason)
	fmt.Printf("Total Tokens: %d\n", resp.Usage.TotalTokens)
	fmt.Printf("=====================\n\n")

	// After parsing AI response
	fmt.Printf("\n=== AI Response Parsing ===\n")
	fmt.Printf("Raw AI Response: %q\n", resp.Choices[0].Message.Content)

	var selection TweetSelectionResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &selection); err != nil {
		fmt.Printf("JSON Unmarshal Error: %+v\n", err)
		fmt.Printf("Attempted to parse: %s\n", resp.Choices[0].Message.Content)
		return nil, nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// Debug text normalization
	fmt.Printf("\n=== Text Normalization ===\n")
	fmt.Printf("Before normalization: %q\n", selection.SelectedTweetText)
	selection.SelectedTweetText = normalizeText(selection.SelectedTweetText)
	fmt.Printf("After normalization: %q\n", selection.SelectedTweetText)

	// Find the selected tweet
	var selectedTweet XSearchRecord
	if err := db.Where("tweet_id = ?", selection.SelectedTweetID).First(&selectedTweet).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to fetch selected tweet: %w", err)
	}

	// Debug database tweet
	fmt.Printf("\n=== Database Tweet ===\n")
	fmt.Printf("Before normalization: %q\n", selectedTweet.Text)
	selectedTweet.Text = normalizeText(selectedTweet.Text)
	fmt.Printf("After normalization: %q\n", selectedTweet.Text)

	// Compare texts
	fmt.Printf("\n=== Text Comparison ===\n")
	fmt.Printf("AI Text: %q\n", selection.SelectedTweetText)
	fmt.Printf("DB Text: %q\n", selectedTweet.Text)
	fmt.Printf("Texts match: %v\n", selection.SelectedTweetText == selectedTweet.Text)
	fmt.Printf("Text lengths - AI: %d, DB: %d\n", len(selection.SelectedTweetText), len(selectedTweet.Text))

	return &selectedTweet, &selection, nil
}

func prepareTweetsForAI(tweets []XSearchRecord) string {
	tweetsList := "Analyze these tweets and select one to engage with:\n\n"

	for _, tweet := range tweets {
		tweetData := fmt.Sprintf(`Tweet ID: %s
Username: @%s
Posted: %s
Content: %s
Engagement: %d likes, %d retweets, %d replies
Is Reply: %v
Is Retweet: %v
Permanent URL: %s

---

`,
			tweet.TweetID,
			tweet.Username,
			tweet.PostedAt.Format("2006-01-02 15:04:05"),
			tweet.Text,
			tweet.LikeCount,
			tweet.RetweetCount,
			tweet.ReplyCount,
			tweet.IsReply,
			tweet.IsRetweet,
			tweet.PermanentURL,
		)
		tweetsList += tweetData
	}

	return tweetsList
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
