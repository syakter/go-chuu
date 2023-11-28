package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"

	// "net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/syakter/go-lastfm/lastfm"
)

type PrettyHandlerOptions struct {
	SlogOpts slog.HandlerOptions
}

type PrettyHandler struct {
	slog.Handler
	l *log.Logger
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String() + ":"

	switch r.Level {
	case slog.LevelDebug:
		level = color.MagentaString(level)
	case slog.LevelInfo:
		level = color.BlueString(level)
	case slog.LevelWarn:
		level = color.YellowString(level)
	case slog.LevelError:
		level = color.RedString(level)
	}

	fields := make(map[string]interface{}, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.Any()

		return true
	})

	b, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}

	timeStr := r.Time.Format("[15:05:05.000]")
	msg := color.CyanString(r.Message)

	h.l.Println(timeStr, level, msg, color.WhiteString(string(b)))

	return nil
}

func NewPrettyHandler(
	out io.Writer,
	opts PrettyHandlerOptions,
) *PrettyHandler {
	h := &PrettyHandler{
		Handler: slog.NewJSONHandler(out, &opts.SlogOpts),
		l:       log.New(out, "", 0),
	}

	return h
}

type UserCount struct {
	Username  string
	Playcount int
}

type AlbumCount struct {
	AlbumName string
	Playcount int
}

type TrackCount struct {
	TrackName string
	Playcount int
}

var group = [20]string{"Codeine_turtle", "odesmut", "dudeactually",
	"z47Breezo", "itsalmostdry",
	"v0__", "Hirammj", "FrozenWaterz", "Silkmoney",
	"Mo98t", "BTGKM9_Redd", "colbster411", "FaRiddim", "Vadermaulkylo",
	"Schwarrtz", "Xutros", "Billy-Shakes", "maloboosie", "icy_twat", "junkiesRpeople"}

var startTime time.Time

// var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
var opts PrettyHandlerOptions
var handler *PrettyHandler
var logger *slog.Logger

func main() {
	startTime = time.Now()
	opts = PrettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}
	handler = NewPrettyHandler(os.Stdout, opts)
	logger = slog.New(handler)

	err := godotenv.Load()

	if err != nil {
		logger.Error("Error loading .env file", "error", err)
		os.Exit(1)
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
					logger.Info("Event ignored", "event", evt)
					continue
				}
				logger.Info("Event received", "event", eventsAPIEvent)

				client.Ack(*evt.Request)

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						start := time.Now()
						message := ev.Text
						message = strings.Split(message, ">")[1]
						logger.Info(message)
						message = strings.TrimSpace(message)
						r := ParseMessage(message, network)
						switch res := r.(type) {
						case slack.FileUploadParameters:
							logger.Info("Uploading File")
							_, err := slack_api.UploadFile(res)
							if err != nil {
								logger.Error("Error while uploading file", "error", err)
							}
							elapsed := time.Since(start).String()
							logger.Info("Time Elapsed", "time", elapsed)
						case string:
							if res != "" {
								_, _, err := slack_api.PostMessage(ev.Channel, slack.MsgOptionText(res, false))
								if err != nil {
									logger.Error("Failed posting message", "error", err)
								}
							} else {
								res = "Who? :extremelaughingemoji:"
								_, _, err := slack_api.PostMessage(ev.Channel, slack.MsgOptionText(res, false))
								if err != nil {
									logger.Error("Failed posting message", "error", err)
								}
							}
							elapsed := time.Since(start).String()
							logger.Info("Time Elapsed", "time", elapsed)
						}

					default:
						logger.Info("Default Event", "event", ev)
					}

				default:
					client.Debugf("Unsupported Events API event received")
				}
			case socketmode.EventTypeConnecting:
				logger.Info("Connecting...")
			case socketmode.EventTypeConnected:
				logger.Info("Connected.")
			case socketmode.EventTypeHello:
				logger.Info("Received hello event from Slack API")
			default:
				logger.Info("Unexpected event type received", "eventType", evt.Type)
			}
		}
	}()

	client.Run()
}

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
	return res
}

func GetTrackScrobbles(artistName, trackName string, network *lastfm.Api) string {
	logger.Debug("GetTrackScrobbles", "artistName", artistName, "trackName", trackName)
	var res string
	counts := make(map[string]int)
	trackName = strings.Replace(trackName, "&amp;", "\u0026", 1)
	for _, user := range group {
		result, err := network.Track.GetInfo(lastfm.P{"artist": artistName, "track": trackName, "username": user})
		if err != nil {
			logger.Error("GetTrackScrobbles error", "error", err)
			return fmt.Sprintf("%s", err)
		}
		if result.UserPlayCount == "" {
			counts[user] = 0
		} else {
			counts[user], err = strconv.Atoi(result.UserPlayCount)
			if err != nil {
				logger.Error("GetTrackScrobbles error", "error", err)
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
	return res
}

func GetTopTracks(username string, period string, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's top tracks for the past ", username)
	switch period {
	case "7d", "1w":
		period = "7day"
		res += "7 days:\n\n"
	case "1m", "30d":
		period = "1month"
		res += "1 month:\n\n"
	case "3m", "90d":
		period = "3month"
		res += "3 months:\n\n"
	case "6m", "180d":
		period = "6month"
		res += "6 months:\n\n"
	case "1y", "365d":
		period = "12month"
		res += "year:\n\n"
	default:
		period = "overall"
		res = strings.TrimSuffix(res, "for the past ")
		res += "of all time:\n\n"
	}

	result, err := network.User.GetTopTracks(lastfm.P{"user": username, "period": period, "limit": 10})
	if err != nil {
		logger.Error("GetTopTracks error", "error", err)
	}
	logger.Debug("GetTopTracks", "user", username, "period", period, "limit", 10)

	for i, album := range result.Tracks {
		albumName := album.Name
		artistName := album.Artist.Name
		res += fmt.Sprintf("%d. %s - %s\n", i+1, artistName, albumName)
	}
	return res
}

func GetAlbumScrobbles(artistName, albumName string, network *lastfm.Api) string {
	albumName = strings.Replace(albumName, "&amp;", "\u0026", 1)
	artistName = strings.Replace(artistName, "&amp;", "\u0026", 1)

	logger.Debug("GetAlbumScrobbles", "artistName", artistName, "albumName", albumName)

	var res string
	counts := make(map[string]int)

	for _, user := range group {
		result, err := network.Album.GetInfo(lastfm.P{"artist": artistName, "album": albumName, "username": user})
		if err != nil {
			logger.Error("GetAlbumScrobbles error", "error", err, "user", user)
			continue
		}
		if result.UserPlayCount == "" {
			counts[user] = 0
		} else {
			counts[user], err = strconv.Atoi(result.UserPlayCount)
			if err != nil {
				logger.Error("GetAlbumScrobbles error", "error", err)
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
	return res
}

func GetRecentTracks(username string, limit int, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's last %d played songs:\n\n", username, limit)
	result, err := network.User.GetRecentTracks(lastfm.P{"user": username, "limit": limit})
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

func GetNowPlaying(network *lastfm.Api) string {
	res := "What everyone is listening to right now:\n\n"
	for _, user := range group {
		result, err := network.User.GetRecentTracks(lastfm.P{"user": user, "limit": 1})
		if err != nil {
			logger.Error("GetNowPlaying error", "error", err, "user", user)
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

func GetTopAlbumsAll(period string, network *lastfm.Api) string {
	type album struct {
		AlbumName string
		Artist    string
	}

	m := make(map[album]int)

	res := "Kagang's top albums for the past "
	switch period {
	case "7d", "1w":
		period = "7day"
		res += "7 days:\n\n"
	case "1m", "30d":
		period = "1month"
		res += "1 month:\n\n"
	case "3m", "90d":
		period = "3month"
		res += "3 months:\n\n"
	case "6m", "180d":
		period = "6month"
		res += "6 months:\n\n"
	case "1y", "365d":
		period = "12month"
		res += "year:\n\n"
	default:
		period = "overall"
		res = strings.TrimSuffix(res, "for the past ")
		res += "of all time:\n\n"
	}

	for _, user := range group {
		result, err := network.User.GetTopAlbums(lastfm.P{"user": user, "period": period, "limit": 500})
		if err != nil {
			logger.Error("GetTopAlbums error", "error", err)
		}
		for _, alb := range result.Albums {
			count, err := strconv.Atoi(alb.PlayCount)
			if err != nil {
				logger.Error("strconv error", "error", err)
			}
			albm := album{alb.Name, alb.Artist.Name}
			if _, ok := m[albm]; ok {
				m[albm] += count
			} else {
				m[albm] = count
			}
		}
	}
	keys := make([]album, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })

	for i, album := range keys {
		res += fmt.Sprintf("%d. %s - %s -- %d scrobbles\n", i+1, album.Artist, album.AlbumName, m[album])
		if i >= 9 {
			break
		}
	}
	return res
}

func GetTopTracksAll(period string, network *lastfm.Api) string {
	type Track struct {
		TrackName string
		Artist    string
	}

	m := make(map[Track]int)

	res := "Kagang's top tracks for the past "
	switch period {
	case "7d", "1w":
		period = "7day"
		res += "7 days:\n\n"
	case "1m", "30d":
		period = "1month"
		res += "1 month:\n\n"
	case "3m", "90d":
		period = "3month"
		res += "3 months:\n\n"
	case "6m", "180d":
		period = "6month"
		res += "6 months:\n\n"
	case "1y", "365d":
		period = "12month"
		res += "year:\n\n"
	default:
		period = "overall"
		res = strings.TrimSuffix(res, "for the past ")
		res += "of all time:\n\n"
	}

	for _, user := range group {
		result, err := network.User.GetTopTracks(lastfm.P{"user": user, "period": period, "limit": 500})
		if err != nil {
			logger.Error("GetTopTracks error", "error", err)
		}
		for _, track := range result.Tracks {
			count, err := strconv.Atoi(track.PlayCount)
			if err != nil {
				logger.Error("strconv error", "error", err)
			}
			tr := Track{track.Name, track.Artist.Name}
			if _, ok := m[tr]; ok {
				m[tr] += count
			} else {
				m[tr] = count
			}
		}
	}
	keys := make([]Track, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })

	for i, track := range keys {
		res += fmt.Sprintf("%d. %s - %s -- %d scrobbles\n", i+1, track.Artist, track.TrackName, m[track])
		if i >= 9 {
			break
		}
	}
	return res
}

func GetTopAlbums(username, period string, network *lastfm.Api) string {
	res := fmt.Sprintf("%s's top albums for the past ", username)
	switch period {
	case "7d", "1w":
		period = "7day"
		res += "7 days:\n\n"
	case "1m", "30d":
		period = "1month"
		res += "1 month:\n\n"
	case "3m", "90d":
		period = "3month"
		res += "3 months:\n\n"
	case "6m", "180d":
		period = "6month"
		res += "6 months:\n\n"
	case "1y", "365d":
		period = "12month"
		res += "year:\n\n"
	default:
		period = "overall"
		res = strings.TrimSuffix(res, "for the past ")
		res += "of all time:\n\n"
	}

	result, err := network.User.GetTopAlbums(lastfm.P{"user": username, "period": period, "limit": 10})
	if err != nil {
		logger.Error("GetTopAlbums error", "error", err)
	}
	logger.Debug("GetTopAlbums", "user", username, "period", period, "limit", 10)

	for i, album := range result.Albums {
		albumName := album.Name
		artistName := album.Artist.Name
		res += fmt.Sprintf("%d. %s - %s\n", i+1, artistName, albumName)
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

	result, err := network.User.GetTopArtists(lastfm.P{"user": username, "period": period, "limit": 10})
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

func ChatGPT(prompt string) string {
	outputFile, err := os.OpenFile("output.txt", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		logger.Error(err.Error())
	}
	defer outputFile.Close()
	cmd := exec.Command("./main", "-t", "2", "-ngl", "32", "-m", "models/codellama.gguf", "-c", "4096", "--repeat_penalty", "1.1", "-n", "500", "-p", prompt)
	cmd.Stdout = outputFile
	err = cmd.Start()
	if err != nil {
		logger.Error("Failed to start command", "command", cmd.Path, "arguments", cmd.Args, "cmdError", cmd.Err, "err", err)
	}
	err = cmd.Wait()
	if err != nil {
		logger.Error("Failed to run command", "command", cmd.Path, "arguments", cmd.Args, "cmdError", cmd.Err, "err", err)
	}

	content, err := os.ReadFile("output.txt")
	if err != nil {
		logger.Error(err.Error())
	}
	return string(content)
}

func GenerateCollage(username string) slack.FileUploadParameters {
	cmd := exec.Command("collage.py", "-u", username)
	err := cmd.Run()
	if err != nil {
		logger.Error("Failed to run command", "command", cmd.Path, "arguments", cmd.Args, "error", err)
	}
	params := slack.FileUploadParameters{
		Title:    "collage.png",
		File:     "collage.png",
		Channels: []string{"chuu-stats"},
	}
	return params
}

func ParseMessage(message string, network *lastfm.Api) any {
	if message == "" {
		return ""
	}

	if strings.HasPrefix(message, "!help") {
		helpStr := "Commands:\n\n" +
			"!np: Now Playing\n" +
			"!disco <user> <artist>: Top albums by <artist> for <user>\n" +
			"!track <user> <period>: Top tracks for <user> in <period>\n" +
			"!dt <user> <artist>: Top tracks by <artist> for <user>\n" +
			"!top <user> <period>: Top albums for <user> in <period>\n" +
			"!ta <user> <period>: Top artists for <user> in <period>\n" +
			"!rp <user> <limit>: Last <limit> songs played by <user>\n" +
			"!kga <period>: Top listened albums in Kagang in <period>\n" +
			"!kgt <period>: Top listened tracks in Kagang in <period>\n" +
			"!up: Uptime"
		return helpStr
	}

	if strings.HasPrefix(message, "!up") {
		uptime := time.Since(startTime)
		return uptime.String()
	}

	if strings.HasPrefix(message, "!chart") {
		message = strings.TrimPrefix(message, "!chart")
		message = strings.TrimSpace(message)
		return GenerateCollage(message)
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

	if strings.HasPrefix(message, "!dt") {
		message = strings.TrimPrefix(message, "!dt")
		message = strings.TrimSpace(message)
		msg := strings.SplitN(message, " ", 2)
		if len(msg) == 2 {
			user := msg[0]
			artist := msg[1]
			return GetTopTracksForArtist(artist, user, network)
		} else {
			return ""
		}
	}

	if strings.HasPrefix(message, "!topartist") || strings.HasPrefix(message, "!ta") {
		if strings.HasPrefix(message, "!topartist") {
			message = strings.TrimPrefix(message, "!topartist")
		} else {
			message = strings.TrimPrefix(message, "!ta")
		}

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

	if strings.HasPrefix(message, "!track") {
		message = strings.TrimPrefix(message, "!track")
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
		return GetTopTracks(user, period, network)
	}

	if strings.HasPrefix(message, "!artist") {
		message = strings.TrimPrefix(message, "!artist")
		message = strings.TrimSpace(message)
		artistName := message
		return GetArtistScrobbles(artistName, network)
	}

	if strings.HasPrefix(message, "!kga") {
		message = strings.TrimPrefix(message, "!kga")
		message = strings.TrimSpace(message)
		period := ""
		if message != "" {
			period = message
		}
		return GetTopAlbumsAll(period, network)
	}

	if strings.HasPrefix(message, "!kgt") {
		message = strings.TrimPrefix(message, "!kgt")
		message = strings.TrimSpace(message)
		period := ""
		if message != "" {
			period = message
		}
		return GetTopTracksAll(period, network)
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

	if strings.Contains(message, " by ") || strings.Contains(message, " By ") {
		if strings.HasPrefix(message, "!t") {
			message = strings.TrimSpace(strings.Split(message, "!t")[1])
			var msg []string
			if strings.Contains(message, " by ") {
				msg = strings.Split(message, " by ")
			} else {
				msg = strings.Split(message, " By ")
			}
			trackName := msg[0]
			artistName := msg[1]

			return GetTrackScrobbles(artistName, trackName, network)
		} else {
			var msg []string
			if strings.Contains(message, " by ") {
				msg = strings.Split(message, " by ")
			} else {
				msg = strings.Split(message, " By ")
			}
			albumName := msg[0]
			artistName := msg[1]

			return GetAlbumScrobbles(artistName, albumName, network)
		}
	} else {
		artistName := message
		return GetArtistScrobbles(artistName, network)
	}

}
