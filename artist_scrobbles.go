package main

import (
	"fmt"
	"github.com/syakter/go-lastfm/lastfm"
	"strconv"
	"strings"
)

func GetArtistScrobbles(artistName string, network *lastfm.Api) string {
	logger.Debug("GetArtistScrobbles", "artistName", artistName)

	artistName = strings.Replace(artistName, "&amp;", "\u0026", 1)

	var res string
	counts := make(map[string]int)

	for _, user := range group {
		result, err := network.Artist.GetInfo(lastfm.P{"artist": artistName, "username": user})

		if err != nil {
			logger.Error("GetArtistScrobbles error", "error", err)
			return fmt.Sprintf("%s", err)
		}

		logger.Debug("Received response from artist.getinfo", "artistName", result.Name, "user", user, "userPlays", result.Stats.UserPlays)

		if result.Stats.UserPlays == "" {
			counts[user] = 0
		} else {
			counts[user], err = strconv.Atoi(result.Stats.UserPlays)
			if err != nil {
				logger.Error("GetArtistScrobbles error", "error", err)
			}
		}
	}

	return res
}
