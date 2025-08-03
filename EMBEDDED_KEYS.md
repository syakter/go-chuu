# Embedding API Keys in Binary

This guide explains how to compile the go-chuu bot with API keys embedded directly in the binary, allowing distribution without exposing keys in plain text files.

## Why Embed Keys?

- **Simplified Distribution**: Send a single binary file instead of binary + .env file
- **Reduced Configuration**: The recipient doesn't need to set up environment variables
- **Basic Obfuscation**: Keys aren't in plain text files (though still extractable by skilled users)
- **Deployment Flexibility**: Works in environments where managing .env files is difficult

## Security Considerations

⚠️ **Important Security Notes:**

- Keys are embedded as strings in the compiled binary
- They are **NOT encrypted**, just not easily visible
- Skilled users can extract them using tools like `strings`, `objdump`, or hex editors
- This provides **obfuscation, not encryption**
- For maximum security, use a proper key management service (AWS KMS, HashiCorp Vault, etc.)

## How to Build with Embedded Keys

### Option 1: Using the Build Script (Recommended)

1. **Ensure your .env file has all required keys:**
   ```bash
   cat .env
   # Should contain:
   # SLACK_BOT_TOKEN=xoxb-your-actual-token
   # SLACK_APP_TOKEN=xapp-your-actual-token  
   # LASTFM_API_KEY=your-actual-api-key
   # LASTFM_API_SECRET=your-actual-secret
   ```

2. **Run the build script:**
   ```bash
   ./build-with-keys.sh
   # Or specify output name:
   ./build-with-keys.sh my-bot-binary
   ```

3. **Distribute the binary:**
   ```bash
   # The binary now contains embedded keys
   scp go-chuu-embedded user@server:/path/to/bot/
   ```

### Option 2: Manual Build with ldflags

```bash
go build -ldflags "\
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedSlackBotToken=xoxb-your-token' \
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedSlackAppToken=xapp-your-token' \
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedLastFMAPIKey=your-api-key' \
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedLastFMSecret=your-secret' \
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedSlackChannelID=C0392543PUY'" \
    -o go-chuu-embedded ./cmd/bot
```

## How It Works

### Priority Order
The bot loads configuration in this priority order:

1. **Environment Variables** (highest priority)
2. **Embedded Keys** (fallback)
3. **Error** if neither is available

### Example Scenarios

**Scenario 1: Embedded keys only**
```bash
# No .env file, no environment variables
./go-chuu-embedded
# Uses embedded keys
```

**Scenario 2: Environment variables override**
```bash
# Even with embedded keys, env vars take precedence
SLACK_BOT_TOKEN=different-token ./go-chuu-embedded
# Uses 'different-token' for Slack, embedded keys for others
```

**Scenario 3: Mixed configuration**
```bash
# Some keys from env, others from embedded
LASTFM_API_KEY=override-key ./go-chuu-embedded
# Uses 'override-key' for LastFM, embedded keys for Slack tokens
```

## Deployment Instructions

### For the Binary Creator (You)

1. Build the binary with your keys:
   ```bash
   ./build-with-keys.sh production-bot
   ```

2. Verify the build:
   ```bash
   # Test that keys are embedded (will fail auth but should start)
   mv .env .env.backup
   ./production-bot
   mv .env.backup .env
   ```

3. Distribute the binary (via secure channel):
   ```bash
   scp production-bot user@server:/opt/go-chuu/
   ```

### For the Binary Recipient

1. **Simple Usage** (keys embedded):
   ```bash
   # Just run the binary - no configuration needed
   ./production-bot
   ```

2. **Override Specific Keys** (if needed):
   ```bash
   # Override just one key
   SLACK_CHANNEL_ID=C1234567890 ./production-bot
   ```

3. **Use .env File** (if preferred):
   ```bash
   # Create .env file - will override embedded keys
   echo "LOG_LEVEL=debug" > .env
   ./production-bot
   ```

## Cross-Platform Builds

### Linux Binary (from any OS)
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "..." -o go-chuu-linux ./cmd/bot
```

### Windows Binary (from any OS)  
```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "..." -o go-chuu.exe ./cmd/bot
```

### macOS Binary (from any OS)
```bash
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "..." -o go-chuu-mac ./cmd/bot
```

## Verification

### Check If Keys Are Embedded
```bash
# This will show embedded strings (including your keys!)
strings go-chuu-embedded | grep -E "xoxb-|xapp-"
```

### Test Configuration Loading
```bash
# Create a simple test
echo 'package main
import("fmt";"github.com/syakter/go-chuu/internal/config")
func main(){cfg,_:=config.LoadEmbedded();fmt.Printf("Bot token: %.8s...\n",cfg.SlackBotToken)}' | 
go run -ldflags "-X 'github.com/syakter/go-chuu/internal/config.EmbeddedSlackBotToken=test-token'" -
```

## Troubleshooting

### Common Issues

**"required environment variable ... is not set"**
- The ldflags didn't work or were malformed
- Rebuild with correct ldflags syntax

**Keys are visible with `strings` command**
- This is expected behavior - keys are obfuscated, not encrypted
- Consider using a key management service for sensitive deployments

**Binary won't start**
- Check that all 4 required keys are embedded
- Verify the binary was built correctly
- Test with a simple configuration test

### Best Practices

1. **Secure the Build Environment**: Only build on trusted machines
2. **Secure Distribution**: Use encrypted channels (SCP, HTTPS, etc.)
3. **Verify Recipients**: Ensure only authorized users receive the binary
4. **Regular Rotation**: Rebuild with new keys periodically
5. **Monitor Access**: Keep logs of who received which binaries
6. **Consider Alternatives**: For production, evaluate proper secret management

## Alternative Approaches

For higher security requirements, consider:

1. **Environment Variables Only**: Most secure, requires proper deployment setup
2. **Docker Secrets**: Good for containerized deployments
3. **Key Management Services**: AWS KMS, Azure Key Vault, HashiCorp Vault
4. **Config Files with Proper Permissions**: 600 permissions, encrypted filesystems
5. **Runtime Key Fetching**: Fetch keys from secure APIs at startup

The embedded keys approach is a pragmatic middle-ground between security and convenience.
