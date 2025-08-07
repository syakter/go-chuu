package types

import (
	"encoding/json"
	"time"
)

// User statistics types
type UserCount struct {
	Username  string `json:"username"`
	Playcount int    `json:"playcount"`
}

type AlbumCount struct {
	AlbumName string `json:"album_name"`
	Playcount int    `json:"playcount"`  // Total scrobbles across all users
	UserCount int    `json:"user_count"` // Number of users who have this album in their top list
}

type TrackCount struct {
	TrackName string `json:"track_name"`
	Playcount int    `json:"playcount"`  // Total scrobbles across all users
	UserCount int    `json:"user_count"` // Number of users who have this track in their top list
}

// Chart types
type Album struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Image  string `json:"image"`
}

type Track struct {
	TrackName string `json:"track_name"`
	Artist    string `json:"artist"`
}

// Command types
type CommandType string

const (
	CommandHelp           CommandType = "help"
	CommandUptime         CommandType = "uptime"
	CommandChart          CommandType = "chart"
	CommandNowPlaying     CommandType = "np"
	CommandTopTracks      CommandType = "track"
	CommandTopAlbums      CommandType = "top"
	CommandTopArtists     CommandType = "ta"
	CommandRecentTracks   CommandType = "rp"
	CommandLeaderboard    CommandType = "leaderboard"
	CommandArtistFans     CommandType = "artist"
	CommandAlbumFans      CommandType = "album"
	CommandTrackFans      CommandType = "trackfans"
	CommandTopAlbumsAll   CommandType = "kga"
	CommandTopTracksAll   CommandType = "kgt"
	CommandDisco          CommandType = "disco"
	CommandDiscoveryTrack CommandType = "dt"
	CommandUnknown        CommandType = "unknown"
)

// Command represents a parsed user command
type Command struct {
	Type     CommandType `json:"type"`
	User     string      `json:"user,omitempty"`
	Period   string      `json:"period,omitempty"`
	Artist   string      `json:"artist,omitempty"`
	Album    string      `json:"album,omitempty"`
	Track    string      `json:"track,omitempty"`
	Limit    int         `json:"limit,omitempty"`
	RawInput string      `json:"raw_input"`
}

// Response types
type BotResponse struct {
	Type    ResponseType `json:"type"`
	Content string       `json:"content,omitempty"`
	File    *FileUpload  `json:"file,omitempty"`
	Error   string       `json:"error,omitempty"`
}

type ResponseType string

const (
	ResponseTypeText  ResponseType = "text"
	ResponseTypeFile  ResponseType = "file"
	ResponseTypeError ResponseType = "error"
)

type FileUpload struct {
	Filename string   `json:"filename"`
	Path     string   `json:"path"`
	Channels []string `json:"channels"`
	Title    string   `json:"title"`
}

// Statistics types
type UserStats struct {
	Username       string    `json:"username"`
	TotalScrobbles int       `json:"total_scrobbles"`
	LastUpdated    time.Time `json:"last_updated"`
}

type LeaderboardEntry struct {
	Username   string    `json:"username"`
	Scrobbles  int       `json:"scrobbles"`
	Rank       int       `json:"rank"`
	PeriodFrom time.Time `json:"period_from"`
	PeriodTo   time.Time `json:"period_to"`
}

// Cache types
type CacheKey struct {
	Type   string `json:"type"`
	User   string `json:"user,omitempty"`
	Period string `json:"period,omitempty"`
	Artist string `json:"artist,omitempty"`
	Album  string `json:"album,omitempty"`
	Track  string `json:"track,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

func (c CacheKey) String() string {
	return c.Type + ":" + c.User + ":" + c.Period + ":" + c.Artist + ":" + c.Album + ":" + c.Track
}

type CacheEntry struct {
	Key       CacheKey  `json:"key"`
	Data      []byte    `json:"data"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Implement MarshalJSON for cacheable types
type UserCounts []UserCount

func (uc UserCounts) MarshalJSON() ([]byte, error) {
	return json.Marshal([]UserCount(uc))
}

type StringSlice []string

func (ss StringSlice) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(ss))
}

type LeaderboardEntries []LeaderboardEntry

func (le LeaderboardEntries) MarshalJSON() ([]byte, error) {
	return json.Marshal([]LeaderboardEntry(le))
}

type AlbumCounts []AlbumCount

func (ac AlbumCounts) MarshalJSON() ([]byte, error) {
	return json.Marshal([]AlbumCount(ac))
}

type TrackCounts []TrackCount

func (tc TrackCounts) MarshalJSON() ([]byte, error) {
	return json.Marshal([]TrackCount(tc))
}
