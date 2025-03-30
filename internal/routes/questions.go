package routes

import (
	"regexp"

	"github.com/pocketbase/pocketbase/tools/types"
)

type Question struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	FlagMask      string                  `json:"flag_mask"`
	Hints         types.JSONArray[string] `json:"hints"`
	Category      string                  `json:"category"`
	CaseSensitive bool                    `json:"case_sensitive"`
	ChallengeID   string                  `json:"challenge_id"`
}

type RawQuestion struct {
	Question
	Flag string `json:"flag"`
}

func replaceAlphanumericWithAsterisk(input string) string {
	// Define a regex to match all alphanumeric characters
	alphanumericRegex := regexp.MustCompile(`[a-zA-Z0-9]`)
	// Replace all matches with "*"
	return alphanumericRegex.ReplaceAllString(input, "*")
}

func SetMask(r *RawQuestion) Question {
	if r.FlagMask == "" {
		r.FlagMask = replaceAlphanumericWithAsterisk(r.Flag)
	}
	return Question{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		FlagMask:    r.FlagMask,
		Hints:       r.Hints,
		Category:    r.Category,
	}
}
