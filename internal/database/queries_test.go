package database

import (
	"testing"

	"github.com/ajesus37/hCTF2/internal/models"
)

func TestCalculateRanks(t *testing.T) {
	tests := []struct {
		name     string
		scores   []int
		expected []int
	}{
		{
			name:     "no ties",
			scores:   []int{100, 90, 80, 70},
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "two-way tie",
			scores:   []int{100, 90, 90, 80},
			expected: []int{1, 2, 2, 4}, // 1224 ranking
		},
		{
			name:     "three-way tie",
			scores:   []int{100, 100, 100, 90},
			expected: []int{1, 1, 1, 4},
		},
		{
			name:     "multiple ties",
			scores:   []int{100, 90, 90, 80, 80, 80, 70},
			expected: []int{1, 2, 2, 4, 4, 4, 7},
		},
		{
			name:     "tie at top",
			scores:   []int{100, 100, 90},
			expected: []int{1, 1, 3},
		},
		{
			name:     "tie at bottom",
			scores:   []int{100, 90, 90},
			expected: []int{1, 2, 2},
		},
		{
			name:     "all tied",
			scores:   []int{100, 100, 100, 100},
			expected: []int{1, 1, 1, 1},
		},
		{
			name:     "single entry",
			scores:   []int{100},
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create entries with the given scores
			entries := make([]models.ScoreboardEntry, len(tt.scores))
			for i, score := range tt.scores {
				entries[i] = models.ScoreboardEntry{
					UserID: "user" + string(rune('A'+i)),
					Points: score,
				}
			}

			// Apply competition ranking logic
			result := applyCompetitionRanking(entries)

			for i, expected := range tt.expected {
				if result[i].Rank != expected {
					t.Errorf("Entry %d (score %d): expected rank %d, got %d",
						i, tt.scores[i], expected, result[i].Rank)
				}
			}
		})
	}
}

// applyCompetitionRanking applies standard competition ranking (1224 rule)
func applyCompetitionRanking(entries []models.ScoreboardEntry) []models.ScoreboardEntry {
	if len(entries) == 0 {
		return entries
	}

	rank := 1
	var prevPoints int

	for i := range entries {
		// Same score = same rank, different score = next rank (skipping if needed)
		if i > 0 && entries[i].Points < prevPoints {
			rank = i + 1
		}
		entries[i].Rank = rank
		prevPoints = entries[i].Points
	}

	return entries
}
