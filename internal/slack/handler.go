package slack

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/syakter/go-chuu/internal/charts"
	"github.com/syakter/go-chuu/internal/commands"
	"github.com/syakter/go-chuu/internal/config"
	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/lastfm"
	"github.com/syakter/go-chuu/internal/ollama"
	"github.com/syakter/go-chuu/internal/profile"
	"github.com/syakter/go-chuu/internal/types"
)

// Handler manages Slack interactions
type Handler struct {
	api          *slack.Client
	client       *socketmode.Client
	lastfmClient *lastfm.Client
	chartGen     *charts.Generator
	parser       *commands.Parser
	config       *config.Config
	logger       *slog.Logger
	ollamaClient *ollama.Client // nil when Ollama is not configured
	startTime    time.Time
}

// NewHandler creates a new Slack handler
func NewHandler(
	cfg *config.Config,
	lastfmClient *lastfm.Client,
	chartGen *charts.Generator,
	parser *commands.Parser,
	logger *slog.Logger,
	ollamaClient *ollama.Client,
) *Handler {
	api := slack.New(
		cfg.SlackBotToken,
		slack.OptionAppLevelToken(cfg.SlackAppToken),
	)

	client := socketmode.New(api)

	return &Handler{
		api:          api,
		client:       client,
		lastfmClient: lastfmClient,
		chartGen:     chartGen,
		parser:       parser,
		config:       cfg,
		logger:       logger,
		ollamaClient: ollamaClient,
		startTime:    time.Now(),
	}
}

// Start begins handling Slack events
func (h *Handler) Start(ctx context.Context) error {
	h.logger.Info("Starting Slack handler")

	h.runRecapScheduler(ctx)
	go h.handleEvents(ctx)

	return h.client.RunContext(ctx)
}

// handleEvents processes incoming Slack events
func (h *Handler) handleEvents(ctx context.Context) {
	for {
		select {
		case evt := <-h.client.Events:
			h.processEvent(ctx, evt)
		case <-ctx.Done():
			h.logger.Info("Event handler stopping")
			return
		}
	}
}

// processEvent processes individual Slack events
func (h *Handler) processEvent(ctx context.Context, evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeEventsAPI:
		h.handleEventsAPI(ctx, evt)
	case socketmode.EventTypeConnecting:
		h.logger.Info("Connecting to Slack...")
	case socketmode.EventTypeConnected:
		h.logger.Info("Connected to Slack")
	case socketmode.EventTypeHello:
		h.logger.Debug("Received hello event from Slack")
	default:
		h.logger.Debug("Unexpected event type", "type", evt.Type)
	}
}

// handleEventsAPI handles Events API events
func (h *Handler) handleEventsAPI(ctx context.Context, evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		h.logger.Warn("Invalid Events API event", "event", evt)
		return
	}

	h.logger.Debug("Received Events API event", "type", eventsAPIEvent.Type)

	// Acknowledge the event
	h.client.Ack(*evt.Request)

	switch eventsAPIEvent.Type {
	case slackevents.CallbackEvent:
		h.handleCallbackEvent(ctx, eventsAPIEvent.InnerEvent)
	default:
		h.logger.Debug("Unsupported Events API event type", "type", eventsAPIEvent.Type)
	}
}

// handleCallbackEvent handles callback events (mentions, messages)
func (h *Handler) handleCallbackEvent(ctx context.Context, innerEvent slackevents.EventsAPIInnerEvent) {
	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		h.handleAppMention(ctx, ev)
	default:
		h.logger.Debug("Unsupported callback event", "type", fmt.Sprintf("%T", ev))
	}
}

// handleAppMention processes app mention events
func (h *Handler) handleAppMention(ctx context.Context, event *slackevents.AppMentionEvent) {
	start := time.Now()
	h.logger.Info("Processing app mention", "user", event.User, "channel", event.Channel, "text", event.Text)

	// Create a timeout context for the command processing
	cmdCtx, cancel := context.WithTimeout(ctx, h.config.RequestTimeout)
	defer cancel()

	// Parse the command
	cmd, err := h.parser.Parse(event.Text)
	if err != nil {
		h.logger.Warn("Failed to parse command", "error", err, "text", event.Text)
		h.sendErrorResponse(event.Channel, err)
		return
	}

	// Process the command
	response := h.processCommand(cmdCtx, cmd)

	// Send the response
	h.sendResponse(cmdCtx, event.Channel, response)

	elapsed := time.Since(start)
	h.logger.Info("Completed app mention processing", "duration", elapsed)
}

// processCommand processes a parsed command and returns a response
func (h *Handler) processCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	h.logger.Debug("Processing command", "type", cmd.Type, "user", cmd.User)

	switch cmd.Type {
	case types.CommandHelp:
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: commands.GetHelpText(),
		}

	case types.CommandUptime:
		uptime := time.Since(h.startTime)
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("Uptime: %s", uptime.String()),
		}

	case types.CommandChart:
		return h.handleChartCommand(ctx, cmd)

	case types.CommandNowPlaying:
		return h.handleNowPlayingCommand(ctx)

	case types.CommandArtistFans:
		return h.handleArtistFansCommand(ctx, cmd)

	case types.CommandAlbumFans:
		return h.handleAlbumFansCommand(ctx, cmd)

	case types.CommandTrackFans:
		return h.handleTrackFansCommand(ctx, cmd)

	case types.CommandLeaderboard:
		return h.handleLeaderboardCommand(ctx)

	case types.CommandTopTracks:
		return h.handleTopTracksCommand(ctx, cmd)

	case types.CommandTopAlbums:
		return h.handleTopAlbumsCommand(ctx, cmd)

	case types.CommandTopArtists:
		return h.handleTopArtistsCommand(ctx, cmd)

	case types.CommandRecentTracks:
		return h.handleRecentTracksCommand(ctx, cmd)

	case types.CommandTopAlbumsAll:
		return h.handleTopAlbumsAllCommand(ctx, cmd)

	case types.CommandTopTracksAll:
		return h.handleTopTracksAllCommand(ctx, cmd)

	case types.CommandDisco:
		return h.handleDiscoCommand(ctx, cmd)

	case types.CommandDiscoveryTrack:
		return h.handleDiscoveryTrackCommand(ctx, cmd)

	case types.CommandRecommend:
		return h.handleRecommendCommand(ctx, cmd)

	case types.CommandAIRec:
		return h.handleAIRecCommand(ctx, cmd)

	case types.CommandHiddenGem:
		return h.handleHiddenGemCommand(ctx, cmd)

	case types.CommandAffinity:
		return h.handleAffinityCommand(ctx, cmd)

	case types.CommandProfile:
		return h.handleProfileCommand(ctx, cmd)

	case types.CommandTopGenres:
		return h.handleTopGenresCommand(ctx, cmd)

	case types.CommandVibe:
		return h.handleVibeCommand(ctx, cmd)

	case types.CommandRecap:
		return h.handleRecapCommand(ctx, cmd)

	default:
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "Command not implemented yet",
		}
	}
}

// handleChartCommand processes chart generation commands
func (h *Handler) handleChartCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	fileUpload, err := h.chartGen.GenerateAlbumChart(ctx, cmd.User, cmd.Period, cmd.ChartSize)
	if err != nil {
		h.logger.Error("Failed to generate chart", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	return &types.BotResponse{
		Type: types.ResponseTypeFile,
		File: fileUpload,
	}
}

// handleNowPlayingCommand processes now playing commands
func (h *Handler) handleNowPlayingCommand(ctx context.Context) *types.BotResponse {
	nowPlaying, err := h.lastfmClient.GetNowPlaying(ctx)
	if err != nil {
		h.logger.Error("Failed to get now playing", "error", err)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(nowPlaying) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: "Nobody is listening to anything right now! 🎵",
		}
	}

	var content strings.Builder
	content.WriteString("What everyone is listening to right now:\n\n")

	for user, track := range nowPlaying {
		content.WriteString(fmt.Sprintf("%s is listening to %s\n", user, track))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleArtistFansCommand processes artist fans commands
func (h *Handler) handleArtistFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := h.lastfmClient.GetArtistScrobbles(ctx, cmd.Artist)
	if err != nil {
		h.logger.Error("Failed to get artist scrobbles", "error", err, "artist", cmd.Artist)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := h.formatUserCounts(fmt.Sprintf("Top %s fans in Kagang:", cmd.Artist), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleAlbumFansCommand processes album fans commands
func (h *Handler) handleAlbumFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := h.lastfmClient.GetAlbumScrobbles(ctx, cmd.Artist, cmd.Album)
	if err != nil {
		h.logger.Error("Failed to get album scrobbles", "error", err, "artist", cmd.Artist, "album", cmd.Album)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := h.formatUserCounts(fmt.Sprintf("Top %s - %s fans in Kagang:", cmd.Artist, cmd.Album), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleTrackFansCommand processes track fans commands
func (h *Handler) handleTrackFansCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	userCounts, err := h.lastfmClient.GetTrackScrobbles(ctx, cmd.Artist, cmd.Track)
	if err != nil {
		h.logger.Error("Failed to get track scrobbles", "error", err, "artist", cmd.Artist, "track", cmd.Track)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	content := h.formatUserCounts(fmt.Sprintf("Top %s - %s fans in Kagang:", cmd.Artist, cmd.Track), userCounts)

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// handleLeaderboardCommand processes leaderboard commands
func (h *Handler) handleLeaderboardCommand(ctx context.Context) *types.BotResponse {
	leaderboard, err := h.lastfmClient.GetWeeklyLeaderboard(ctx)
	if err != nil {
		h.logger.Error("Failed to get weekly leaderboard", "error", err)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	if len(leaderboard) > 0 {
		fromDate := leaderboard[0].PeriodFrom.Format("2006/01/02")
		toDate := leaderboard[0].PeriodTo.Format("2006/01/02")
		content.WriteString(fmt.Sprintf("Weekly Leaderboard (%s to %s):\n\n", fromDate, toDate))
	} else {
		content.WriteString("Weekly Leaderboard:\n\n")
	}

	for _, entry := range leaderboard {
		var emoji string
		switch entry.Rank {
		case 1:
			emoji = "👑"
		case 2:
			emoji = "🥈"
		case 3:
			emoji = "🥉"
		default:
			emoji = fmt.Sprintf("%d.", entry.Rank)
		}

		content.WriteString(fmt.Sprintf("%s %s: %d scrobbles\n", emoji, entry.Username, entry.Scrobbles))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleTopTracksCommand processes top tracks commands
func (h *Handler) handleTopTracksCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	tracks, err := h.lastfmClient.GetUserTopTracks(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get user top tracks", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(cmd.Period)
	content.WriteString(fmt.Sprintf("%s's top tracks%s:\n\n", cmd.User, periodText))

	for i, track := range tracks {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, track))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// formatUserCounts formats user counts into a readable string
func (h *Handler) formatUserCounts(title string, userCounts []types.UserCount) string {
	var content strings.Builder
	content.WriteString(title + "\n\n")

	for i, userCount := range userCounts {
		var prefix string
		switch i {
		case 0:
			prefix = "👑."
		case 1:
			prefix = "🥈."
		case 2:
			prefix = "🥉."
		default:
			prefix = fmt.Sprintf("%d.", i+1)
		}

		content.WriteString(fmt.Sprintf("%s %s: %d scrobbles\n", prefix, userCount.Username, userCount.Playcount))
	}

	return content.String()
}

// formatPeriodText formats period into readable text
func (h *Handler) formatPeriodText(period string) string {
	if period == "" || period == "overall" {
		return " of all time"
	}

	switch period {
	case "7d", "1w":
		return " for the past 7 days"
	case "1m", "30d":
		return " for the past month"
	case "3m", "90d":
		return " for the past 3 months"
	case "6m", "180d":
		return " for the past 6 months"
	case "1y", "365d":
		return " for the past year"
	default:
		return fmt.Sprintf(" for period: %s", period)
	}
}

// sendResponse sends a bot response to the specified channel
func (h *Handler) sendResponse(ctx context.Context, channel string, response *types.BotResponse) {
	switch response.Type {
	case types.ResponseTypeText:
		if response.Content != "" {
			if _, _, err := h.api.PostMessage(channel, slack.MsgOptionText(response.Content, false)); err != nil {
				h.logger.Error("Failed to send text message", "error", err)
			}
		}

	case types.ResponseTypeFile:
		if response.File != nil {
			fileInfo, err := os.Stat(response.File.Path)
			if err != nil {
				h.logger.Error("Failed to stat chart file", "error", err)
				break
			}
			params := slack.UploadFileV2Parameters{
				File:     response.File.Path,
				FileSize: int(fileInfo.Size()),
				Filename: response.File.Filename,
				Channel:  channel,
				Title:    response.File.Title,
			}

			uploadCtx, uploadCancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer uploadCancel()
			if _, err := h.api.UploadFileV2Context(uploadCtx, params); err != nil {
				h.logger.Error("Failed to upload file", "error", err)
			}
		}

	case types.ResponseTypeError:
		if _, _, err := h.api.PostMessage(channel, slack.MsgOptionText(response.Error, false)); err != nil {
			h.logger.Error("Failed to send error message", "error", err)
		}
	}
}

// buildRecPrompt asks the model to recommend artists based on a user's listening taste.
func buildRecPrompt(username string, topArtists []string, topGenres []types.GenreCount, periodText string) string {
	var sb strings.Builder
	sb.WriteString("You are a music recommendation expert. Recommend 10 artists this listener would enjoy that are NOT already in their listening history.\n\n")
	sb.WriteString("Return a numbered list of exactly 10 artists, one per line, in this exact format:\n")
	sb.WriteString("1. Artist Name — one sentence (max 15 words) explaining the connection to their taste\n\n")
	sb.WriteString("Only return the numbered list. No preamble, no summary.\n\n")

	sb.WriteString(fmt.Sprintf("Listener: %s\n", username))
	if periodText != "" {
		sb.WriteString(fmt.Sprintf("Period: %s\n", strings.TrimPrefix(strings.TrimSpace(periodText), "for ")))
	}
	if len(topArtists) > 0 {
		sb.WriteString(fmt.Sprintf("Their top artists: %s\n", strings.Join(topArtists, ", ")))
	}
	if len(topGenres) > 0 {
		genreNames := make([]string, len(topGenres))
		for i, g := range topGenres {
			genreNames[i] = g.Name
		}
		sb.WriteString(fmt.Sprintf("Their top genres: %s\n", strings.Join(genreNames, ", ")))
	}

	return sb.String()
}

// buildVibePrompt constructs the Ollama prompt for a user's taste profile.
func buildVibePrompt(username, periodText string, genres []types.GenreCount, artists []string) string {
	var genreNames []string
	for _, g := range genres {
		genreNames = append(genreNames, g.Name)
	}

	var sb strings.Builder
	sb.WriteString("You are a witty music critic writing quick listener profiles for a group chat. ")
	sb.WriteString("Write a 2-3 sentence description of this person's music taste based on their listening data. ")
	sb.WriteString("Be specific — reference their actual artists and genres. Make it punchy and fun, like something you'd share with friends. ")
	sb.WriteString("Don't be generic or wishy-washy.\n\n")
	sb.WriteString(fmt.Sprintf("Listener: %s\n", username))
	if periodText != "" {
		sb.WriteString(fmt.Sprintf("Period: %s\n", strings.TrimPrefix(strings.TrimSpace(periodText), "for ")))
	}
	if len(genreNames) > 0 {
		sb.WriteString(fmt.Sprintf("Top genres: %s\n", strings.Join(genreNames, ", ")))
	}
	if len(artists) > 0 {
		sb.WriteString(fmt.Sprintf("Top artists: %s\n", strings.Join(artists, ", ")))
	}

	return sb.String()
}

// sendErrorResponse sends an error response to the user
func (h *Handler) sendErrorResponse(channel string, err error) {
	errorMessage := errors.GetUserFriendlyMessage(err)
	if _, _, sendErr := h.api.PostMessage(channel, slack.MsgOptionText(errorMessage, false)); sendErr != nil {
		h.logger.Error("Failed to send error message", "error", sendErr)
	}
}

// handleTopAlbumsCommand processes top albums commands
func (h *Handler) handleTopAlbumsCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	albums, err := h.lastfmClient.GetUserTopAlbums(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get user top albums", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(cmd.Period)
	content.WriteString(fmt.Sprintf("%s's top albums%s:\n\n", cmd.User, periodText))

	for i, album := range albums {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, album))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleTopArtistsCommand processes top artists commands
func (h *Handler) handleTopArtistsCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	artists, err := h.lastfmClient.GetUserTopArtists(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get user top artists", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(cmd.Period)
	content.WriteString(fmt.Sprintf("%s's top artists%s:\n\n", cmd.User, periodText))

	for i, artist := range artists {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, artist))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleRecentTracksCommand processes recent tracks commands
func (h *Handler) handleRecentTracksCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	tracks, err := h.lastfmClient.GetUserRecentTracks(ctx, cmd.User, cmd.Limit)
	if err != nil {
		h.logger.Error("Failed to get user recent tracks", "error", err, "user", cmd.User, "limit", cmd.Limit)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("%s's recent tracks:\n\n", cmd.User))

	for i, track := range tracks {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, track))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleTopAlbumsAllCommand processes top albums for all users commands
func (h *Handler) handleTopAlbumsAllCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	period := cmd.Period
	if period == "" {
		period = "7d" // default to 7 days
	}

	albums, err := h.lastfmClient.GetTopAlbumsAcrossUsers(ctx, period, 10)
	if err != nil {
		h.logger.Error("Failed to get top albums across users", "error", err, "period", period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(albums) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: "No albums found for the specified period! 🎵",
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(period)
	content.WriteString(fmt.Sprintf("Top albums in Kagang%s:\n\n", periodText))

	for i, album := range albums {
		content.WriteString(fmt.Sprintf("%d. %s (%d scrobbles, %d users)\n", i+1, album.AlbumName, album.Playcount, album.UserCount))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleTopTracksAllCommand processes top tracks for all users commands
func (h *Handler) handleTopTracksAllCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	period := cmd.Period
	if period == "" {
		period = "7d" // default to 7 days
	}

	tracks, err := h.lastfmClient.GetTopTracksAcrossUsers(ctx, period, 10)
	if err != nil {
		h.logger.Error("Failed to get top tracks across users", "error", err, "period", period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(tracks) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: "No tracks found for the specified period! 🎵",
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(period)
	content.WriteString(fmt.Sprintf("Top tracks in Kagang%s:\n\n", periodText))

	for i, track := range tracks {
		content.WriteString(fmt.Sprintf("%d. %s (%d scrobbles, %d users)\n", i+1, track.TrackName, track.Playcount, track.UserCount))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleDiscoCommand processes disco commands
func (h *Handler) handleDiscoCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	albums, err := h.lastfmClient.GetUserTopAlbumsByArtist(ctx, cmd.User, cmd.Artist, 10)
	if err != nil {
		h.logger.Error("Failed to get user albums by artist", "error", err, "user", cmd.User, "artist", cmd.Artist)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(albums) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("No albums by %s found for %s! 🎵", cmd.Artist, cmd.User),
		}
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("%s's top albums by %s:\n\n", cmd.User, cmd.Artist))

	for i, album := range albums {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, album))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleRecommendCommand processes group recommendation commands
func (h *Handler) handleRecommendCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	period := cmd.Period
	if period == "" {
		period = "overall"
	}

	recommendations, err := h.lastfmClient.GetGroupRecommendations(ctx, cmd.User, period)
	if err != nil {
		h.logger.Error("Failed to get group recommendations", "error", err, "user", cmd.User, "period", period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(recommendations) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("No recommendations found for %s — they might already listen to everything the group loves!", cmd.User),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(period)
	content.WriteString(fmt.Sprintf("Artists the group loves that %s should check out%s:\n\n", cmd.User, periodText))

	for i, rec := range recommendations {
		if rec.UserPlaycount == 0 {
			content.WriteString(fmt.Sprintf("%d. %s — %d group scrobbles (0 plays by %s)\n", i+1, rec.Name, rec.GroupTotal, cmd.User))
		} else {
			content.WriteString(fmt.Sprintf("%d. %s — %d group scrobbles (%d plays by %s)\n", i+1, rec.Name, rec.GroupTotal, rec.UserPlaycount, cmd.User))
		}
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleAIRecCommand generates AI-driven artist recommendations based on the user's taste.
func (h *Handler) handleAIRecCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	if h.ollamaClient == nil {
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "AI features are not configured. Set OLLAMA_URL in the bot's environment to enable !airec.",
		}
	}

	period := cmd.Period
	if period == "" {
		period = "overall"
	}

	artists, err := h.lastfmClient.GetUserTopArtists(ctx, cmd.User, period, 10)
	if err != nil {
		h.logger.Error("Failed to get top artists for airec", "error", err, "user", cmd.User)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	genres, err := h.lastfmClient.GetUserTopGenres(ctx, cmd.User, period, 8)
	if err != nil {
		h.logger.Warn("Could not fetch genres for airec", "error", err)
	}

	prompt := buildRecPrompt(cmd.User, artists, genres, h.formatPeriodText(period))

	h.logger.Debug("Calling Ollama for airec", "user", cmd.User, "model", h.ollamaClient.Model())
	response, err := h.ollamaClient.Chat(ctx, []ollama.Message{{Role: "user", Content: prompt}})
	if err != nil {
		h.logger.Error("Ollama call failed for airec", "error", err)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "AI model is unavailable right now. Try again later.",
		}
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: fmt.Sprintf("AI recommendations for %s%s (via %s):\n\n%s", cmd.User, h.formatPeriodText(period), h.ollamaClient.Model(), response),
	}
}

// handleHiddenGemCommand processes hidden gem commands
func (h *Handler) handleHiddenGemCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	period := cmd.Period
	if period == "" {
		period = "overall"
	}

	gems, err := h.lastfmClient.GetHiddenGem(ctx, cmd.User, period)
	if err != nil {
		h.logger.Error("Failed to get hidden gems", "error", err, "user", cmd.User, "period", period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(gems) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("No hidden gems found for %s!", cmd.User),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(period)
	content.WriteString(fmt.Sprintf("%s's hidden gems%s:\n\n", cmd.User, periodText))

	for i, gem := range gems {
		var othersDesc string
		switch gem.OthersCount {
		case 0:
			othersDesc = "nobody else also listens"
		case 1:
			othersDesc = "1 other person also listens"
		default:
			othersDesc = fmt.Sprintf("%d others also listen", gem.OthersCount)
		}
		content.WriteString(fmt.Sprintf("%d. %s — %d plays (%s)\n", i+1, gem.Name, gem.UserPlaycount, othersDesc))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// affinityDetailLimit is how many of the top matches get their shared artists spelled out.
// Every user is listed — truncating a ranking hides the omission from the reader — but only the
// top few carry the extra detail line.
const affinityDetailLimit = 3

// handleAffinityCommand ranks the group by taste similarity to the given user
func (h *Handler) handleAffinityCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	period := cmd.Period
	if period == "" {
		period = "overall"
	}

	scores, err := h.lastfmClient.GetAffinity(ctx, cmd.User, period)
	if err != nil {
		h.logger.Error("Failed to get affinity", "error", err, "user", cmd.User, "period", period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(scores) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("Not enough listening data to compare %s against the group%s.", cmd.User, h.formatPeriodText(period)),
		}
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("Taste affinity for %s%s:\n\n", cmd.User, h.formatPeriodText(period)))

	for i, score := range scores {
		var prefix string
		switch i {
		case 0:
			prefix = "👑."
		case 1:
			prefix = "🥈."
		case 2:
			prefix = "🥉."
		default:
			prefix = fmt.Sprintf("%d.", i+1)
		}

		content.WriteString(fmt.Sprintf("%s %s — %.1f%% (%d shared artists)\n",
			prefix, score.Username, score.Score*100, score.SharedCount))

		if i < affinityDetailLimit && len(score.TopShared) > 0 {
			content.WriteString(fmt.Sprintf("     %s\n", strings.Join(score.TopShared, ", ")))
		}
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleProfileCommand processes profile card commands
func (h *Handler) handleProfileCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	data, err := profile.FetchData(ctx, h.lastfmClient.GetAPI(), cmd.User, cmd.Period)
	if err != nil {
		h.logger.Error("Failed to fetch profile data", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: profile.FormatMarkdown(data),
	}
}

// handleTopGenresCommand processes top genres commands
func (h *Handler) handleTopGenresCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	genres, err := h.lastfmClient.GetUserTopGenres(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get user top genres", "error", err, "user", cmd.User, "period", cmd.Period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(genres) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("No genre data found for %s.", cmd.User),
		}
	}

	var content strings.Builder
	periodText := h.formatPeriodText(cmd.Period)
	content.WriteString(fmt.Sprintf("%s's top genres%s:\n\n", cmd.User, periodText))

	for i, genre := range genres {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, genre.Name))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleVibeCommand generates an AI taste profile for a user using Ollama.
func (h *Handler) handleVibeCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	if h.ollamaClient == nil {
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "AI features are not configured. Set OLLAMA_URL in the bot's environment to enable !vibe.",
		}
	}

	genres, err := h.lastfmClient.GetUserTopGenres(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get genres for vibe", "error", err, "user", cmd.User)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	artists, err := h.lastfmClient.GetUserTopArtists(ctx, cmd.User, cmd.Period, 10)
	if err != nil {
		h.logger.Error("Failed to get artists for vibe", "error", err, "user", cmd.User)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(genres) == 0 && len(artists) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("Not enough listening data found for %s.", cmd.User),
		}
	}

	prompt := buildVibePrompt(cmd.User, h.formatPeriodText(cmd.Period), genres, artists)

	h.logger.Debug("Calling Ollama for vibe", "user", cmd.User, "model", h.ollamaClient.Model())
	response, err := h.ollamaClient.Chat(ctx, []ollama.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		h.logger.Error("Ollama call failed for vibe", "error", err)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "AI model is unavailable right now. Try again later.",
		}
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: fmt.Sprintf("*%s's vibe%s* (via %s)\n\n%s", cmd.User, h.formatPeriodText(cmd.Period), h.ollamaClient.Model(), response),
	}
}

// handleDiscoveryTrackCommand processes discovery track commands
func (h *Handler) handleDiscoveryTrackCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	tracks, err := h.lastfmClient.GetUserTopTracksByArtist(ctx, cmd.User, cmd.Artist, 10)
	if err != nil {
		h.logger.Error("Failed to get user tracks by artist", "error", err, "user", cmd.User, "artist", cmd.Artist)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	if len(tracks) == 0 {
		return &types.BotResponse{
			Type:    types.ResponseTypeText,
			Content: fmt.Sprintf("No tracks by %s found for %s! 🎵", cmd.Artist, cmd.User),
		}
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("%s's top tracks by %s:\n\n", cmd.User, cmd.Artist))

	for i, track := range tracks {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, track))
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content.String(),
	}
}

// handleRecapCommand processes manual group recap requests.
func (h *Handler) handleRecapCommand(ctx context.Context, cmd *types.Command) *types.BotResponse {
	if h.ollamaClient == nil {
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: "AI features are not configured. Set OLLAMA_URL in the bot's environment to enable !recap.",
		}
	}

	period := cmd.Period
	if period == "" {
		period = "7d"
	}

	recapCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	content, err := h.generateRecap(recapCtx, period)
	if err != nil {
		h.logger.Error("Failed to generate recap", "error", err, "period", period)
		return &types.BotResponse{
			Type:  types.ResponseTypeError,
			Error: errors.GetUserFriendlyMessage(err),
		}
	}

	return &types.BotResponse{
		Type:    types.ResponseTypeText,
		Content: content,
	}
}

// generateRecap fetches group data and produces an AI narrative for the given period.
func (h *Handler) generateRecap(ctx context.Context, period string) (string, error) {
	data, err := h.lastfmClient.FetchGroupRecapData(ctx, period)
	if err != nil {
		return "", err
	}

	if len(data.Users) == 0 {
		return "No listening activity found for this period.", nil
	}

	prompt := buildRecapPrompt(data)
	h.logger.Debug("Calling Ollama for recap", "period", period, "model", h.ollamaClient.Model())
	response, err := h.ollamaClient.Chat(ctx, []ollama.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return "", fmt.Errorf("AI model unavailable: %w", err)
	}

	return fmt.Sprintf("*Kagang listening recap%s* (via %s)\n\n%s", h.formatPeriodText(period), h.ollamaClient.Model(), response), nil
}

// buildRecapPrompt constructs the AI prompt for a group listening recap.
func buildRecapPrompt(data *lastfm.GroupRecapData) string {
	var sb strings.Builder

	sb.WriteString("You are a witty music journalist writing a brief group listening recap for a friend group chat.\n")
	sb.WriteString("Write a 3-5 sentence narrative summary. Cover:\n")
	sb.WriteString("- What the group has been into overall (shared obsessions, standout albums or tracks)\n")
	sb.WriteString("- Notable individual trends (most active listener, anyone on a unique listening path)\n")
	sb.WriteString("- Any artists that appear across multiple people's lists (call out the shared obsession)\n")
	sb.WriteString("Be specific — reference real artists, albums, and names from the data. Keep it fun and conversational.\n")
	sb.WriteString("Write flowing prose, no bullet points.\n\n")

	sb.WriteString(fmt.Sprintf("Period: %s\n", data.Period))

	sb.WriteString("\nPer-listener data (username, total plays, top artists):\n")
	for _, u := range data.Users {
		if len(u.TopArtists) > 0 {
			sb.WriteString(fmt.Sprintf("- %s (%d plays): %s\n", u.Username, u.TotalPlays, strings.Join(u.TopArtists, ", ")))
		}
	}

	if len(data.TopAlbums) > 0 {
		limit := 5
		if len(data.TopAlbums) < limit {
			limit = len(data.TopAlbums)
		}
		sb.WriteString("\nTop albums across the group:\n")
		for _, a := range data.TopAlbums[:limit] {
			sb.WriteString(fmt.Sprintf("- %s (%d plays, %d listeners)\n", a.AlbumName, a.Playcount, a.UserCount))
		}
	}

	if len(data.TopTracks) > 0 {
		limit := 5
		if len(data.TopTracks) < limit {
			limit = len(data.TopTracks)
		}
		sb.WriteString("\nTop tracks across the group:\n")
		for _, t := range data.TopTracks[:limit] {
			sb.WriteString(fmt.Sprintf("- %s (%d plays)\n", t.TrackName, t.Playcount))
		}
	}

	return sb.String()
}

// runRecapScheduler starts a background goroutine that posts the recap on schedule.
// It is a no-op if RecapSchedule is not "weekly" or "monthly".
func (h *Handler) runRecapScheduler(ctx context.Context) {
	schedule := h.config.RecapSchedule
	if schedule != "weekly" && schedule != "monthly" {
		return
	}

	h.logger.Info("Recap scheduler started", "schedule", schedule, "weekday", h.config.RecapWeekday, "hour", h.config.RecapHour)

	go func() {
		for {
			next := h.nextRecapTime()
			h.logger.Debug("Next recap scheduled", "at", next)

			select {
			case <-time.After(time.Until(next)):
				h.postScheduledRecap(ctx)
			case <-ctx.Done():
				h.logger.Info("Recap scheduler stopping")
				return
			}
		}
	}()
}

// nextRecapTime returns the next time the scheduled recap should fire.
func (h *Handler) nextRecapTime() time.Time {
	now := time.Now()

	switch h.config.RecapSchedule {
	case "weekly":
		target := time.Weekday(h.config.RecapWeekday)
		daysUntil := (int(target) - int(now.Weekday()) + 7) % 7
		if daysUntil == 0 && now.Hour() >= h.config.RecapHour {
			daysUntil = 7 // already past this week's window
		}
		return time.Date(now.Year(), now.Month(), now.Day()+daysUntil, h.config.RecapHour, 0, 0, 0, now.Location())

	case "monthly":
		// First of this month at recap hour; if already past, use next month
		candidate := time.Date(now.Year(), now.Month(), 1, h.config.RecapHour, 0, 0, 0, now.Location())
		if now.After(candidate) {
			candidate = candidate.AddDate(0, 1, 0)
		}
		return candidate

	default:
		return now.Add(100 * 365 * 24 * time.Hour)
	}
}

// postScheduledRecap generates a recap and posts it to the configured Slack channel.
func (h *Handler) postScheduledRecap(ctx context.Context) {
	if h.ollamaClient == nil {
		h.logger.Warn("Skipping scheduled recap: Ollama not configured")
		return
	}

	period := "7d"
	if h.config.RecapSchedule == "monthly" {
		period = "1m"
	}

	h.logger.Info("Generating scheduled recap", "period", period)

	recapCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	content, err := h.generateRecap(recapCtx, period)
	if err != nil {
		h.logger.Error("Failed to generate scheduled recap", "error", err)
		return
	}

	if _, _, err := h.api.PostMessage(h.config.SlackChannelID, slack.MsgOptionText(content, false)); err != nil {
		h.logger.Error("Failed to post scheduled recap", "error", err)
		return
	}

	h.logger.Info("Scheduled recap posted successfully")
}
