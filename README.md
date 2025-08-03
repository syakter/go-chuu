# go-chuu 🎵

A sophisticated multi-platform bot that provides music statistics and social features for Last.fm users on both Slack and Discord. Originally designed for the "Kagang" music community, go-chuu helps groups discover music trends, compare listening habits, and stay connected through shared musical experiences.

## Features ✨

### 🎯 Core Commands
- **Now Playing** (`!np`): See what everyone in your group is currently listening to
- **Artist Fans** (`artistname` or `!artist artistname`): Find the biggest fans of any artist in your group
- **Album/Track Fans** (`album by artist`, `!t track by artist`): Compare listening habits for specific releases
- **Personal Stats** (`!top`, `!track`, `!ta`): Get individual top albums, tracks, and artists
- **Group Stats** (`!kga`, `!kgt`): See the most popular content across your entire group
- **Leaderboards** (`!leaderboard`): Weekly scrobble competitions
- **Visual Charts** (`!chart`): Generate beautiful 3x3 album artwork grids
- **Listening Club** (`!lc`): Weekly community album listening and voting system

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
- Last.fm API credentials
- Platform credentials:
  - **For Slack**: Bot Token and App Token
  - **For Discord**: Bot Token

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

| Variable | Description | Example | Required When |
|----------|-------------|---------|---------------|
| `LASTFM_API_KEY` | Last.fm API Key | `abc123...` | Always |
| `LASTFM_API_SECRET` | Last.fm API Secret | `def456...` | Always |
| `SLACK_BOT_TOKEN` | Slack Bot User OAuth Token | `xoxb-...` | Slack enabled |
| `SLACK_APP_TOKEN` | Slack App Token | `xapp-...` | Slack enabled |
| `DISCORD_BOT_TOKEN` | Discord Bot Token | `MTEx...` | Discord enabled |

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
| `ENABLE_SLACK` | `true` | Enable Slack integration |
| `ENABLE_DISCORD` | `false` | Enable Discord integration |
| `SLACK_CHANNEL_ID` | `C0392543PUY` | Default Slack channel for uploads |
| `DISCORD_GUILD_ID` | `` | Discord server ID (optional) |

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

### 📚 Listening Club
- `!lc set Artist - Album` - Set the weekly listening club album
- `!lc vote <1-10> [comment]` - Vote on the current album (1-10 scale)
- `!lc current` - Show current listening club album and voting info
- `!lc results` - Display voting results with statistics and comments
- `!lc clear` - Clear current week (admin function)

**Time Periods**: `7d`, `1m`, `3m`, `6m`, `1y`, `overall`

#### How Listening Club Works

1. **Set Weekly Album**: Any member can set the week's listening club album
2. **Listen & Vote**: Members listen to the album and vote on a 1-10 scale
3. **Add Comments**: Optional comments provide context for ratings
4. **View Results**: See average ratings, vote distribution, and individual comments
5. **Weekly Reset**: Albums are tied to calendar weeks (Monday-Sunday)
6. **Cross-Platform**: Works on both Slack and Discord with unified voting

**Example Usage:**
```
!lc set Radiohead - OK Computer
!lc vote 9 One of the greatest albums ever made!
!lc current
!lc results
```

## Platform Setup 🔧

### Slack Setup

1. **Create a Slack App**
   - Go to [api.slack.com/apps](https://api.slack.com/apps)
   - Click "Create New App" → "From scratch"
   - Choose app name and workspace

2. **Configure Bot Permissions**
   - Go to "OAuth & Permissions"
   - Add these Bot Token Scopes:
     - `app_mentions:read`
     - `chat:write`
     - `files:write`
     - `channels:read`

3. **Enable Socket Mode**
   - Go to "Socket Mode" and enable it
   - Create an App-Level Token with `connections:write` scope

4. **Enable Events**
   - Go to "Event Subscriptions" and enable events
   - Subscribe to `app_mention` bot event

5. **Install to Workspace**
   - Go to "Install App" and install to your workspace
   - Copy the Bot User OAuth Token and App Token

### Discord Setup

1. **Create a Discord Application**
   - Go to [discord.com/developers/applications](https://discord.com/developers/applications)
   - Click "New Application" and give it a name

2. **Create a Bot**
   - Go to the "Bot" section
   - Click "Add Bot"
   - Copy the bot token

3. **Set Bot Permissions**
   - Go to "OAuth2" → "URL Generator"
   - Select "bot" scope
   - Select these permissions:
     - Send Messages
     - Attach Files
     - Read Message History
     - Use Slash Commands (optional)

4. **Invite Bot to Server**
   - Use the generated URL to invite the bot to your Discord server
   - Make sure the bot has appropriate channel permissions

### Environment Configuration Examples

**Slack Only:**
```bash
LASTFM_API_KEY=your_lastfm_key
LASTFM_API_SECRET=your_lastfm_secret
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_APP_TOKEN=xapp-your-app-token
ENABLE_SLACK=true
ENABLE_DISCORD=false
```

**Discord Only:**
```bash
LASTFM_API_KEY=your_lastfm_key
LASTFM_API_SECRET=your_lastfm_secret
DISCORD_BOT_TOKEN=your_discord_token
ENABLE_SLACK=false
ENABLE_DISCORD=true
```

**Both Platforms:**
```bash
LASTFM_API_KEY=your_lastfm_key
LASTFM_API_SECRET=your_lastfm_secret
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_APP_TOKEN=xapp-your-app-token
DISCORD_BOT_TOKEN=your_discord_token
ENABLE_SLACK=true
ENABLE_DISCORD=true
```

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
# Build for current platform
go build -o go-chuu ./cmd/bot

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o go-chuu-linux ./cmd/bot

# Build with version info
go build -ldflags "-X main.version=v2.0.0" -o go-chuu ./cmd/bot
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

**Slack bot not responding to mentions**
- Verify `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN` are correct
- Check bot has necessary OAuth scopes (`app_mentions:read`, `chat:write`)
- Ensure bot is invited to the channel
- Make sure `ENABLE_SLACK=true`

**Discord bot not responding**
- Verify `DISCORD_BOT_TOKEN` is correct
- Check bot has necessary permissions in the server
- Ensure bot is online and has access to the channels
- Make sure `ENABLE_DISCORD=true`
- Try mentioning the bot or sending a DM

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
