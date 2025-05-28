package lastfm

import "encoding/json"

type UserApi struct {
	api *Api
}

type RecentTracksResponse struct {
	RecentTracks struct {
		Track []struct {
			Artist struct {
				Name string `json:"#text"`
			} `json:"artist"`
			Name string `json:"name"`
		} `json:"track"`
	} `json:"recenttracks"`
}

type TopArtistsResponse struct {
	TopArtists struct {
		Artist []struct {
			Name       string `json:"name"`
			PlayCount  string `json:"playcount"`
			Listeners  string `json:"listeners"`
			Mbid       string `json:"mbid"`
			Url        string `json:"url"`
			Streamable string `json:"streamable"`
		} `json:"artist"`
	} `json:"topartists"`
}

type RecentTracks struct {
	Tracks []Track
}

type Track struct {
	Artist struct {
		Name string
	}
	Name string
}

type TopArtists struct {
	Artists []Artist
}

type Artist struct {
	Name string
}

func (api *UserApi) GetRecentTracks(params P) (*RecentTracks, error) {
	logger.Debug("getting recent tracks",
		"user", params["user"],
		"limit", params["limit"],
	)

	data, err := api.api.call("user.getRecentTracks", params)
	if err != nil {
		logger.Error("failed to get recent tracks",
			"user", params["user"],
			"error", err,
		)
		return nil, err
	}

	var response RecentTracksResponse
	if err := json.Unmarshal(data, &response); err != nil {
		logger.Error("failed to parse recent tracks response",
			"user", params["user"],
			"error", err,
		)
		return nil, err
	}

	result := &RecentTracks{
		Tracks: make([]Track, len(response.RecentTracks.Track)),
	}

	for i, track := range response.RecentTracks.Track {
		result.Tracks[i] = Track{
			Name: track.Name,
			Artist: struct{ Name string }{
				Name: track.Artist.Name,
			},
		}
	}

	logger.Debug("successfully retrieved recent tracks",
		"user", params["user"],
		"track_count", len(result.Tracks),
	)
	return result, nil
}

func (api *UserApi) GetTopArtists(params P) (*TopArtists, error) {
	logger.Debug("getting top artists",
		"user", params["user"],
		"period", params["period"],
	)

	data, err := api.api.call("user.getTopArtists", params)
	if err != nil {
		logger.Error("failed to get top artists",
			"user", params["user"],
			"error", err,
		)
		return nil, err
	}

	var response TopArtistsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		logger.Error("failed to parse top artists response",
			"user", params["user"],
			"error", err,
		)
		return nil, err
	}

	result := &TopArtists{
		Artists: make([]Artist, len(response.TopArtists.Artist)),
	}

	for i, artist := range response.TopArtists.Artist {
		result.Artists[i] = Artist{
			Name: artist.Name,
		}
	}

	logger.Debug("successfully retrieved top artists",
		"user", params["user"],
		"artist_count", len(result.Artists),
	)
	return result, nil
}
