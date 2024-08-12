package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/fogleman/gg"
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
	if r.NumAttrs() > 0 {
		h.l.Println(timeStr, level, msg, color.WhiteString(string(b)))
	} else {
		h.l.Println(timeStr, level, msg)
	}

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

var group = [21]string{
	"Codeine_turtle", "odesmut", "dudeactually",
	"z47Breezo", "itsalmostdry",
	"v0__", "Hirammj", "FrozenWaterz", "Silkmoney",
	"Mo98t", "BTGKM9_Redd", "colbster411", "FaRiddim", "Vadermaulkylo",
	"Schwarrtz", "Xutros", "Billy-Shakes", "maloboosie", "icy_twat", "junkiesRpeople", "rumnitty",
}

var startTime time.Time

var (
	opts    PrettyHandlerOptions
	handler *PrettyHandler
	logger  *slog.Logger
)

func main() {
	startTime = time.Now()
	opts = PrettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: slog.LevelInfo,
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
						logger.Info("Message Received", "message", message)
						message = strings.Split(message, ">")[1]
						message = strings.TrimSpace(message)
						logger.Debug("Message after parsing", "message", message)
						r := ParseMessage(message, network)
						logger.Debug("ParseMessage", "result", r)
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

func callWebServerAPI(username, period string) ([]byte, error) {
	url := fmt.Sprintf("http://topster.gg/api/topalbums/%s/%s", username, period)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func createAlbumChart(albums []struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Image  string `json:"image"`
}) (*gg.Context, error) {
	const (
		width  = 600
		height = 600
		rows   = 3
		cols   = 3
	)

	dc := gg.NewContext(width, height)

	for i, album := range albums[:9] { // Limit to 9 albums
		x := float64(i%cols) * (float64(width) / float64(cols))
		y := float64(i/cols) * (float64(height) / float64(rows))

		// Download album art
		resp, err := http.Get(album.Image)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		img, _, err := image.Decode(resp.Body)
		if err != nil {
			return nil, err
		}

		// Draw album art
		dc.DrawImage(img, int(x), int(y))
	}

	return dc, nil
}

func GenerateAlbumChart(username string, network *lastfm.Api) slack.FileUploadParameters {
	// Call the web server API
	chartData, err := callWebServerAPI(username, "7day")
	if err != nil {
		logger.Error("Web server API call error", "error", err)
		return slack.FileUploadParameters{}
	}

	// Parse the JSON response
	var albums []struct {
		Name   string `json:"name"`
		Artist string `json:"artist"`
		Image  string `json:"image"`
	}
	err = json.Unmarshal(chartData, &albums)
	if err != nil {
		logger.Error("JSON unmarshal error", "error", err)
		return slack.FileUploadParameters{}
	}

	// Generate chart
	chartContext, err := createAlbumChart(albums)
	if err != nil {
		logger.Error("Create album chart error", "error", err)
		return slack.FileUploadParameters{}
	}

	// Get the image from the context
	chartImage := chartContext.Image()

	// Save chart as image
	filename := fmt.Sprintf("%s_album_chart.png", username)
	file, err := os.Create(filename)
	if err != nil {
		logger.Error("Error creating file", "error", err)
		return slack.FileUploadParameters{}
	}
	defer file.Close()

	err = png.Encode(file, chartImage)
	if err != nil {
		logger.Error("PNG encoding error", "error", err)
		return slack.FileUploadParameters{}
	}

	params := slack.FileUploadParameters{
		File:     filename,
		Filename: filename,
		Channels: []string{"C0392543PUY"},
		Title:    fmt.Sprintf("Album chart for %s", username),
	}

	return params
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

func GetWeeklyLeaderboard(network *lastfm.Api) string {
	var fromTime, toTime time.Time
	counts := make(map[string]int)

	// Calculate from and to times for the past 7 days
	toTime = time.Now()                 // current time
	fromTime = toTime.AddDate(0, 0, -7) // subtract 7 days from today

	for _, user := range group {
		// Format times as Unix timestamps for Last.fm API parameters
		fromTimestamp := strconv.FormatInt(fromTime.Unix(), 10)
		toTimestamp := strconv.FormatInt(toTime.Unix(), 10)

		artistChart, err := network.User.GetWeeklyArtistChart(lastfm.P{"user": user, "from": fromTimestamp, "to": toTimestamp})
		if err != nil {
			logger.Error("GetWeeklyArtistChart error", "error", err)
			continue
		}

		totalPlayCount := 0
		for _, artist := range artistChart.Artists {
			playcount, err := strconv.Atoi(artist.PlayCount)
			if err != nil {
				logger.Error("strconv artist.Playcount", "error", err)
				continue
			}
			totalPlayCount += playcount
		}
		counts[user] = totalPlayCount
	}

	var usercounts []UserCount
	for user, count := range counts {
		usercounts = append(usercounts, UserCount{Username: user, Playcount: count})
	}
	sort.Slice(usercounts, func(i, j int) bool {
		return usercounts[i].Playcount > usercounts[j].Playcount
	})

	fromDate := fromTime.Format("2006/01/02")
	toDate := toTime.Format("2006/01/02")
	res := fmt.Sprintf("Weekly Leaderboard for past 7 days: %s to %s\n\n", fromDate, toDate)
	for i, usercount := range usercounts {
		ranking := fmt.Sprintf("%d. %s: %d scrobbles\n", i+1, usercount.Username, usercount.Playcount)
		switch i {
		case 0:
			res += "👑 " + ranking
		case 1:
			res += "🥈 " + ranking
		case 2:
			res += "🥉 " + ranking
		default:
			res += ranking
		}
	}

	return res
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
			"!leaderboard: Leaderboard for previous week\n" +
			"!up: Uptime"
		return helpStr
	}

	if strings.HasPrefix(message, "!up") {
		uptime := time.Since(startTime)
		return uptime.String()
	}

	if strings.HasPrefix(message, "!chart") {
		message = strings.TrimPrefix(message, "!chart")
		username := strings.TrimSpace(message)
		return GenerateAlbumChart(username, network)
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

	if strings.HasPrefix(message, "!leaderboard") {
		return GetWeeklyLeaderboard(network)
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
