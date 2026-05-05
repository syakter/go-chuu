package profile

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/syakter/go-chuu/internal/lastfm"
)

// AlbumEntry holds data for one album in the profile card
type AlbumEntry struct {
	Name      string
	Artist    string
	URL       string
	ImageData string // "data:image/jpeg;base64,..." or empty
	Playcount int
}

// ArtistEntry holds data for one artist in the profile card
type ArtistEntry struct {
	Name      string
	URL       string
	Playcount int
}

// TrackEntry holds data for one track in the profile card
type TrackEntry struct {
	Name      string
	Artist    string
	URL       string
	Playcount int
}

// RecentEntry holds data for one recent track
type RecentEntry struct {
	Name      string
	Artist    string
	URL       string
	Timestamp string
}

// ProfileData holds all fetched data for the HTML profile card
type ProfileData struct {
	Username     string
	Period       string
	PeriodLabel  string
	ProfileURL   string
	TopAlbums    []AlbumEntry
	TopArtists   []ArtistEntry
	TopTracks    []TrackEntry
	RecentTracks []RecentEntry
}

// albumWithImage is an intermediate type that preserves the image URL
type albumWithImage struct {
	entry    AlbumEntry
	imageURL string
}

// FetchData fetches Last.fm profile data for a user concurrently and returns it.
// Album images are not embedded; use Generate for the HTML version with artwork.
func FetchData(ctx context.Context, api lastfm.APIInterface, username, period string) (*ProfileData, error) {
	normalised := normalizePeriod(period)

	type albumsResult struct {
		items []AlbumEntry
		err   error
	}
	type artistsResult struct {
		items []ArtistEntry
		err   error
	}
	type tracksResult struct {
		items []TrackEntry
		err   error
	}
	type recentResult struct {
		items []RecentEntry
		err   error
	}

	albumsCh := make(chan albumsResult, 1)
	artistsCh := make(chan artistsResult, 1)
	tracksCh := make(chan tracksResult, 1)
	recentCh := make(chan recentResult, 1)

	go func() {
		resp, err := api.GetTopAlbums(ctx, map[string]interface{}{
			"user": username, "period": normalised, "limit": 10,
		})
		if err != nil {
			albumsCh <- albumsResult{err: err}
			return
		}
		var items []AlbumEntry
		for _, a := range resp.TopAlbums.Albums {
			pc, _ := strconv.Atoi(a.PlayCount)
			items = append(items, AlbumEntry{
				Name:      a.Name,
				Artist:    a.Artist.Name,
				URL:       a.URL,
				Playcount: pc,
			})
		}
		albumsCh <- albumsResult{items: items}
	}()

	go func() {
		resp, err := api.GetTopArtists(ctx, map[string]interface{}{
			"user": username, "period": normalised, "limit": 10,
		})
		if err != nil {
			artistsCh <- artistsResult{err: err}
			return
		}
		var items []ArtistEntry
		for _, a := range resp.TopArtists.Artists {
			pc, _ := strconv.Atoi(a.PlayCount)
			items = append(items, ArtistEntry{
				Name:      a.Name,
				URL:       a.URL,
				Playcount: pc,
			})
		}
		artistsCh <- artistsResult{items: items}
	}()

	go func() {
		resp, err := api.GetTopTracks(ctx, map[string]interface{}{
			"user": username, "period": normalised, "limit": 10,
		})
		if err != nil {
			tracksCh <- tracksResult{err: err}
			return
		}
		var items []TrackEntry
		for _, t := range resp.TopTracks.Tracks {
			pc, _ := strconv.Atoi(t.PlayCount)
			items = append(items, TrackEntry{
				Name:      t.Name,
				Artist:    t.Artist.Name,
				URL:       t.URL,
				Playcount: pc,
			})
		}
		tracksCh <- tracksResult{items: items}
	}()

	go func() {
		resp, err := api.GetRecentTracks(ctx, map[string]interface{}{
			"user": username, "limit": 6,
		})
		if err != nil {
			recentCh <- recentResult{err: err}
			return
		}
		var items []RecentEntry
		for _, t := range resp.RecentTracks.Tracks {
			if t.NowPlaying == "true" {
				continue
			}
			ts := t.Date.Text
			if ts == "" {
				ts = "now playing"
			}
			items = append(items, RecentEntry{
				Name:      t.Name,
				Artist:    t.Artist.Name,
				URL:       t.URL,
				Timestamp: ts,
			})
			if len(items) >= 5 {
				break
			}
		}
		recentCh <- recentResult{items: items}
	}()

	ar := <-albumsCh
	tr := <-artistsCh
	tkr := <-tracksCh
	rr := <-recentCh

	var errMsgs []string
	if ar.err != nil {
		errMsgs = append(errMsgs, "albums: "+ar.err.Error())
	}
	if tr.err != nil {
		errMsgs = append(errMsgs, "artists: "+tr.err.Error())
	}
	if tkr.err != nil {
		errMsgs = append(errMsgs, "tracks: "+tkr.err.Error())
	}
	if rr.err != nil {
		errMsgs = append(errMsgs, "recent: "+rr.err.Error())
	}
	if len(errMsgs) == 4 {
		return nil, fmt.Errorf("all API calls failed: %s", strings.Join(errMsgs, "; "))
	}

	return &ProfileData{
		Username:     username,
		Period:       period,
		PeriodLabel:  periodLabel(period),
		ProfileURL:   "https://www.last.fm/user/" + username,
		TopAlbums:    ar.items,
		TopArtists:   tr.items,
		TopTracks:    tkr.items,
		RecentTracks: rr.items,
	}, nil
}

// Generate fetches data for the user, embeds album artwork as base64, renders HTML,
// writes it to a temp file, and returns the file path. Used by the CLI.
func Generate(ctx context.Context, api lastfm.APIInterface, username, period string) (string, error) {
	normalised := normalizePeriod(period)

	type albumsResult struct {
		items []albumWithImage
		err   error
	}
	type artistsResult struct {
		items []ArtistEntry
		err   error
	}
	type tracksResult struct {
		items []TrackEntry
		err   error
	}
	type recentResult struct {
		items []RecentEntry
		err   error
	}

	albumsCh := make(chan albumsResult, 1)
	artistsCh := make(chan artistsResult, 1)
	tracksCh := make(chan tracksResult, 1)
	recentCh := make(chan recentResult, 1)

	go func() {
		resp, err := api.GetTopAlbums(ctx, map[string]interface{}{
			"user": username, "period": normalised, "limit": 10,
		})
		if err != nil {
			albumsCh <- albumsResult{err: err}
			return
		}
		var items []albumWithImage
		for _, a := range resp.TopAlbums.Albums {
			pc, _ := strconv.Atoi(a.PlayCount)
			imgURL := pickBestImage(a.Images)
			items = append(items, albumWithImage{
				entry: AlbumEntry{
					Name:      a.Name,
					Artist:    a.Artist.Name,
					URL:       a.URL,
					Playcount: pc,
				},
				imageURL: imgURL,
			})
		}
		albumsCh <- albumsResult{items: items}
	}()

	go func() {
		resp, err := api.GetTopArtists(ctx, map[string]interface{}{
			"user": username, "period": normalised, "limit": 10,
		})
		if err != nil {
			artistsCh <- artistsResult{err: err}
			return
		}
		var items []ArtistEntry
		for _, a := range resp.TopArtists.Artists {
			pc, _ := strconv.Atoi(a.PlayCount)
			items = append(items, ArtistEntry{
				Name:      a.Name,
				URL:       a.URL,
				Playcount: pc,
			})
		}
		artistsCh <- artistsResult{items: items}
	}()

	go func() {
		resp, err := api.GetTopTracks(ctx, map[string]interface{}{
			"user": username, "period": normalised, "limit": 10,
		})
		if err != nil {
			tracksCh <- tracksResult{err: err}
			return
		}
		var items []TrackEntry
		for _, t := range resp.TopTracks.Tracks {
			pc, _ := strconv.Atoi(t.PlayCount)
			items = append(items, TrackEntry{
				Name:      t.Name,
				Artist:    t.Artist.Name,
				URL:       t.URL,
				Playcount: pc,
			})
		}
		tracksCh <- tracksResult{items: items}
	}()

	go func() {
		resp, err := api.GetRecentTracks(ctx, map[string]interface{}{
			"user": username, "limit": 6,
		})
		if err != nil {
			recentCh <- recentResult{err: err}
			return
		}
		var items []RecentEntry
		for _, t := range resp.RecentTracks.Tracks {
			if t.NowPlaying == "true" {
				continue
			}
			ts := t.Date.Text
			if ts == "" {
				ts = "now playing"
			}
			items = append(items, RecentEntry{
				Name:      t.Name,
				Artist:    t.Artist.Name,
				URL:       t.URL,
				Timestamp: ts,
			})
			if len(items) >= 5 {
				break
			}
		}
		recentCh <- recentResult{items: items}
	}()

	ar := <-albumsCh
	tr := <-artistsCh
	tkr := <-tracksCh
	rr := <-recentCh

	var errMsgs []string
	if ar.err != nil {
		errMsgs = append(errMsgs, "albums: "+ar.err.Error())
	}
	if tr.err != nil {
		errMsgs = append(errMsgs, "artists: "+tr.err.Error())
	}
	if tkr.err != nil {
		errMsgs = append(errMsgs, "tracks: "+tkr.err.Error())
	}
	if rr.err != nil {
		errMsgs = append(errMsgs, "recent: "+rr.err.Error())
	}
	if len(errMsgs) == 4 {
		return "", fmt.Errorf("all API calls failed: %s", strings.Join(errMsgs, "; "))
	}

	albums := embedAlbumImages(ctx, ar.items)

	data := ProfileData{
		Username:     username,
		Period:       period,
		PeriodLabel:  periodLabel(period),
		ProfileURL:   "https://www.last.fm/user/" + username,
		TopAlbums:    albums,
		TopArtists:   tr.items,
		TopTracks:    tkr.items,
		RecentTracks: rr.items,
	}

	html, err := renderHTML(data)
	if err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}

	dir := filepath.Join(os.TempDir(), "go-chuu-profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create profile directory: %w", err)
	}

	periodSlug := period
	if periodSlug == "" {
		periodSlug = "overall"
	}
	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s-%s.html", username, periodSlug, ts)
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return "", fmt.Errorf("failed to write profile file: %w", err)
	}

	return path, nil
}

// embedAlbumImages fetches and base64-encodes album artwork in parallel (max 10 concurrent).
func embedAlbumImages(ctx context.Context, items []albumWithImage) []AlbumEntry {
	results := make([]AlbumEntry, len(items))
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for i, item := range items {
		results[i] = item.entry
		if item.imageURL == "" {
			continue
		}
		wg.Add(1)
		go func(idx int, imgURL string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			results[idx].ImageData = fetchBase64Image(ctx, imgURL)
		}(i, item.imageURL)
	}
	wg.Wait()
	return results
}

// pickBestImage selects the largest available image URL from a slice of Image structs.
func pickBestImage(images []lastfm.Image) string {
	preference := []string{"extralarge", "large", "medium", "small"}
	bySize := make(map[string]string)
	for _, img := range images {
		bySize[img.Size] = img.URL
	}
	for _, size := range preference {
		if u, ok := bySize[size]; ok && u != "" {
			return u
		}
	}
	return ""
}

// fetchBase64Image fetches an image URL and returns a data URI string.
func fetchBase64Image(ctx context.Context, imageURL string) string {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", imageURL, nil)
	if err != nil {
		return ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	encoded := base64.StdEncoding.EncodeToString(body)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
}

// FormatMarkdown formats a ProfileData as a Slack mrkdwn message.
func FormatMarkdown(d *ProfileData) string {
	var b strings.Builder

	fmt.Fprintf(&b, "*<https://www.last.fm/user/%s|%s>* · %s\n", d.Username, d.Username, d.PeriodLabel)

	if len(d.TopArtists) > 0 {
		b.WriteString("\n*Top Artists*\n")
		for i, a := range d.TopArtists {
			if a.Playcount > 0 {
				fmt.Fprintf(&b, "%d. <%s|%s> — %d plays\n", i+1, a.URL, a.Name, a.Playcount)
			} else {
				fmt.Fprintf(&b, "%d. <%s|%s>\n", i+1, a.URL, a.Name)
			}
		}
	}

	if len(d.TopAlbums) > 0 {
		b.WriteString("\n*Top Albums*\n")
		for i, a := range d.TopAlbums {
			if a.Playcount > 0 {
				fmt.Fprintf(&b, "%d. <%s|%s> — %s (%d plays)\n", i+1, a.URL, a.Name, a.Artist, a.Playcount)
			} else {
				fmt.Fprintf(&b, "%d. <%s|%s> — %s\n", i+1, a.URL, a.Name, a.Artist)
			}
		}
	}

	if len(d.TopTracks) > 0 {
		b.WriteString("\n*Top Tracks*\n")
		for i, t := range d.TopTracks {
			if t.Playcount > 0 {
				fmt.Fprintf(&b, "%d. <%s|%s> — %s (%d plays)\n", i+1, t.URL, t.Name, t.Artist, t.Playcount)
			} else {
				fmt.Fprintf(&b, "%d. <%s|%s> — %s\n", i+1, t.URL, t.Name, t.Artist)
			}
		}
	}

	if len(d.RecentTracks) > 0 {
		b.WriteString("\n*Recently Played*\n")
		for _, t := range d.RecentTracks {
			fmt.Fprintf(&b, "• <%s|%s> — %s  _%s_\n", t.URL, t.Name, t.Artist, t.Timestamp)
		}
	}

	return b.String()
}

// normalizePeriod converts user-friendly periods to Last.fm API periods.
func normalizePeriod(period string) string {
	switch strings.ToLower(period) {
	case "7d", "1w":
		return "7day"
	case "1m", "30d":
		return "1month"
	case "3m", "90d":
		return "3month"
	case "6m", "180d":
		return "6month"
	case "1y", "365d":
		return "12month"
	default:
		return "overall"
	}
}

func periodLabel(period string) string {
	switch strings.ToLower(period) {
	case "7d", "1w":
		return "past 7 days"
	case "1m", "30d":
		return "past month"
	case "3m", "90d":
		return "past 3 months"
	case "6m", "180d":
		return "past 6 months"
	case "1y", "365d":
		return "past year"
	case "overall", "":
		return "all time"
	default:
		return period
	}
}

// renderHTML renders the profile data into a self-contained HTML string.
func renderHTML(data ProfileData) (string, error) {
	funcMap := template.FuncMap{
		"add1": func(i int) int { return i + 1 },
	}
	tmpl, err := template.New("profile").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Username}}'s music profile · {{.PeriodLabel}}</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    background: #0d0d0d;
    color: #e8e8e8;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    padding: 2rem;
    max-width: 900px;
    margin: 0 auto;
  }
  a { color: #d63c3c; text-decoration: none; }
  a:hover { text-decoration: underline; }

  .header {
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
    margin-bottom: 2rem;
    border-bottom: 1px solid #2a2a2a;
    padding-bottom: 1rem;
  }
  .header h1 { font-size: 1.6rem; font-weight: 700; }
  .header .period { font-size: 1rem; color: #888; }
  .header .lfm-link { margin-left: auto; font-size: 0.85rem; color: #666; }

  .section { margin-bottom: 2rem; }
  .section-title {
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: #555;
    margin-bottom: 0.9rem;
  }

  .album-grid {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 0.5rem;
  }
  .album-card {
    position: relative;
    aspect-ratio: 1;
    overflow: hidden;
    border-radius: 4px;
    background: #1a1a1a;
  }
  .album-card img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .album-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 2rem;
    color: #2a2a2a;
  }
  .album-overlay {
    position: absolute;
    bottom: 0; left: 0; right: 0;
    background: linear-gradient(transparent, rgba(0,0,0,0.88));
    padding: 0.4rem 0.5rem 0.5rem;
    opacity: 0;
    transition: opacity 0.18s ease;
  }
  .album-card:hover .album-overlay { opacity: 1; }
  .album-name {
    font-size: 0.72rem;
    font-weight: 600;
    line-height: 1.3;
    color: #fff;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .album-artist {
    font-size: 0.65rem;
    color: #bbb;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .album-plays { font-size: 0.6rem; color: #888; margin-top: 0.1rem; }

  .ranked-list { list-style: none; }
  .ranked-list li {
    display: flex;
    align-items: baseline;
    gap: 0.6rem;
    padding: 0.4rem 0;
    border-bottom: 1px solid #1c1c1c;
    font-size: 0.88rem;
  }
  .ranked-list li:last-child { border-bottom: none; }
  .rank-num { min-width: 1.4rem; color: #444; font-size: 0.75rem; text-align: right; }
  .entry-name { flex: 1; }
  .entry-sub { color: #666; font-size: 0.8rem; }
  .entry-plays { color: #555; font-size: 0.78rem; white-space: nowrap; }

  .recent-list { list-style: none; }
  .recent-list li {
    display: flex;
    align-items: baseline;
    gap: 0.6rem;
    padding: 0.4rem 0;
    border-bottom: 1px solid #1c1c1c;
    font-size: 0.85rem;
  }
  .recent-list li:last-child { border-bottom: none; }
  .recent-track { flex: 1; }
  .recent-artist { color: #666; }
  .recent-time { color: #444; font-size: 0.75rem; white-space: nowrap; }

  .two-col {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 2rem;
  }

  @media (max-width: 600px) {
    .album-grid { grid-template-columns: repeat(3, 1fr); }
    .two-col { grid-template-columns: 1fr; }
  }
</style>
</head>
<body>

<header class="header">
  <h1><a href="{{.ProfileURL}}" target="_blank">{{.Username}}</a></h1>
  <span class="period">· {{.PeriodLabel}}</span>
  <a class="lfm-link" href="{{.ProfileURL}}" target="_blank">last.fm ↗</a>
</header>

{{if .TopAlbums}}
<section class="section">
  <div class="section-title">Top Albums</div>
  <div class="album-grid">
    {{range .TopAlbums}}
    <a class="album-card" href="{{.URL}}" target="_blank" title="{{.Artist}} — {{.Name}}">
      {{if .ImageData}}
      <img src="{{.ImageData}}" alt="{{.Name}}" loading="lazy">
      {{else}}
      <div class="album-placeholder">♫</div>
      {{end}}
      <div class="album-overlay">
        <div class="album-name">{{.Name}}</div>
        <div class="album-artist">{{.Artist}}</div>
        {{if .Playcount}}<div class="album-plays">{{.Playcount}} plays</div>{{end}}
      </div>
    </a>
    {{end}}
  </div>
</section>
{{end}}

<div class="two-col">
{{if .TopArtists}}
  <section class="section">
    <div class="section-title">Top Artists</div>
    <ul class="ranked-list">
      {{range $i, $a := .TopArtists}}
      <li>
        <span class="rank-num">{{add1 $i}}</span>
        <span class="entry-name"><a href="{{$a.URL}}" target="_blank">{{$a.Name}}</a></span>
        {{if $a.Playcount}}<span class="entry-plays">{{$a.Playcount}}</span>{{end}}
      </li>
      {{end}}
    </ul>
  </section>
{{end}}

{{if .TopTracks}}
  <section class="section">
    <div class="section-title">Top Tracks</div>
    <ul class="ranked-list">
      {{range $i, $t := .TopTracks}}
      <li>
        <span class="rank-num">{{add1 $i}}</span>
        <span class="entry-name">
          <a href="{{$t.URL}}" target="_blank">{{$t.Name}}</a>
          <span class="entry-sub"> — {{$t.Artist}}</span>
        </span>
        {{if $t.Playcount}}<span class="entry-plays">{{$t.Playcount}}</span>{{end}}
      </li>
      {{end}}
    </ul>
  </section>
{{end}}
</div>

{{if .RecentTracks}}
<section class="section">
  <div class="section-title">Recent Tracks</div>
  <ul class="recent-list">
    {{range .RecentTracks}}
    <li>
      <span class="recent-track">
        <a href="{{.URL}}" target="_blank">{{.Name}}</a>
        <span class="recent-artist"> — {{.Artist}}</span>
      </span>
      <span class="recent-time">{{.Timestamp}}</span>
    </li>
    {{end}}
  </ul>
</section>
{{end}}

</body>
</html>`
