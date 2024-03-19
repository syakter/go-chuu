package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/syakter/go-lastfm/lastfm"
)

func GetArtistScrobbles(artistName string, network *lastfm.Api) string {
	logger.Debug("Start GetArtistScrobbles", "artistName", artistName)

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

	var usercounts []UserCount
	for user, count := range counts {
		usercounts = append(usercounts, UserCount{Username: user, Playcount: count})
	}
	sort.Slice(usercounts, func(i, j int) bool {
		return usercounts[i].Playcount > usercounts[j].Playcount
	})
	res = fmt.Sprintf("Top %s fans in Kagang:\n", artistName)
	for i, usercount := range usercounts {
		if i == 0 {
			res += fmt.Sprintf("👑. %s: %d scrobbles\n", usercount.Username, usercount.Playcount)
		} else if i == 1 {
			res += fmt.Sprintf("🥈. %s: %d scrobbles\n", usercount.Username, usercount.Playcount)
		} else if i == 2 {
			res += fmt.Sprintf("🥉. %s: %d scrobbles\n", usercount.Username, usercount.Playcount)
		} else {
			res += fmt.Sprintf("%d. %s: %d scrobbles\n", i+1, usercount.Username, usercount.Playcount)
		}
	}
	logger.Debug("GetArtistScrobbles done", "result", res)

	return res
}

func GetTopAlbumsForArtist(artist, username string, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's most listened to albums by %s:\n\n", username, artist)

	result, err := network.Artist.GetTopAlbums(lastfm.P{"artist": artist, "limit": 300})
	if err != nil {
		logger.Error("GetTopAlbums error", "error", err)
		return ""
	}

	var albums []string
	for _, album := range result.Albums {
		albums = append(albums, album.Name)
	}

	counts := make(map[string]int)
	for _, album := range albums {
		result, err := network.Album.GetInfo(lastfm.P{"artist": artist, "album": album, "username": username})

		if err != nil {
			logger.Error("Error during GetTopAlbumsForArtist 1", "error", err)
			continue
		}
		if result.UserPlayCount == "" {
			counts[album] = 0
		} else {
			counts[album], err = strconv.Atoi(result.UserPlayCount)
			if err != nil {
				logger.Error("Error during GetTopAlbumsForArtist 2", "error", err)
			}
		}
	}

	var albumcounts []AlbumCount
	for album, count := range counts {
		albumcounts = append(albumcounts, AlbumCount{AlbumName: album, Playcount: count})
	}

	sort.Slice(albumcounts, func(i, j int) bool {
		return albumcounts[i].Playcount > albumcounts[j].Playcount
	})

	maxCount := 0
	for i, albumcount := range albumcounts {
		res += fmt.Sprintf("%d. %s: %d scrobbles\n", i+1, albumcount.AlbumName, albumcount.Playcount)
		maxCount++
		if maxCount >= 10 {
			break
		}
	}

	return res
}

func GetTopTracksForArtist(artist, username string, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's most listened to tracks by %s:\n\n", username, artist)

	result, err := network.Artist.GetTopTracks(lastfm.P{"artist": artist, "limit": 500})
	if err != nil {
		logger.Error("GetTopTracks error", "error", err)
		return ""
	}

	var tracks []string
	for _, track := range result.Tracks {
		tracks = append(tracks, track.Name)
	}

	counts := make(map[string]int)
	for _, track := range tracks {
		result, err := network.Track.GetInfo(lastfm.P{"artist": artist, "track": track, "username": username})

		if err != nil {
			logger.Error("Error during GetTopTracksForArtist 1", "error", err)
			continue
		}
		if result.UserPlayCount == "" {
			counts[track] = 0
		} else {
			counts[track], err = strconv.Atoi(result.UserPlayCount)
			if err != nil {
				logger.Error("Error during GetTopTracksForArtist 2", "error", err)
			}
		}
	}

	var trackcounts []TrackCount
	for track, count := range counts {
		trackcounts = append(trackcounts, TrackCount{TrackName: track, Playcount: count})
	}

	sort.Slice(trackcounts, func(i, j int) bool {
		return trackcounts[i].Playcount > trackcounts[j].Playcount
	})

	maxCount := 0
	for i, trackcount := range trackcounts {
		res += fmt.Sprintf("%d. %s: %d scrobbles\n", i+1, trackcount.TrackName, trackcount.Playcount)
		maxCount++
		if maxCount >= 10 {
			break
		}
	}

	return res
}
