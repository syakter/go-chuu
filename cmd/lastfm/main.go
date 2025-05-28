package main

import (
	"flag"
	"fmt"
	"os"

	"go-chuu/lastfm"
)

func main() {
	var (
		apiKey    = flag.String("key", os.Getenv("LASTFM_API_KEY"), "Last.fm API key")
		apiSecret = flag.String("secret", os.Getenv("LASTFM_API_SECRET"), "Last.fm API secret")
		username  = flag.String("user", "", "Last.fm username")
		command   = flag.String("cmd", "", "Command to run (recent, top)")
		limit     = flag.Int("limit", 10, "Number of items to fetch")
		period    = flag.String("period", "7day", "Time period (7day, 1month, 3month, 6month, 12month, overall)")
	)

	flag.Parse()

	if *apiKey == "" || *apiSecret == "" {
		fmt.Println("Error: API key and secret are required")
		fmt.Println("Set them via flags or LASTFM_API_KEY and LASTFM_API_SECRET environment variables")
		os.Exit(1)
	}

	if *username == "" {
		fmt.Println("Error: username is required")
		flag.Usage()
		os.Exit(1)
	}

	if *command == "" {
		fmt.Println("Error: command is required")
		flag.Usage()
		os.Exit(1)
	}

	api := lastfm.New(*apiKey, *apiSecret)

	switch *command {
	case "recent":
		result, err := api.User.GetRecentTracks(lastfm.P{
			"user":  *username,
			"limit": *limit,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s's last %d played songs:\n\n", *username, *limit)
		for i, track := range result.Tracks {
			fmt.Printf("%d. %s - %s\n", i+1, track.Artist.Name, track.Name)
		}

	case "top":
		result, err := api.User.GetTopArtists(lastfm.P{
			"user":   *username,
			"period": *period,
			"limit":  *limit,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s's top %d artists ", *username, *limit)
		switch *period {
		case "7day":
			fmt.Println("for the past week:\n")
		case "1month":
			fmt.Println("for the past month:\n")
		case "3month":
			fmt.Println("for the past 3 months:\n")
		case "6month":
			fmt.Println("for the past 6 months:\n")
		case "12month":
			fmt.Println("for the past year:\n")
		default:
			fmt.Println("of all time:\n")
		}
		for i, artist := range result.Artists {
			fmt.Printf("%d. %s\n", i+1, artist.Name)
		}

	default:
		fmt.Printf("Error: unknown command %q\n", *command)
		fmt.Println("Available commands: recent, top")
		os.Exit(1)
	}
}
