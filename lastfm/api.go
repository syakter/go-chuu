package lastfm

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type Api struct {
	ApiKey    string
	ApiSecret string
	Client    *http.Client
	BaseURL   string
	User      *UserApi
}

type P map[string]any

func New(apiKey, apiSecret string) *Api {
	logger.Debug("creating new Last.fm API client", "api_key", apiKey)
	api := &Api{
		ApiKey:    apiKey,
		ApiSecret: apiSecret,
		Client:    &http.Client{},
		BaseURL:   "https://ws.audioscrobbler.com/2.0/",
	}
	api.User = &UserApi{api: api}
	return api
}

func (api *Api) call(method string, params P) ([]byte, error) {
	logger.Debug("making API call",
		"method", method,
		"base_url", api.BaseURL,
	)

	values := url.Values{}
	values.Set("method", method)
	values.Set("api_key", api.ApiKey)
	values.Set("format", "json")

	for k, v := range params {
		switch v := v.(type) {
		case string:
			values.Set(k, v)
		case int:
			values.Set(k, strconv.Itoa(v))
		case bool:
			values.Set(k, strconv.FormatBool(v))
		}

	}

	resp, err := api.Client.Get(api.BaseURL + "?" + values.Encode())
	if err != nil {
		logger.Error("API request failed",
			"method", method,
			"error", err,
		)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("API request returned non-200 status",
			"method", method,
			"status", resp.StatusCode,
		)
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("failed to read response body",
			"method", method,
			"error", err,
		)
		return nil, err
	}

	logger.Debug("API call successful",
		"method", method,
		"response_size", len(body),
	)
	return body, nil
}
