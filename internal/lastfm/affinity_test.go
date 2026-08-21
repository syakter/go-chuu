package lastfm

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	apperrors "github.com/syakter/go-chuu/internal/errors"
)

// affinityMockAPI serves per-user top artists so similarity scenarios can be expressed, and can
// make selected users fail to exercise the skip-and-continue path. All other API methods come
// from the embedded mockAPI.
type affinityMockAPI struct {
	*mockAPI
	artists   map[string]map[string]int // username -> artist -> playcount
	failUsers map[string]bool
}

func (m *affinityMockAPI) GetTopArtists(ctx context.Context, params map[string]interface{}) (*TopArtistsResponse, error) {
	m.recordCall("GetTopArtists")

	user, _ := params["user"].(string)
	if m.failUsers[user] {
		return nil, errors.New("simulated API failure")
	}

	resp := &TopArtistsResponse{}
	for name, playcount := range m.artists[user] {
		resp.TopArtists.Artists = append(resp.TopArtists.Artists, Artist{
			Name:      name,
			PlayCount: strconv.Itoa(playcount),
		})
	}

	return resp, nil
}

// vectorsFrom builds the (vectors, canonical, display) triple computeAffinity expects from a
// simple username -> artist -> playcount map, keeping the given casing as the canonical form.
func vectorsFrom(users map[string]map[string]int) (map[string]map[string]int, map[string]string, map[string]string) {
	vectors := make(map[string]map[string]int, len(users))
	canonical := make(map[string]string)
	display := make(map[string]string, len(users))

	for user, artists := range users {
		vector := make(map[string]int, len(artists))
		for artist, playcount := range artists {
			key := strings.ToLower(artist)
			vector[key] = playcount
			if _, ok := canonical[key]; !ok {
				canonical[key] = artist
			}
		}
		userLower := strings.ToLower(user)
		vectors[userLower] = vector
		display[userLower] = user
	}

	return vectors, canonical, display
}

// Fixtures are all at least affinityMinArtists long, since shorter vectors are skipped by design.
var (
	indieTaste = map[string]int{"Radiohead": 500, "Duster": 200, "Slowdive": 100, "Alvvays": 50, "Big Thief": 20}
	metalTaste = map[string]int{"Metallica": 500, "Slayer": 200, "Anthrax": 100, "Megadeth": 50, "Testament": 20}
)

func TestComputeAffinity_IdenticalTasteScoresOne(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": indieTaste,
		"bob":   indieTaste,
	})

	scores := computeAffinity("alice", vectors, canonical, display)

	if len(scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(scores))
	}
	if math.Abs(scores[0].Score-1.0) > 1e-9 {
		t.Errorf("Expected identical vectors to score 1.0, got %f", scores[0].Score)
	}
	if scores[0].SharedCount != len(indieTaste) {
		t.Errorf("Expected %d shared artists, got %d", len(indieTaste), scores[0].SharedCount)
	}
}

func TestComputeAffinity_DisjointTasteScoresZero(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": indieTaste,
		"bob":   metalTaste,
	})

	scores := computeAffinity("alice", vectors, canonical, display)

	if len(scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(scores))
	}
	if scores[0].Score != 0 {
		t.Errorf("Expected disjoint vectors to score 0, got %f", scores[0].Score)
	}
	if scores[0].SharedCount != 0 {
		t.Errorf("Expected 0 shared artists, got %d", scores[0].SharedCount)
	}
	if len(scores[0].TopShared) != 0 {
		t.Errorf("Expected no shared artists listed, got %v", scores[0].TopShared)
	}
}

// A single shared artist must not produce a perfect score — that is the failure mode of
// comparing only the intersection instead of the union.
func TestComputeAffinity_SingleOverlapIsNotPerfect(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": indieTaste,
		"bob":   {"Radiohead": 500, "Metallica": 200, "Slayer": 100, "Anthrax": 50, "Megadeth": 20},
	})

	scores := computeAffinity("alice", vectors, canonical, display)

	if len(scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(scores))
	}
	if scores[0].Score >= 0.9 {
		t.Errorf("Expected a single overlap out of five artists to score well below 1.0, got %f", scores[0].Score)
	}
	if scores[0].Score <= 0 {
		t.Errorf("Expected a single overlap to score above 0, got %f", scores[0].Score)
	}
}

// The point of log scaling: a light listener with the same proportions as a heavy listener
// should still rank as a close match.
func TestComputeAffinity_VolumeIndependence(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"heavy": {"Radiohead": 4000, "Duster": 2000, "Slowdive": 1000, "Alvvays": 500, "Big Thief": 200},
		"light": {"Radiohead": 40, "Duster": 20, "Slowdive": 10, "Alvvays": 5, "Big Thief": 2},
		"other": {"Radiohead": 4000, "Metallica": 2000, "Slayer": 1000, "Anthrax": 500, "Megadeth": 200},
	})

	scores := computeAffinity("heavy", vectors, canonical, display)

	if len(scores) != 2 {
		t.Fatalf("Expected 2 scores, got %d", len(scores))
	}
	if scores[0].Username != "light" {
		t.Errorf("Expected the proportionally-identical light listener to rank first, got %s", scores[0].Username)
	}
	if scores[0].Score < 0.95 {
		t.Errorf("Expected a light listener with matching proportions to score above 0.95, got %f", scores[0].Score)
	}
}

func TestComputeAffinity_ExcludesTargetAndRanksDescending(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": indieTaste,
		"bob":   indieTaste,
		"carol": {"Radiohead": 500, "Duster": 200, "Metallica": 100, "Slayer": 50, "Anthrax": 20},
		"dave":  metalTaste,
	})

	scores := computeAffinity("alice", vectors, canonical, display)

	if len(scores) != 3 {
		t.Fatalf("Expected 3 scores, got %d", len(scores))
	}
	for _, score := range scores {
		if strings.EqualFold(score.Username, "alice") {
			t.Errorf("Target user should not appear in their own ranking")
		}
	}
	for i := 1; i < len(scores); i++ {
		if scores[i-1].Score < scores[i].Score {
			t.Errorf("Scores are not sorted descending: %v", scores)
		}
	}
	want := []string{"bob", "carol", "dave"}
	for i, name := range want {
		if scores[i].Username != name {
			t.Errorf("Expected %s at index %d, got %s", name, i, scores[i].Username)
		}
	}
}

// Map iteration order is randomised, so repeated runs over the same input must produce
// byte-identical ordering.
func TestComputeAffinity_DeterministicOrderingOnTies(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": indieTaste,
		"zoe":   indieTaste,
		"bob":   indieTaste,
		"carol": indieTaste,
	})

	first := computeAffinity("alice", vectors, canonical, display)
	for i := 0; i < 20; i++ {
		next := computeAffinity("alice", vectors, canonical, display)
		if len(next) != len(first) {
			t.Fatalf("Result length changed between runs: %d vs %d", len(first), len(next))
		}
		for j := range first {
			if first[j].Username != next[j].Username {
				t.Fatalf("Ordering is not deterministic: %s vs %s at index %d", first[j].Username, next[j].Username, j)
			}
		}
	}

	// All scores tie, so the tiebreak is alphabetical
	want := []string{"bob", "carol", "zoe"}
	for i, name := range want {
		if first[i].Username != name {
			t.Errorf("Expected %s at index %d, got %s", name, i, first[i].Username)
		}
	}
}

func TestComputeAffinity_TopSharedRankedByContribution(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": indieTaste,
		"bob":   indieTaste,
	})

	scores := computeAffinity("alice", vectors, canonical, display)

	if len(scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(scores))
	}
	// Ranked by contribution to the dot product, i.e. by playcount for identical vectors
	want := []string{"Radiohead", "Duster", "Slowdive"}
	if len(scores[0].TopShared) != len(want) {
		t.Fatalf("Expected %d top shared artists, got %v", len(want), scores[0].TopShared)
	}
	for i, name := range want {
		if scores[0].TopShared[i] != name {
			t.Errorf("Expected %s at index %d, got %s", name, i, scores[0].TopShared[i])
		}
	}
}

func TestComputeAffinity_EdgeCases(t *testing.T) {
	t.Run("unknown target", func(t *testing.T) {
		vectors, canonical, display := vectorsFrom(map[string]map[string]int{"bob": indieTaste})
		if scores := computeAffinity("alice", vectors, canonical, display); scores != nil {
			t.Errorf("Expected nil for a target with no data, got %v", scores)
		}
	})

	t.Run("target with only zero playcounts", func(t *testing.T) {
		vectors, canonical, display := vectorsFrom(map[string]map[string]int{
			"alice": {"Radiohead": 0, "Duster": 0, "Slowdive": 0, "Alvvays": 0, "Big Thief": 0},
			"bob":   indieTaste,
		})
		if scores := computeAffinity("alice", vectors, canonical, display); scores != nil {
			t.Errorf("Expected nil for a zero-norm target, got %v", scores)
		}
	})

	t.Run("other user with only zero playcounts is skipped", func(t *testing.T) {
		vectors, canonical, display := vectorsFrom(map[string]map[string]int{
			"alice": indieTaste,
			"bob":   {"Radiohead": 0, "Duster": 0, "Slowdive": 0, "Alvvays": 0, "Big Thief": 0},
			"carol": indieTaste,
		})
		scores := computeAffinity("alice", vectors, canonical, display)
		if len(scores) != 1 || scores[0].Username != "carol" {
			t.Errorf("Expected only carol to be scored, got %v", scores)
		}
	})

	t.Run("target alone in the group", func(t *testing.T) {
		vectors, canonical, display := vectorsFrom(map[string]map[string]int{"alice": indieTaste})
		if scores := computeAffinity("alice", vectors, canonical, display); len(scores) != 0 {
			t.Errorf("Expected no scores when nobody else is configured, got %v", scores)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if scores := computeAffinity("alice", nil, nil, nil); scores != nil {
			t.Errorf("Expected nil for empty input, got %v", scores)
		}
	})
}

// Cosine over two- or three-artist vectors measures coincidence, not taste
func TestComputeAffinity_SkipsTinyVectors(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": indieTaste,
		"tiny":  {"Radiohead": 500, "Duster": 200},
		"bob":   indieTaste,
	})

	scores := computeAffinity("alice", vectors, canonical, display)

	if len(scores) != 1 || scores[0].Username != "bob" {
		t.Errorf("Expected the two-artist listener to be skipped, got %v", scores)
	}

	// ...and a target with too little data of their own yields nothing at all
	small, smallCanonical, smallDisplay := vectorsFrom(map[string]map[string]int{
		"alice": {"Radiohead": 500, "Duster": 200},
		"bob":   indieTaste,
	})
	if got := computeAffinity("alice", small, smallCanonical, smallDisplay); got != nil {
		t.Errorf("Expected nil for a target below the minimum vector size, got %v", got)
	}
}

// A NaN score would sort non-deterministically and render as "NaN%"
func TestComputeAffinity_NeverProducesNaN(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": indieTaste,
		"bob":   indieTaste,
		"carol": metalTaste,
		"dave":  {"Radiohead": 0, "Duster": 0, "Slowdive": 0, "Alvvays": 0, "Big Thief": 0},
	})

	for _, score := range computeAffinity("alice", vectors, canonical, display) {
		if math.IsNaN(score.Score) || math.IsInf(score.Score, 0) {
			t.Errorf("Got a non-finite score for %s: %f", score.Username, score.Score)
		}
	}
}

// Artist names are matched case-insensitively but must be displayed with their original casing.
func TestComputeAffinity_PreservesCanonicalCasing(t *testing.T) {
	vectors, canonical, display := vectorsFrom(map[string]map[string]int{
		"alice": {"Boards of Canada": 300, "Duster": 200, "Slowdive": 100, "Alvvays": 50, "Big Thief": 20},
	})
	// bob spells the artist differently; matching should still work
	vectors["bob"] = map[string]int{"boards of canada": 300, "duster": 200, "slowdive": 100, "alvvays": 50, "big thief": 20}
	display["bob"] = "bob"

	scores := computeAffinity("alice", vectors, canonical, display)

	if len(scores) != 1 {
		t.Fatalf("Expected 1 score, got %d", len(scores))
	}
	if scores[0].SharedCount != 5 {
		t.Errorf("Expected case-insensitive matching to find 5 shared artists, got %d", scores[0].SharedCount)
	}
	if len(scores[0].TopShared) == 0 || scores[0].TopShared[0] != "Boards of Canada" {
		t.Errorf("Expected canonical casing to be preserved, got %v", scores[0].TopShared)
	}
}

func TestClient_GetAffinity_OneCallPerUser(t *testing.T) {
	client := createTestClientWithUsers([]string{"testuser1", "testuser2", "testuser3"})

	if _, err := client.GetAffinity(context.Background(), "testuser1", "overall"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mock := client.api.(*mockAPI)
	if got := mock.calls["GetTopArtists"]; got != 3 {
		t.Errorf("Expected GetTopArtists to be called once per user (3), got %d", got)
	}
}

// The per-user artist vectors are cached, so a second caller in the same TTL window makes no
// API calls at all — that is the whole reason GetUserTopArtistsWithPlaycounts exists.
func TestClient_GetAffinity_ReusesCachedUserVectors(t *testing.T) {
	client := createTestClientWithUsers([]string{"testuser1", "testuser2", "testuser3"})
	mock := client.api.(*mockAPI)

	if _, err := client.GetAffinity(context.Background(), "testuser1", "overall"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	after := mock.calls["GetTopArtists"]

	// A different target user over the same period reuses every cached vector
	if _, err := client.GetAffinity(context.Background(), "testuser2", "overall"); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if got := mock.calls["GetTopArtists"]; got != after {
		t.Errorf("Expected the second caller to make no API calls, went from %d to %d", after, got)
	}
}

func TestClient_GetAffinity_SkipsFailingUsers(t *testing.T) {
	client := createTestClientWithUsers([]string{"testuser1", "testuser2", "testuser3"})
	client.api = &affinityMockAPI{
		mockAPI:   newMockAPI(),
		failUsers: map[string]bool{"testuser2": true},
		artists: map[string]map[string]int{
			"testuser1": indieTaste,
			"testuser2": indieTaste,
			"testuser3": indieTaste,
		},
	}

	// A failing user other than the target is logged and omitted, not fatal
	scores, err := client.GetAffinity(context.Background(), "testuser1", "overall")
	if err != nil {
		t.Fatalf("Expected a failing user to be skipped, got error: %v", err)
	}
	if len(scores) != 1 || scores[0].Username != "testuser3" {
		t.Errorf("Expected only testuser3 in the ranking, got %v", scores)
	}
}

// Losing the target's own data is a different failure from the target having no data — reporting
// it as "not enough listening data" would misdiagnose an API outage.
func TestClient_GetAffinity_TargetFailureIsFatal(t *testing.T) {
	client := createTestClientWithUsers([]string{"testuser1", "testuser2"})
	client.api = &affinityMockAPI{
		mockAPI:   newMockAPI(),
		failUsers: map[string]bool{"testuser1": true},
		artists:   map[string]map[string]int{"testuser2": indieTaste},
	}

	_, err := client.GetAffinity(context.Background(), "testuser1", "overall")
	if err == nil {
		t.Fatal("Expected an error when the target's own data cannot be fetched")
	}
	if !apperrors.IsType(err, apperrors.ErrorTypeAPI) {
		t.Errorf("Expected an API error, got %v", err)
	}
}

func TestClient_GetAffinity_Rejects24h(t *testing.T) {
	client := createTestClientWithUsers([]string{"testuser1", "testuser2"})

	_, err := client.GetAffinity(context.Background(), "testuser1", "24h")
	if err == nil {
		t.Fatal("Expected 24h to be rejected")
	}
	if !apperrors.IsType(err, apperrors.ErrorTypeValidation) {
		t.Errorf("Expected a validation error, got %v", err)
	}
	if mock := client.api.(*mockAPI); mock.calls["GetTopArtists"] != 0 {
		t.Errorf("Expected 24h to be rejected before any API call, got %d calls", mock.calls["GetTopArtists"])
	}
}

// Goroutines finish in a random order, so the displayed casing must not depend on which user's
// fetch lands first. The target's own spelling wins.
func TestClient_GetAffinity_TargetCasingWinsDeterministically(t *testing.T) {
	for i := 0; i < 20; i++ {
		client := createTestClientWithUsers([]string{"testuser1", "testuser2"})
		client.api = &affinityMockAPI{
			mockAPI: newMockAPI(),
			artists: map[string]map[string]int{
				"testuser1": {"MF DOOM": 500, "Duster": 200, "Slowdive": 100, "Alvvays": 50, "Big Thief": 20},
				"testuser2": {"MF Doom": 500, "Duster": 200, "Slowdive": 100, "Alvvays": 50, "Big Thief": 20},
			},
		}

		scores, err := client.GetAffinity(context.Background(), "testuser1", "overall")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(scores) != 1 {
			t.Fatalf("Expected 1 score, got %d", len(scores))
		}
		if scores[0].TopShared[0] != "MF DOOM" {
			t.Fatalf("Expected the target's casing to win, got %q on run %d", scores[0].TopShared[0], i)
		}
	}
}

func TestClient_GetAffinity_RanksGroup(t *testing.T) {
	client := createTestClientWithUsers([]string{"testuser1", "testuser2", "testuser3"})
	client.api = &affinityMockAPI{
		mockAPI: newMockAPI(),
		artists: map[string]map[string]int{
			"testuser1": indieTaste,
			"testuser2": indieTaste,
			"testuser3": metalTaste,
		},
	}

	scores, err := client.GetAffinity(context.Background(), "testuser1", "overall")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(scores) != 2 {
		t.Fatalf("Expected 2 scores, got %d", len(scores))
	}
	if scores[0].Username != "testuser2" {
		t.Errorf("Expected testuser2 to rank first, got %s", scores[0].Username)
	}
	if math.Abs(scores[0].Score-1.0) > 1e-9 {
		t.Errorf("Expected an identical listener to score 1.0, got %f", scores[0].Score)
	}
	if scores[1].Score != 0 {
		t.Errorf("Expected a disjoint listener to score 0, got %f", scores[1].Score)
	}
}
