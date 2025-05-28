package main

import (
	"fmt"
	"strings"

	"go-chuu/lastfm"
)

func GetRecentTracks(username string, limit int, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's last %d played songs:\n\n", username, limit)
	result, err := network.User.GetRecentTracks(lastfm.P{
		"user":   username,
		"limit":  limit,
		"format": "json",
	})
	if err != nil {
		logger.Error("GetRecentTracks error", "error", err)
	}
	for i, track := range result.Tracks {
		if i >= limit {
			break
		}
		artistName := track.Artist.Name
		trackName := track.Name
		res += fmt.Sprintf("%d. %s - %s\n", i+1, artistName, trackName)
	}

	return res
}

func GetTopArtists(username, period string, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's top artists for the past ", username)
	switch period {
	case "7d", "1w":
		period = "7day"
		res += "7 days:\n\n"
	case "1m", "30d":
		period = "1month"
		res += "1 month:\n\n"
	case "3m":
		period = "3month"
		res += "3 months:\n\n"
	case "6m":
		period = "6month"
		res += "6 months:\n\n"
	case "1y":
		period = "12month"
		res += "year:\n\n"
	default:
		period = "overall"
		res = strings.TrimSuffix(res, "for the past ")
		res += "of all time:\n\n"
	}

	result, err := network.User.GetTopArtists(lastfm.P{
		"user":   username,
		"period": period,
		"limit":  10,
		"format": "json",
	})
	if err != nil {
		logger.Error("GetTopAlbums error", "error", err)
	}
	logger.Debug("GetTopArtists", "user", username, "period", period, "limit", 10)

	for i, artist := range result.Artists {
		artistName := artist.Name
		res += fmt.Sprintf("%d. %s\n", i+1, artistName)
	}

	return res
}
