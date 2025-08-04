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

// GetCurrentWeek returns the current week number and year based on Eastern Time
func GetCurrentWeek() (int, int) {
	// Use the week start date to determine the week number
	// This ensures consistency with our Sunday-based weeks
	weekStart := GetWeekStart()
	year, week := weekStart.ISOWeek()
	return week, year
}

// GetWeekStart returns the start of the current week (Sunday midnight ET)
func GetWeekStart() time.Time {
	// Load Eastern Time zone
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		// Fallback to UTC if timezone loading fails
		et = time.UTC
	}

	// Get current time in Eastern Time
	now := time.Now().In(et)
	weekday := int(now.Weekday())

	// Calculate days to go back to Sunday
	daysBack := weekday
	if weekday == 0 { // Already Sunday
		daysBack = 0
	}

	// Go back to most recent Sunday
	sunday := now.AddDate(0, 0, -daysBack)
	return time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 0, 0, 0, 0, et)
}

// GetWeekEnd returns the end of the current week (Saturday 11:59:59 PM ET)
func GetWeekEnd() time.Time {
	return GetWeekStart().AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
}
