package help

// GetHelpContent returns the structured help content
func GetHelpContent() *HelpContent {
	return &HelpContent{
		Title: "🎵 go-chuu Bot Commands",
		Sections: []Section{
			{
				Title: "Quick Stats",
				Icon:  "📊",
				Commands: []Command{
					{
						Name:        "np",
						Usage:       "!np",
						Description: "See what everyone is currently listening to",
						Example:     "!np",
					},
					{
						Name:        "uptime",
						Usage:       "!up",
						Description: "Show bot uptime",
						Example:     "!up",
					},
					{
						Name:        "leaderboard",
						Usage:       "!leaderboard",
						Description: "Weekly scrobble leaderboard",
						Example:     "!leaderboard",
					},
				},
			},
			{
				Title: "Personal Stats",
				Icon:  "👤",
				Commands: []Command{
					{
						Name:        "top",
						Usage:       "!top <user> [period]",
						Description: "Top albums for a user",
						Example:     "!top john 1m",
					},
					{
						Name:        "track",
						Usage:       "!track <user> [period]",
						Description: "Top tracks for a user",
						Example:     "!track jane 7d",
					},
					{
						Name:        "ta",
						Usage:       "!ta <user> [period]",
						Description: "Top artists for a user",
						Example:     "!ta bob 3m",
					},
					{
						Name:        "rp",
						Usage:       "!rp <user> [limit]",
						Description: "Recent tracks played by user (max 20)",
						Example:     "!rp alice 10",
					},
					{
						Name:        "chart",
						Usage:       "!chart <user> [period]",
						Description: "Generate 3x3 album artwork chart",
						Example:     "!chart sarah 1m",
					},
				},
			},
			{
				Title: "Group Stats",
				Icon:  "👥",
				Commands: []Command{
					{
						Name:        "kga",
						Usage:       "!kga [period]",
						Description: "Top albums across the entire group",
						Example:     "!kga 1m",
					},
					{
						Name:        "kgt",
						Usage:       "!kgt [period]",
						Description: "Top tracks across the entire group",
						Example:     "!kgt 7d",
					},
				},
			},
			{
				Title: "Artist & Album Discovery",
				Icon:  "🔍",
				Commands: []Command{
					{
						Name:        "artist",
						Usage:       "!artist <artist> | <artist>",
						Description: "Find the biggest fans of an artist",
						Example:     "Radiohead",
					},
					{
						Name:        "album",
						Usage:       "<album> by <artist>",
						Description: "Find the biggest fans of an album",
						Example:     "OK Computer by Radiohead",
					},
					{
						Name:        "track",
						Usage:       "!t <track> by <artist>",
						Description: "Find the biggest fans of a track",
						Example:     "!t Creep by Radiohead",
					},
					{
						Name:        "disco",
						Usage:       "!disco <user> <artist>",
						Description: "User's top albums by specific artist",
						Example:     "!disco john Radiohead",
					},
					{
						Name:        "dt",
						Usage:       "!dt <user> <artist>",
						Description: "User's top tracks by specific artist",
						Example:     "!dt jane Pink Floyd",
					},
				},
			},
			{
				Title: "Listening Club",
				Icon:  "📚",
				Commands: []Command{
					{
						Name:        "lc set",
						Usage:       "!lc set <Artist> - <Album>",
						Description: "Set the weekly listening club album",
						Example:     "!lc set Radiohead - OK Computer",
					},
					{
						Name:        "lc vote",
						Usage:       "!lc vote <1-10> [comment]",
						Description: "Vote on the current album (1-10 scale)",
						Example:     "!lc vote 9 Amazing album!",
					},
					{
						Name:        "lc current",
						Usage:       "!lc current",
						Description: "Show current listening club album",
						Example:     "!lc current",
					},
					{
						Name:        "lc results",
						Usage:       "!lc results",
						Description: "Show voting results and statistics",
						Example:     "!lc results",
					},
					{
						Name:        "lc clear",
						Usage:       "!lc clear",
						Description: "Clear current week (admin only)",
						Example:     "!lc clear",
					},
				},
			},
		},
		Footer: "Time Periods: 24h, 7d, 1m, 3m, 6m, 1y, overall | <> = required, [] = optional",
	}
}
