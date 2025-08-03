#!/bin/bash

# Build script for embedding API keys into the binary
# Usage: ./build-with-keys.sh [output-name]

set -e

# Default output name
OUTPUT_NAME="${1:-go-chuu-embedded}"

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo "Error: .env file not found. Please create one with your API keys."
    exit 1
fi

# Source the .env file to load variables
echo "Loading API keys from .env file..."
source .env

# Validate required variables
if [ -z "$SLACK_BOT_TOKEN" ] || [ -z "$SLACK_APP_TOKEN" ] || [ -z "$LASTFM_API_KEY" ] || [ -z "$LASTFM_API_SECRET" ]; then
    echo "Error: Missing required API keys in .env file"
    echo "Required: SLACK_BOT_TOKEN, SLACK_APP_TOKEN, LASTFM_API_KEY, LASTFM_API_SECRET"
    exit 1
fi

# Set default channel ID if not provided
SLACK_CHANNEL_ID="${SLACK_CHANNEL_ID:-C0392543PUY}"

echo "Building binary with embedded API keys..."
echo "Output: $OUTPUT_NAME"

# Build with ldflags to embed the keys
go build -ldflags "\
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedSlackBotToken=$SLACK_BOT_TOKEN' \
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedSlackAppToken=$SLACK_APP_TOKEN' \
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedLastFMAPIKey=$LASTFM_API_KEY' \
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedLastFMSecret=$LASTFM_API_SECRET' \
    -X 'github.com/syakter/go-chuu/internal/config.EmbeddedSlackChannelID=$SLACK_CHANNEL_ID'" \
    -o "$OUTPUT_NAME" ./cmd/bot

echo "✅ Build completed successfully!"
echo "📦 Binary: $OUTPUT_NAME"
echo "🔐 API keys are embedded in the binary"
echo ""
echo "⚠️  Security notes:"
echo "   - The keys are embedded as strings in the binary"
echo "   - They're not encrypted, just not easily visible"
echo "   - Skilled users could still extract them with tools like 'strings'"
echo "   - For maximum security, consider using a key management service"
echo ""
echo "📋 Usage:"
echo "   The binary will use embedded keys by default"
echo "   Environment variables will override embedded keys if set"
echo "   No .env file is needed on the target server"
