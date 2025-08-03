package main

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/syakter/go-lastfm/lastfm"
)

func TestGetArtistScrobbles(t *testing.T) {
	err := godotenv.Load()
	if err != nil {
		t.Fatal("err not nil")
	}
	LF_API_KEY := os.Getenv("LF_API_KEY")
	LF_API_SECRET := os.Getenv("LF_API_SECRET")

	artist := "Future"
	api := lastfm.New(LF_API_KEY, LF_API_SECRET)

	msg := GetArtistScrobbles(artist, api)
	if msg == "" {
		t.Fatal("No str response")
	}
}
