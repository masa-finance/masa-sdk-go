-- Create x_search_records table
CREATE TABLE x_search_records (
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
CREATE INDEX idx_x_search_records_search_id ON x_search_records(search_id);
CREATE UNIQUE INDEX idx_x_search_records_tweet_id ON x_search_records(tweet_id);
CREATE INDEX idx_x_search_records_user_id ON x_search_records(user_id);
CREATE INDEX idx_x_search_records_username ON x_search_records(username);
CREATE INDEX idx_x_search_records_posted_at ON x_search_records(posted_at);
CREATE INDEX idx_x_search_records_conversation_id ON x_search_records(conversation_id);
CREATE INDEX idx_x_search_records_in_reply_to_id ON x_search_records(in_reply_to_id);
CREATE INDEX idx_x_search_records_quoted_tweet_id ON x_search_records(quoted_tweet_id);
CREATE INDEX idx_x_search_records_retweeted_id ON x_search_records(retweeted_id);
CREATE INDEX idx_x_search_records_thread_id ON x_search_records(thread_id);
CREATE INDEX idx_x_search_records_searched_at ON x_search_records(searched_at);
CREATE INDEX idx_x_search_records_deleted_at ON x_search_records(deleted_at); 