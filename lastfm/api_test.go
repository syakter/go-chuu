package lastfm

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	api := New("test_key", "test_secret")
	if api.ApiKey != "test_key" {
		t.Errorf("Expected ApiKey to be 'test_key', got %s", api.ApiKey)
	}
	if api.ApiSecret != "test_secret" {
		t.Errorf("Expected ApiSecret to be 'test_secret', got %s", api.ApiSecret)
	}
	if api.BaseURL != "https://ws.audioscrobbler.com/2.0/" {
		t.Errorf("Expected BaseURL to be 'https://ws.audioscrobbler.com/2.0/', got %s", api.BaseURL)
	}
	if api.User == nil {
		t.Error("Expected User to be initialized")
	}
}

func TestApiCall(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request parameters
		if r.URL.Query().Get("method") != "test.method" {
			t.Errorf("Expected method 'test.method', got %s", r.URL.Query().Get("method"))
		}
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("Expected format 'json', got %s", r.URL.Query().Get("format"))
		}
		if r.URL.Query().Get("api_key") != "test_key" {
			t.Errorf("Expected api_key 'test_key', got %s", r.URL.Query().Get("api_key"))
		}
		if r.URL.Query().Get("test_param") != "test_value" {
			t.Errorf("Expected test_param 'test_value', got %s", r.URL.Query().Get("test_param"))
		}

		// Return a test response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	// Create API client with test server URL
	api := New("test_key", "test_secret")
	api.BaseURL = server.URL + "/"

	// Make test call
	params := P{"test_param": "test_value"}
	response, err := api.call("test.method", params)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if string(response) != `{"success": true}` {
		t.Errorf("Expected response '{\"success\": true}', got %s", string(response))
	}
}

func TestApiCallError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create API client with test server URL
	api := New("test_key", "test_secret")
	api.BaseURL = server.URL + "/"

	// Make test call
	_, err := api.call("test.method", P{})
	if err == nil {
		t.Error("Expected error for status 500, got nil")
	}
}
