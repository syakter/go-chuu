package commands

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/syakter/go-chuu/internal/errors"
	"github.com/syakter/go-chuu/internal/types"
)

// Parser handles command parsing and validation
type Parser struct {
	validUsers map[string]bool
}

// NewParser creates a new command parser
func NewParser(validUsers []string) *Parser {
	userMap := make(map[string]bool)
	for _, user := range validUsers {
		userMap[strings.ToLower(user)] = true
	}

	return &Parser{
		validUsers: userMap,
	}
}

// Parse parses a user message into a Command
func (p *Parser) Parse(message string) (*types.Command, error) {
	if message == "" {
		return nil, errors.NewValidationError("empty message")
	}

	// Remove mention prefix and clean up
	message = p.cleanMessage(message)

	command := &types.Command{
		RawInput: message,
		Type:     types.CommandUnknown,
	}

	// Parse different command types
	if err := p.parseCommand(command, message); err != nil {
		return nil, err
	}

	// Validate the parsed command
	if err := p.validateCommand(command); err != nil {
		return nil, err
	}

	return command, nil
}

// cleanMessage removes Slack mentions and extra whitespace
func (p *Parser) cleanMessage(message string) string {
	// Remove Slack user mention format: <@USERID>
	re := regexp.MustCompile(`<@[^>]+>`)
	message = re.ReplaceAllString(message, "")

	// Remove extra whitespace and trim
	message = regexp.MustCompile(`\s+`).ReplaceAllString(message, " ")
	message = strings.TrimSpace(message)

	return message
}

// parseCommand parses the message into command components
func (p *Parser) parseCommand(command *types.Command, message string) error {
	if !strings.HasPrefix(message, "!") {
		// Handle non-command messages (artist/album/track queries)
		return p.parseQuery(command, message)
	}

	// Split command and arguments
	parts := strings.Fields(message)
	if len(parts) == 0 {
		return errors.NewValidationError("invalid command format")
	}

	cmdStr := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmdStr {
	case "!help":
		command.Type = types.CommandHelp

	case "!up", "!uptime":
		command.Type = types.CommandUptime

	case "!chart":
		command.Type = types.CommandChart
		return p.parseChartArgs(command, args)

	case "!np":
		command.Type = types.CommandNowPlaying

	case "!track":
		command.Type = types.CommandTopTracks
		return p.parseUserPeriodArgs(command, args)

	case "!top":
		command.Type = types.CommandTopAlbums
		return p.parseUserPeriodArgs(command, args)

	case "!ta", "!topartist":
		command.Type = types.CommandTopArtists
		return p.parseUserPeriodArgs(command, args)

	case "!rp":
		command.Type = types.CommandRecentTracks
		return p.parseRecentTracksArgs(command, args)

	case "!leaderboard":
		command.Type = types.CommandLeaderboard

	case "!artist":
		command.Type = types.CommandArtistFans
		if len(args) == 0 {
			return errors.NewValidationError("artist name is required")
		}
		command.Artist = strings.Join(args, " ")

	case "!kga":
		command.Type = types.CommandTopAlbumsAll
		if len(args) > 0 {
			command.Period = args[0]
		}

	case "!kgt":
		command.Type = types.CommandTopTracksAll
		if len(args) > 0 {
			command.Period = args[0]
		}

	case "!disco":
		command.Type = types.CommandDisco
		return p.parseUserArtistArgs(command, args)

	case "!dt":
		command.Type = types.CommandDiscoveryTrack
		return p.parseUserArtistArgs(command, args)

	case "!rec":
		command.Type = types.CommandRecommend
		return p.parseUserPeriodArgs(command, args)

	case "!hidden":
		command.Type = types.CommandHiddenGem
		return p.parseUserPeriodArgs(command, args)

	case "!profile":
		command.Type = types.CommandProfile
		return p.parseUserPeriodArgs(command, args)

	case "!t":
		command.Type = types.CommandTrackFans
		return p.parseTrackQuery(command, strings.Join(args, " "))

	default:
		return errors.NewValidationError("unknown command: " + cmdStr)
	}

	return nil
}

// parseQuery handles non-command queries (artist/album by artist)
func (p *Parser) parseQuery(command *types.Command, message string) error {
	if strings.Contains(message, " by ") || strings.Contains(message, " By ") {
		// Album or track by artist - find the last occurrence of " by "
		album, artist, found := p.splitOnLastBy(message)
		if !found {
			return errors.NewValidationError("invalid format: use 'album by artist' or 'track by artist'")
		}

		command.Type = types.CommandAlbumFans
		command.Album = album
		command.Artist = artist
	} else {
		// Just artist name
		command.Type = types.CommandArtistFans
		command.Artist = message
	}

	return nil
}

// parseTrackQuery handles track by artist queries
func (p *Parser) parseTrackQuery(command *types.Command, query string) error {
	if !strings.Contains(query, " by ") && !strings.Contains(query, " By ") {
		return errors.NewValidationError("track format should be: !t track by artist")
	}

	// Find the last occurrence of " by "
	track, artist, found := p.splitOnLastBy(query)
	if !found {
		return errors.NewValidationError("invalid track format: use 'track by artist'")
	}

	command.Track = track
	command.Artist = artist

	return nil
}

// parseChartArgs parses chart command arguments
func (p *Parser) parseChartArgs(command *types.Command, args []string) error {
	if len(args) == 0 {
		return errors.NewValidationError("chart command requires username")
	}

	command.User = args[0]
	command.ChartSize = 3 // default

	if len(args) > 1 {
		command.Period = args[1]
	} else {
		command.Period = "7d" // default
	}

	if len(args) > 2 {
		sizeStr := strings.ToLower(args[2])
		// Accept "NxN" or just "N"
		if idx := strings.Index(sizeStr, "x"); idx != -1 {
			sizeStr = sizeStr[:idx]
		}
		n, err := strconv.Atoi(sizeStr)
		if err != nil || n < 2 || n > 10 {
			return errors.NewValidationError("chart size must be between 2 and 10 (e.g. 5x5)")
		}
		command.ChartSize = n
	}

	return nil
}

// parseUserPeriodArgs parses commands that take user and optional period
func (p *Parser) parseUserPeriodArgs(command *types.Command, args []string) error {
	if len(args) == 0 {
		return errors.NewValidationError("username is required")
	}

	command.User = args[0]
	if len(args) > 1 {
		command.Period = args[1]
	}

	return nil
}

// parseUserArtistArgs parses commands that take user and artist
func (p *Parser) parseUserArtistArgs(command *types.Command, args []string) error {
	if len(args) < 2 {
		return errors.NewValidationError("both username and artist are required")
	}

	command.User = args[0]
	command.Artist = strings.Join(args[1:], " ")

	return nil
}

// parseRecentTracksArgs parses recent tracks command arguments
func (p *Parser) parseRecentTracksArgs(command *types.Command, args []string) error {
	if len(args) == 0 {
		return errors.NewValidationError("username is required")
	}

	command.User = args[0]
	command.Limit = 5 // default

	if len(args) > 1 {
		limit, err := strconv.Atoi(args[1])
		if err != nil {
			return errors.NewValidationError("invalid limit: must be a number")
		}
		if limit > 20 {
			return errors.NewValidationError("limit cannot exceed 20")
		}
		command.Limit = limit
	}

	return nil
}

// validateCommand validates the parsed command
func (p *Parser) validateCommand(command *types.Command) error {
	// Validate user if specified
	if command.User != "" {
		if !p.isValidUser(command.User) {
			return errors.NewValidationError("unknown user: " + command.User)
		}
	}

	// Validate period if specified
	if command.Period != "" {
		if !p.isValidPeriod(command.Period) {
			return errors.NewValidationError("invalid period: " + command.Period + " (valid: 24h, 7d, 1m, 3m, 6m, 1y, overall)")
		}
	}

	// Validate artist/album/track names
	if command.Artist != "" && len(command.Artist) > 100 {
		return errors.NewValidationError("artist name too long")
	}

	if command.Album != "" && len(command.Album) > 100 {
		return errors.NewValidationError("album name too long")
	}

	if command.Track != "" && len(command.Track) > 100 {
		return errors.NewValidationError("track name too long")
	}

	return nil
}

// isValidUser checks if a user is in the valid users list
func (p *Parser) isValidUser(user string) bool {
	return p.validUsers[strings.ToLower(user)]
}

// isValidPeriod checks if a period is valid
func (p *Parser) isValidPeriod(period string) bool {
	validPeriods := []string{"24h", "7d", "1w", "1m", "30d", "3m", "90d", "6m", "180d", "1y", "365d", "overall"}
	period = strings.ToLower(period)

	for _, valid := range validPeriods {
		if period == valid {
			return true
		}
	}

	return false
}

// GetHelpText returns the help text for commands
func GetHelpText() string {
	return `Commands:

` +
		`!np: Now Playing
` +
		`!disco <user> <artist>: Top albums by <artist> for <user>
` +
		`!track <user> <period>: Top tracks for <user> in <period>
` +
		`!dt <user> <artist>: Top tracks by <artist> for <user>
` +
		`!top <user> <period>: Top albums for <user> in <period>
` +
		`!ta <user> <period>: Top artists for <user> in <period>
` +
		`!rp <user> <limit>: Last <limit> songs played by <user>
` +
		`!chart <user> <period> [size]: Visual album chart for <user> (size: 2x2–10x10, default 3x3)
` +
		`!kga <period>: Top listened albums in Kagang in <period>
` +
		`!kgt <period>: Top listened tracks in Kagang in <period>
` +
		`!rec <user> [period]: Artists the group loves that <user> should check out
` +
		`!hidden <user> [period]: <user>'s hidden gems — artists they love that nobody else listens to
` +
		`!profile <user> [period]: HTML music profile card — opens in browser
` +
		`!leaderboard: Leaderboard for previous week
` +
		`!artist <artist>: Top <artist> fans in the group
` +
		`!t <track> by <artist>: Top fans of specific track
` +
		`<album> by <artist>: Top fans of specific album
` +
		`!up: Bot uptime

` +
		`Periods: 24h, 7d, 1m, 3m, 6m, 1y, overall`
}

// splitOnLastBy splits a string on the last occurrence of " by " (case insensitive)
// Returns the parts before and after the last " by ", and whether a split was found
func (p *Parser) splitOnLastBy(message string) (before, after string, found bool) {
	// Find the last occurrence of either " by " or " By "
	lastByIndex := strings.LastIndex(message, " by ")
	lastCapByIndex := strings.LastIndex(message, " By ")

	// Choose the later occurrence
	lastIndex := -1
	if lastByIndex > lastCapByIndex {
		lastIndex = lastByIndex
	} else if lastCapByIndex != -1 {
		lastIndex = lastCapByIndex
	}

	if lastIndex == -1 {
		return "", "", false
	}

	before = strings.TrimSpace(message[:lastIndex])
	after = strings.TrimSpace(message[lastIndex+4:]) // +4 for " by " or " By "
	return before, after, before != "" && after != ""
}
