package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/syakter/go-lastfm/lastfm"
)

type UserCount struct {
	Username  string
	Playcount int
}

type AlbumCount struct {
	AlbumName string
	Playcount int
}

var group = [19]string{"Codeine_turtle", "odesmut", "dudeactually",
	"z47Breezo", "itsalmostdry", "grittyfemme10",
	"v0__", "Hirammj", "FrozenWaterz", "Silkmoney",
	"Mo98t", "BTGKM9_Redd", "colbster411", "FaRiddim", "Vadermaulkylo", "Schwarrtz", "Xutros", "Billy-Shakes", "maloboosie"}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	SLACK_BOT_TOKEN := os.Getenv("SLACK_BOT_TOKEN")
	SLACK_APP_TOKEN := os.Getenv("SLACK_APP_TOKEN")
	LF_API_KEY := os.Getenv("LF_API_KEY")
	LF_API_SECRET := os.Getenv("LF_API_SECRET")

	slack_api := slack.New(
		SLACK_BOT_TOKEN,
		slack.OptionAppLevelToken(SLACK_APP_TOKEN),
	)

	client := socketmode.New(
		slack_api,
	)

	network := lastfm.New(LF_API_KEY, LF_API_SECRET)

	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)
					continue
				}
				fmt.Printf("Event received: %+v\n", eventsAPIEvent)

				client.Ack(*evt.Request)

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						start := time.Now()
						message := ev.Text
						message = strings.Split(message, ">")[1]
						message = strings.TrimSpace(message)
						res := ParseMessage(message, network)
						if res != "" {
							_, _, err := slack_api.PostMessage(ev.Channel, slack.MsgOptionText(res, false))
							if err != nil {
								fmt.Printf("failed posting message: %v\n", evt)
							}
						} else {
							res = "Who? :extremelaughingemoji:"
							_, _, err := slack_api.PostMessage(ev.Channel, slack.MsgOptionText(res, false))
							if err != nil {
								fmt.Printf("failed posting message: %v\n", evt)
							}
						}
						elapsed := time.Since(start)
						fmt.Printf("time elapsed = %s\n", elapsed)
					default:
						fmt.Printf("event = %v\n", ev)
					}

				default:
					client.Debugf("unsupported Events API event received")
				}

			default:
				fmt.Fprintf(os.Stderr, "unexpected event type received: %s\n", evt.Type)
			}
		}
	}()

	client.Run()
}

func GetArtistScrobbles(artistName string, network *lastfm.Api) string {
	// fmt.Printf("artistName = %s\n", artistName)
	if strings.ToLower(artistName) == "mike jones" {
		return "WHO ⁉"
	}
	artistName = strings.Replace(artistName, "&amp;", "\u0026", 1)
	var res string
	counts := make(map[string]int)
	for _, user := range group {
		result, err := network.Artist.GetInfo(lastfm.P{"artist": artistName, "username": user})
		// fmt.Printf("res: %v\n", result)
		if err != nil {
			fmt.Printf("network.Artist.GetInfo err = %v\n", err)
			// return "Who? :extremelaughingemoji:"
			return fmt.Sprintf("%s", err)
		}
		if result.Stats.UserPlays == "" {
			counts[user] = 0
		} else {
			counts[user], err = strconv.Atoi(result.Stats.UserPlays)
			if err != nil {
				fmt.Printf("strconv.Atoi err = %v\n", err)
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
	// fmt.Printf("result = %v\n", res)
	return res
}

func GetTrackScrobbles(artistName, trackName string, network *lastfm.Api) string {
	// fmt.Printf("artistName = %s, trackName = %s\n", artistName, trackName)
	var res string
	counts := make(map[string]int)
	for _, user := range group {
		result, err := network.Track.GetInfo(lastfm.P{"artist": artistName, "track": trackName, "username": user})
		// fmt.Printf("res: %v\n", result)
		if err != nil {
			fmt.Printf("network.Track.GetInfo err = %v\n", err)
			// return ""
			return fmt.Sprintf("%s", err)
		}
		if result.UserPlayCount == "" {
			counts[user] = 0
		} else {
			counts[user], err = strconv.Atoi(result.UserPlayCount)
			if err != nil {
				fmt.Printf("%v\n", err)
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
	res = fmt.Sprintf("Top %s - %s fans in Kagang:\n\n", artistName, trackName)
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
	// fmt.Printf("result = %v\n", res)
	return res
}

func GetAlbumScrobbles(artistName, albumName string, network *lastfm.Api) string {
	// fmt.Printf("artistName = %s, albumName = %s\n", artistName, albumName)
	albumName = strings.Replace(albumName, "&amp;", "\u0026", 1)
	// fmt.Printf("albumName = %s\n", albumName)
	var res string
	counts := make(map[string]int)
	for _, user := range group {
		result, err := network.Album.GetInfo(lastfm.P{"artist": artistName, "album": albumName, "username": user})
		// fmt.Printf("res: %v\n", result)
		if err != nil {
			// fmt.Printf("network.Track.GetInfo err = %v\n", err)
			// return "Last.fm error dipshit"
			fmt.Printf("%s: %s\n", err, user)
			continue
		}
		if result.UserPlayCount == "" {
			fmt.Printf("%s\n", result)
			counts[user] = 0
		} else {
			fmt.Printf("%s\n", result)
			counts[user], err = strconv.Atoi(result.UserPlayCount)
			if err != nil {
				fmt.Printf("%v\n", err)
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
	res = fmt.Sprintf("Top %s - %s fans in Kagang:\n\n", artistName, albumName)
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
	// fmt.Printf("result = %v\n", res)
	return res
}

func GetRecentTracks(username string, limit int, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's last %d played songs:\n\n", username, limit)
	result, err := network.User.GetRecentTracks(lastfm.P{"user": username, "limit": limit})
	if err != nil {
		fmt.Printf("network.User.GetRecentTracks err = %v\n", err)
	}
	for i, track := range result.Tracks {
		if i >= limit {
			break
		}
		artistName := track.Artist.Name
		trackName := track.Name
		// nowPlaying := track.NowPlaying
		// fmt.Printf("now playing = %s", nowPlaying)
		res += fmt.Sprintf("%d. %s - %s\n", i+1, artistName, trackName)
	}
	// fmt.Printf("result = %v\n", res)
	return res
}

func GetNowPlaying(network *lastfm.Api) string {
	res := "What everyone is listening to right now:\n\n"
	for _, user := range group {
		result, err := network.User.GetRecentTracks(lastfm.P{"user": user, "limit": 1})
		if err != nil {
			// fmt.Printf("GetNowPlaying error: %v\n", err)
			// return "It's Last.fm's fault :agony: "
			fmt.Printf("%s: %s\n", err, user)
			continue
		}
		if len(result.Tracks) > 0 {
			track := result.Tracks[0]
			if track.NowPlaying == "true" {
				artistName := track.Artist.Name
				trackName := track.Name
				res += fmt.Sprintf("%s is listening to %s - %s\n", user, artistName, trackName)
			}
		} else {
			return "Y'all aint listening to shit!"
		}

	}
	// fmt.Printf("result = %v\n", res)
	return res

}

func GetTopAlbumsForArtist(artist, username string, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's most listened to albums by %s:\n\n", username, artist)

	result, err := network.Artist.GetTopAlbums(lastfm.P{"artist": artist, "limit": 35})
	fmt.Printf("Top Albums:\n")
	for _, album := range result.Albums {
		fmt.Printf("%s", album.Name)
	}
	fmt.Printf("Top Albums: %s\n", result.Albums)
	if err != nil {
		// fmt.Printf("GetTopAlbums err = %v\n", err)
		return fmt.Sprintf("%s", err)
	}

	var albums []string
	for _, album := range result.Albums {
		albums = append(albums, album.Name)
	}

	counts := make(map[string]int)
	for _, album := range albums {
		result, err := network.Album.GetInfo(lastfm.P{"artist": artist, "album": album, "username": username})

		if err != nil {
			fmt.Printf("Error during GetTopAlbumsForArtist 1: %v\n", err)
			continue
		}
		if result.UserPlayCount == "" {
			counts[album] = 0
		} else {
			counts[album], err = strconv.Atoi(result.UserPlayCount)
			if err != nil {
				fmt.Printf("Error during GetTopAlbumsForArtist 2: %v\n", err)
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
	// fmt.Printf("result = %v\n", res)
	return res
}

func GetTopAlbums(username, period string, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's top albums for the past ", username)
	switch period {
	case "7d":
		period = "7day"
		res += "7 days:\n\n"
	case "1m":
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
		res = strings.TrimSuffix(res, "in the past ")
		res += "of all time:\n\n"
	}

	result, err := network.User.GetTopAlbums(lastfm.P{"user": username, "period": period, "limit": 10})
	if err != nil {
		fmt.Printf("GetTopAlbums err = %v\n", err)
	}

	for i, album := range result.Albums {
		albumName := album.Name
		artistName := album.Artist.Name
		res += fmt.Sprintf("%d. %s - %s\n", i+1, artistName, albumName)
	}
	// fmt.Printf("result = %v\n", res)
	return res
}

func GetTopArtists(username, period string, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's top artists for the past ", username)
	switch period {
	case "7d":
		period = "7day"
		res += "7 days:\n\n"
	case "1m":
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

	result, err := network.User.GetTopArtists(lastfm.P{"user": username, "period": period, "limit": 10})
	if err != nil {
		fmt.Printf("GetTopAlbums err = %v\n", err)
	}

	for i, artist := range result.Artists {
		artistName := artist.Name
		res += fmt.Sprintf("%d. %s\n", i+1, artistName)
	}
	// fmt.Printf("result = %v\n", res)
	return res
}

func ParseMessage(message string, network *lastfm.Api) string {
	if message == "" {
		return ""
	}

	if strings.HasPrefix(message, "!disco") {
		message = strings.TrimPrefix(message, "!disco")
		message = strings.TrimSpace(message)
		msg := strings.SplitN(message, " ", 2)
		if len(msg) == 2 {
			user := msg[0]
			artist := msg[1]
			return GetTopAlbumsForArtist(artist, user, network)
		} else {
			return ""
		}
	}

	if strings.HasPrefix(message, "!artist") || strings.HasPrefix(message, "!a") {
		message = strings.TrimPrefix(message, "!artist")
		message = strings.TrimSpace(message)
		msg := strings.Split(message, " ")
		user := ""
		period := ""
		if len(msg) == 2 {
			user = msg[0]
			period = msg[1]
		} else {
			user = msg[0]
		}
		return GetTopArtists(user, period, network)
	}

	if strings.HasPrefix(message, "!top") {
		message = strings.TrimPrefix(message, "!top")
		message = strings.TrimSpace(message)
		msg := strings.Split(message, " ")
		user := ""
		period := ""
		if len(msg) == 2 {
			user = msg[0]
			period = msg[1]
		} else {
			user = msg[0]
		}
		return GetTopAlbums(user, period, network)
	}

	if strings.HasPrefix(message, "!np") {
		return GetNowPlaying(network)
	}

	if strings.HasPrefix(message, "!rp") {
		message = strings.TrimPrefix(message, "!rp")
		message = strings.TrimSpace(message)
		msg := strings.Split(message, " ")
		user := ""
		limit := 5
		if len(msg) == 2 {
			user = msg[0]
			limit, _ = strconv.Atoi(msg[1])
		} else if len(msg) == 1 {
			user = msg[0]
		}

		if limit > 20 {
			return "Go fuck yourself"
		}

		return GetRecentTracks(user, limit, network)
	}

	if strings.Contains(message, " by ") {
		if strings.HasPrefix(message, "!t") {
			message = strings.TrimSpace(strings.Split(message, "!t")[1])
			msg := strings.Split(message, " by ")
			trackName := msg[0]
			artistName := msg[1]

			return GetTrackScrobbles(artistName, trackName, network)
		} else {
			msg := strings.Split(message, " by ")
			albumName := msg[0]
			artistName := msg[1]

			return GetAlbumScrobbles(artistName, albumName, network)
		}
	} else {
		artistName := message
		return GetArtistScrobbles(artistName, network)
	}
}
