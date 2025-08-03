package listeningclub

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Service manages listening club operations
type Service struct {
	storage Storage
	logger  *slog.Logger
}

// NewService creates a new listening club service
func NewService(storage Storage, logger *slog.Logger) *Service {
	return &Service{
		storage: storage,
		logger:  logger,
	}
}

// SetAlbum sets the current week's listening club album
func (s *Service) SetAlbum(artist, album, setBy string) error {
	lc, err := s.storage.Load()
	if err != nil {
		return fmt.Errorf("failed to load listening club data: %w", err)
	}

	// Check if we need to start a new week
	currentWeek, currentYear := GetCurrentWeek()
	if lc.WeekNumber != currentWeek || lc.Year != currentYear {
		// New week, clear votes and update week info
		lc.Votes = make(map[string]*Vote)
		lc.WeekNumber = currentWeek
		lc.Year = currentYear
	}

	weekStart := GetWeekStart()
	weekEnd := GetWeekEnd()

	lc.CurrentAlbum = &Album{
		Artist:    strings.TrimSpace(artist),
		Album:     strings.TrimSpace(album),
		SetBy:     setBy,
		WeekStart: weekStart,
		WeekEnd:   weekEnd,
		Active:    true,
	}

	if err := s.storage.Save(lc); err != nil {
		return fmt.Errorf("failed to save listening club data: %w", err)
	}

	s.logger.Info("Listening club album set",
		"artist", artist,
		"album", album,
		"set_by", setBy,
		"week", currentWeek,
		"year", currentYear)

	return nil
}

// Vote adds or updates a user's vote for the current album
func (s *Service) Vote(platform, userID, username string, rating int, comment string) error {
	if rating < 1 || rating > 10 {
		return fmt.Errorf("rating must be between 1 and 10")
	}

	lc, err := s.storage.Load()
	if err != nil {
		return fmt.Errorf("failed to load listening club data: %w", err)
	}

	if lc.CurrentAlbum == nil || !lc.CurrentAlbum.Active {
		return fmt.Errorf("no active listening club album set")
	}

	// Check if voting period is still active
	if time.Now().After(lc.CurrentAlbum.WeekEnd) {
		return fmt.Errorf("voting period has ended")
	}

	voteKey := GetVoteKey(platform, userID)
	lc.Votes[voteKey] = &Vote{
		Username: username,
		Rating:   rating,
		Comment:  strings.TrimSpace(comment),
		VotedAt:  time.Now(),
		Platform: platform,
		UserID:   userID,
	}

	if err := s.storage.Save(lc); err != nil {
		return fmt.Errorf("failed to save vote: %w", err)
	}

	s.logger.Info("Vote recorded",
		"username", username,
		"rating", rating,
		"platform", platform,
		"album", fmt.Sprintf("%s - %s", lc.CurrentAlbum.Artist, lc.CurrentAlbum.Album))

	return nil
}

// GetCurrentAlbum returns the current listening club album
func (s *Service) GetCurrentAlbum() (*Album, error) {
	lc, err := s.storage.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load listening club data: %w", err)
	}

	if lc.CurrentAlbum == nil || !lc.CurrentAlbum.Active {
		return nil, nil
	}

	return lc.CurrentAlbum, nil
}

// GetVoteStats returns voting statistics for the current album
func (s *Service) GetVoteStats() (*VoteStats, error) {
	lc, err := s.storage.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load listening club data: %w", err)
	}

	if lc.CurrentAlbum == nil {
		return &VoteStats{
			RatingCounts: make(map[int]int),
		}, nil
	}

	stats := &VoteStats{
		RatingCounts: make(map[int]int),
	}

	totalRating := 0
	for _, vote := range lc.Votes {
		stats.TotalVotes++
		totalRating += vote.Rating
		stats.RatingCounts[vote.Rating]++
	}

	if stats.TotalVotes > 0 {
		stats.AverageRating = float64(totalRating) / float64(stats.TotalVotes)
	}

	return stats, nil
}

// GetUserVote returns a specific user's vote
func (s *Service) GetUserVote(platform, userID string) (*Vote, error) {
	lc, err := s.storage.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load listening club data: %w", err)
	}

	voteKey := GetVoteKey(platform, userID)
	vote, exists := lc.Votes[voteKey]
	if !exists {
		return nil, nil
	}

	return vote, nil
}

// GetAllVotes returns all votes for the current album
func (s *Service) GetAllVotes() ([]*Vote, error) {
	lc, err := s.storage.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load listening club data: %w", err)
	}

	votes := make([]*Vote, 0, len(lc.Votes))
	for _, vote := range lc.Votes {
		votes = append(votes, vote)
	}

	return votes, nil
}

// ClearWeek clears the current week's album and votes (admin function)
func (s *Service) ClearWeek() error {
	lc, err := s.storage.Load()
	if err != nil {
		return fmt.Errorf("failed to load listening club data: %w", err)
	}

	lc.CurrentAlbum = nil
	lc.Votes = make(map[string]*Vote)
	currentWeek, currentYear := GetCurrentWeek()
	lc.WeekNumber = currentWeek
	lc.Year = currentYear

	if err := s.storage.Save(lc); err != nil {
		return fmt.Errorf("failed to save cleared data: %w", err)
	}

	s.logger.Info("Listening club week cleared")
	return nil
}

// ParseRating parses a rating string into an integer
func ParseRating(ratingStr string) (int, error) {
	rating, err := strconv.Atoi(strings.TrimSpace(ratingStr))
	if err != nil {
		return 0, fmt.Errorf("invalid rating format: %s", ratingStr)
	}
	if rating < 1 || rating > 10 {
		return 0, fmt.Errorf("rating must be between 1 and 10")
	}
	return rating, nil
}
