# go-chuu 🎵

A sophisticated Slack bot that provides music statistics and social features for Last.fm users. Originally designed for the "Kagang" music community, go-chuu helps groups discover music trends, compare listening habits, and stay connected through shared musical experiences.

## Features ✨

### 🎯 Core Commands
- **Now Playing** (`!np`): See what everyone in your group is currently listening to
- **Artist Fans** (`artistname` or `!artist artistname`): Find the biggest fans of any artist in your group
- **Album/Track Fans** (`album by artist`, `!t track by artist`): Compare listening habits for specific releases
- **Personal Stats** (`!top`, `!track`, `!ta`): Get individual top albums, tracks, and artists
- **Group Stats** (`!kga`, `!kgt`): See the most popular content across your entire group
- **Leaderboards** (`!leaderboard`): Weekly scrobble competitions
- **Visual Charts** (`!chart`): Generate beautiful 3x3 album artwork grids

### 🚀 Technical Features
- **High Performance**: Parallel API processing with configurable concurrency limits
- **Smart Caching**: In-memory cache with TTL to reduce API calls and improve response times
- **Robust Error Handling**: Graceful degradation and user-friendly error messages
- **Input Validation**: Comprehensive validation and sanitization of user commands
- **Observability**: Structured logging with multiple levels and performance metrics
- **Security**: Environment-based configuration with no hardcoded secrets

## Quick Start 🚀

### Prerequisites
- Go 1.22+
- Slack Bot Token and App Token
- Last.fm API credentials

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/syakter/go-chuu.git
   cd go-chuu
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Configure environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your credentials
   ```

4. **Build and run**
   ```bash
   go build -o go-chuu ./cmd/bot
   ./go-chuu
   ```

### Docker Deployment

```bash
# Build image
docker build -t go-chuu .

# Run with environment file
docker run --env-file .env --network=host go-chuu
```

## Configuration 📋

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `SLACK_BOT_TOKEN` | Slack Bot User OAuth Token | `xoxb-...` |
| `SLACK_APP_TOKEN` | Slack App Token | `xapp-...` |
| `LASTFM_API_KEY` | Last.fm API Key | `abc123...` |
| `LASTFM_API_SECRET` | Last.fm API Secret | `def456...` |

### Optional Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `USERS` | [default list] | Comma-separated list of Last.fm usernames |
| `MAX_CONCURRENT_REQUESTS` | `10` | Maximum parallel API requests |
| `CACHE_ENABLED` | `true` | Enable/disable response caching |
| `CACHE_TTL` | `5m` | Cache time-to-live |
| `REQUEST_TIMEOUT` | `30s` | API request timeout |
| `SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |
| `SLACK_CHANNEL_ID` | `C0392543PUY` | Default Slack channel for uploads |

## Commands Reference 📖

### Basic Usage
- `!help` - Show all available commands
- `!up` - Display bot uptime

### Music Discovery
- `!np` - Show what everyone is currently playing
- `Radiohead` - Show top Radiohead fans
- `OK Computer by Radiohead` - Show top fans of specific album
- `!t Creep by Radiohead` - Show top fans of specific track

### Personal Stats
- `!top username [period]` - Top albums for user
- `!track username [period]` - Top tracks for user  
- `!ta username [period]` - Top artists for user
- `!rp username [limit]` - Recent tracks (max 20)
- `!chart username [period]` - Generate visual album chart

### Group Stats
- `!kga [period]` - Group's top albums
- `!kgt [period]` - Group's top tracks
- `!leaderboard` - Weekly scrobble leaderboard

### Discovery Features
- `!disco username artist` - User's top albums by specific artist
- `!dt username artist` - User's top tracks by specific artist

**Time Periods**: `7d`, `1m`, `3m`, `6m`, `1y`, `overall`

## Architecture 🏗️

```
cmd/
└── bot/           # Application entry point
internal/
├── cache/         # Caching layer with TTL support
├── charts/        # Album chart generation
├── commands/      # Command parsing and validation
├── config/        # Configuration management
├── errors/        # Custom error types
├── lastfm/        # Last.fm API client with concurrency
├── slack/         # Slack event handling
└── types/         # Shared type definitions
```

### Key Design Principles
- **Clean Architecture**: Separated concerns with dependency injection
- **Concurrent Processing**: Parallel API calls for improved performance
- **Graceful Error Handling**: Fails gracefully with meaningful user feedback
- **Comprehensive Testing**: Unit tests for all core components
- **Observability**: Structured logging and performance metrics

## Development 🛠️

### Running Tests
```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/commands
```

### Code Quality
```bash
# Format code
go fmt ./...

# Lint code (requires golangci-lint)
golangci-lint run

# Vet code
go vet ./...
```

### Building
```bash
# Build for current platform (with build info)
make build

# Build with embedded API keys (requires .env file)
make build-with-keys

# Cross-compile for Windows (perfect for ARM Mac → Windows workflow)
make build-windows-with-keys

# Create release builds for all platforms
make release-with-keys

# See all available targets
make help
```

**Legacy commands (still work):**
```bash
# Build for current platform
go build -o go-chuu ./cmd/bot

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o go-chuu-linux ./cmd/bot
```

## Deployment 🚀

### Docker
```dockerfile
# Multi-stage build for smaller image
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o go-chuu ./cmd/bot

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/go-chuu .
CMD ["./go-chuu"]
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-chuu
spec:
  replicas: 1
  selector:
    matchLabels:
      app: go-chuu
  template:
    metadata:
      labels:
        app: go-chuu
    spec:
      containers:
      - name: go-chuu
        image: go-chuu:latest
        envFrom:
        - secretRef:
            name: go-chuu-secrets
```

## Contributing 🤝

1. **Fork the repository**
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Make your changes** with tests
4. **Run the test suite** (`go test ./...`)
5. **Commit your changes** (`git commit -m 'Add amazing feature'`)
6. **Push to the branch** (`git push origin feature/amazing-feature`)
7. **Open a Pull Request**

### Code Style Guidelines
- Follow standard Go formatting (`go fmt`)
- Write comprehensive tests for new features
- Use meaningful commit messages
- Document public APIs
- Handle errors gracefully

## Troubleshooting 🔧

### Common Issues

**Bot not responding to mentions**
- Verify `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN` are correct
- Check bot has necessary OAuth scopes (`app_mentions:read`, `chat:write`)
- Ensure bot is invited to the channel

**Last.fm API errors**
- Verify API credentials are valid
- Check rate limiting (bot automatically handles this)
- Ensure usernames exist on Last.fm

**Performance issues**
- Increase `MAX_CONCURRENT_REQUESTS` for faster responses
- Enable caching with appropriate `CACHE_TTL`
- Monitor logs for API timeout errors

### Monitoring
```bash
# View logs with structured output
docker logs go-chuu | grep "level=error"

# Monitor cache performance
curl http://localhost:8080/metrics  # If metrics endpoint added
```

## Changelog 📝

### v2.0.0 - Major Refactor
- ✨ Complete architecture redesign with clean separation of concerns
- 🚀 Added parallel processing for significant performance improvements
- 💾 Implemented intelligent caching system
- 🛡️ Enhanced security with environment-based configuration
- 🧪 Comprehensive test suite with >90% coverage
- 📊 Structured logging and error handling
- 🐳 Docker support and deployment configurations
- 📖 Complete documentation and setup guides

### v1.0.0 - Initial Release
- Basic Slack bot functionality
- Last.fm API integration
- Core music statistics commands

## License 📄

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments 🙏

- [Last.fm API](https://www.last.fm/api) for music data
- [Slack API](https://api.slack.com/) for bot platform
- [topster.gg](http://topster.gg/) for album chart data
- The Kagang community for inspiration and testing

---

**Made with ❤️ for music lovers everywhere**
