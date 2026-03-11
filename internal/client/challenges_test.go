package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ajesus37/hCTF2/internal/client"
)

func TestGetChallengeWithQuestionsDecodesQuestions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"challenge": map[string]any{
				"id": "ch-1", "name": "Web 101", "category": "web",
				"difficulty": "easy", "initial_points": 100,
			},
			"questions": []map[string]any{
				{"id": "q-1", "name": "Part 1", "flag_mask": "flag{***}", "points": 50},
				{"id": "q-2", "name": "Part 2", "flag_mask": "flag{***}", "points": 50},
			},
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, "token")
	ch, qs, err := c.GetChallengeWithQuestions("ch-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.Title != "Web 101" {
		t.Errorf("expected title 'Web 101', got %q", ch.Title)
	}
	if len(qs) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(qs))
	}
	if qs[0].ID != "q-1" || qs[1].ID != "q-2" {
		t.Errorf("unexpected question IDs: %v %v", qs[0].ID, qs[1].ID)
	}
}
