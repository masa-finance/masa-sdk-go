<div align="center">
  <img src="assets/masa-logo.png" alt="Masa Logo" width="80"/>
  <h3>Official Go SDK for the Masa Protocol</h3>
  <p>A powerful Go library for interacting with the Masa universe</p>
</div>

[![Go Report Card](https://goreportcard.com/badge/github.com/masa-finance/masa-sdk-go)](https://goreportcard.com/report/github.com/masa-finance/masa-sdk-go)
[![GoDoc](https://godoc.org/github.com/masa-finance/masa-sdk-go?status.svg)](https://godoc.org/github.com/masa-finance/masa-sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- 🔄 **Asynchronous Request Queue** - Built-in priority queue system for handling concurrent API requests
- 🔍 **X (Twitter) Integration** - Comprehensive search and profile data retrieval
- ⚡ **Rate Limiting** - Intelligent rate limiting and retry mechanisms
- 🛡️ **Error Handling** - Robust error handling with custom error types
- 📊 **Response Management** - Channel-based response handling for async operations

## Installation

```bash
go get github.com/masa-finance/masa-sdk-go
```

## Quick Start

### Initialize Queue

```go
queue := x.NewRequestQueue(5) // 5 concurrent workers
queue.Start()
defer queue.Stop()
```

### Search X (Twitter)

```go
responseChan := queue.AddRequest(x.SearchRequest, map[string]interface{}{
    "query": "web3",
    "count": 10,
    "additionalProps": map[string]interface{}{
        "fromDate": "2024-01-01",
        "toDate":   "2024-03-20",
    },
}, x.DefaultPriority)

response := <-responseChan
```

### Get X Profile

```go
responseChan := queue.AddRequest(x.ProfileRequest, map[string]interface{}{
    "username": "elonmusk",
}, x.DefaultPriority)

response := <-responseChan
```

## Queue Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| MaxConcurrentRequests | 5 | Maximum number of concurrent workers |
| APIRequestsPerSecond | 20 | Rate limit for API requests |
| DefaultRetries | 10 | Number of retry attempts |
| DefaultPriority | 100 | Default priority for requests |

## Error Handling

The SDK provides custom error types for different scenarios:
- RateLimitError
- WorkerRateLimitError
- TimeoutError
- ConnectionError
- EmptyResponseError

## Testing

```bash
go test ./tests/integration/... -v
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 📚 [Documentation](https://docs.masa.finance)
- 💬 [Discord Community](https://discord.gg/masa)
- 🐦 [Twitter](https://twitter.com/masa_finance)