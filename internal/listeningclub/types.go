package listeningclub

import (
	"time"
)

// Album represents a listening club album
type Album struct {
	Artist    string    `json:"artist"`
	Album     string    `json:"album"`
	SetBy     string    `json:"set_by"`
	WeekStart time.Time `json:"week_start"`
	WeekEnd   time.Time `json:"week_end"`
	Active    bool      `json:"active"`
}

// Vote represents a user's vote for the current album
type Vote struct {
	Username string    `json:"username"`
	Rating   int       `json:"rating"` // 1-10 scale
	Comment  string    `json:"comment,omitempty"`
	VotedAt  time.Time `json:"voted_at"`
	Platform string    `json:"platform"` // "slack" or "discord"
	UserID   string    `json:"user_id"`  // platform-specific user ID
}

// ListeningClub represents the current state of the listening club
type ListeningClub struct {
	CurrentAlbum *Album           `json:"current_album,omitempty"`
	Votes        map[string]*Vote `json:"votes"` // key is platform:user_id to prevent duplicate votes
	WeekNumber   int              `json:"week_number"`
	Year         int              `json:"year"`
}

// VoteStats represents voting statistics for an album
type VoteStats struct {
	TotalVotes    int         `json:"total_votes"`
	AverageRating float64     `json:"average_rating"`
	RatingCounts  map[int]int `json:"rating_counts"` // count of each rating 1-10
}

// GetVoteKey generates a unique key for a vote to prevent duplicates
func GetVoteKey(platform, userID string) string {
	return platform + ":" + userID
}

// GetCurrentWeek returns the current week number and year
func GetCurrentWeek() (int, int) {
	now := time.Now()
	year, week := now.ISOWeek()
	return week, year
}

// GetWeekStart returns the start of the current week (Monday)
func GetWeekStart() time.Time {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7
	}
	// Go back to Monday
	monday := now.AddDate(0, 0, -(weekday - 1))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, monday.Location())
}

// GetWeekEnd returns the end of the current week (Sunday)
func GetWeekEnd() time.Time {
	return GetWeekStart().AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
}
