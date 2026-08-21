package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/syakter/go-chuu/internal/cache"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/lastfm"
)

func main() {
	// Create minimal config for testing (without Slack)
	apiKey := os.Getenv("LASTFM_API_KEY")
	apiSecret := os.Getenv("LASTFM_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: LASTFM_API_KEY and LASTFM_API_SECRET environment variables required")
		os.Exit(1)
	}

	cfg := &config.Config{
		LastFMAPIKey:          apiKey,
		LastFMAPISecret:       apiSecret,
		MaxConcurrentRequests: 10,
		Users:                 config.DefaultUsers,
		CacheTTL:              5 * time.Minute,
		RequestTimeout:        30 * time.Second,
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Create cache
	cache := cache.NewInMemoryCache(100)

	// Create Last.fm client
	client := lastfm.NewClient(cfg, cache, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Example: Get artist scrobbles
	artist := "Radiohead"
	if len(os.Args) > 1 {
		artist = os.Args[1]
	}

	fmt.Printf("Fetching scrobbles for '%s'...\n", artist)
	results, err := client.GetArtistScrobbles(ctx, artist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nResults for '%s':\n", artist)
	for _, result := range results {
		fmt.Printf("  - %s: %d plays\n", result.Username, result.Playcount)
	}

	// Example: Get now playing
	fmt.Printf("\nChecking who's listening now...\n")
	nowPlaying, err := client.GetNowPlaying(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting now playing: %v\n", err)
	} else if len(nowPlaying) == 0 {
		fmt.Println("  No one is listening right now")
	} else {
		for user, track := range nowPlaying {
			fmt.Printf("  - %s: %s\n", user, track)
		}
	}
}
