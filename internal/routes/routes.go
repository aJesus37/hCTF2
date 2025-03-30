package routes

import (
	"encoding/json"
	"fmt"
	"hCTF/internal/templates"
	"log"
	"net/http"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/template"
)

type Result struct {
	Success bool `json:"success"`
	RawQuestion
}

type Challenge struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ValidateFlag validates a submitted flag
func SubmitQuestionAnswer(app *pocketbase.PocketBase, templateRegistry *template.Registry, re *core.RequestEvent) error {
	flag := re.Request.FormValue("flag")
	fmt.Printf("Flag submitted: %s\n", flag)
	questionId := re.Request.PathValue("id")

	// Fetch the question from the database
	question := RawQuestion{}

	err := app.DB().NewQuery("SELECT * FROM questions WHERE id = {:id}").Bind(dbx.Params{"id": questionId}).One(&question)
	if err != nil {
		return apis.NewInternalServerError("Database error", err)
	}

	result := Result{
		Success:     false,
		RawQuestion: question,
	}

	// Check if the provided flag matches the stored flag
	if question.CaseSensitive {
		fmt.Printf("Case sensitive flag check: %s vs %s\n", flag, question.Flag)
		if flag == question.Flag {
			result = Result{
				Success:     true,
				RawQuestion: question,
			}
		}
	} else {
		fmt.Printf("Case insensitive flag check: %s vs %s\n", strings.ToLower(flag), strings.ToLower(question.Flag))
		if strings.EqualFold(flag, question.Flag) {
			result = Result{
				Success:     true,
				RawQuestion: question,
			}
		}
	}

	// Render home page in HTML
	html, err := templateRegistry.LoadFS(templates.TemplateFS, "html/submit-question.html").Render(result)
	if err != nil {
		log.Printf("Template error details: %v", err)
		return apis.NewBadRequestError("Template error", err)
	}
	return re.HTML(http.StatusOK, html)
}

// APIGetChallenges returns all challenges
func ListChallenges(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	// Fetch all challenges from the database
	challenges := []Challenge{}
	err := app.DB().NewQuery("SELECT * FROM challenges").All(&challenges)
	if err != nil {
		return apis.NewInternalServerError("Could not list challenges", err)
	}

	jsonData, err := json.Marshal(challenges)
	if err != nil {
		return apis.NewInternalServerError("Failed to serialize challenges", err)
	}

	re.Response.WriteHeader(http.StatusOK)
	re.Response.Write(jsonData)
	return nil
}

// APIGetQuestions returns questions for a challenge
func ListChallengeQuestions(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	challengeID := re.Request.PathValue("id")

	// Fetch all questions for the given challenge from the database
	questions := []Question{}
	err := app.DB().NewQuery("SELECT * FROM questions WHERE challenge_id = {:id}").Bind(dbx.Params{"id": challengeID}).All(&questions)
	if err != nil {
		return apis.NewInternalServerError("Could not list questions", err)
	}

	jsonData, err := json.Marshal(questions)
	if err != nil {
		return apis.NewInternalServerError("Failed to serialize questions", err)
	}

	re.Response.WriteHeader(http.StatusOK)
	re.Response.Write(jsonData)
	return nil
}

func CreateChallenge(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	challenge := Challenge{}
	if err := json.NewDecoder(re.Request.Body).Decode(&challenge); err != nil {
		return apis.NewBadRequestError("Invalid request body", err)
	}

	// Insert the new challenge into the database
	_, err := app.DB().NewQuery("INSERT INTO challenges (name, description) VALUES ({:name}, {:description})").Bind(dbx.Params{
		"name":        challenge.Name,
		"description": challenge.Description,
	}).Execute()

	if err != nil {
		fmt.Printf("Error inserting challenge: %v\n", err)
		return apis.NewInternalServerError("Could not create challenge", err)
	}

	re.Response.WriteHeader(http.StatusCreated)
	return nil
}

func DeleteChallenge(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	challengeID := re.Request.PathValue("id")

	// Delete the challenge from the database
	_, err := app.DB().NewQuery("DELETE FROM challenges WHERE id = {:id}").Bind(dbx.Params{"id": challengeID}).Execute()
	if err != nil {
		return apis.NewInternalServerError("Could not delete challenge", err)
	}

	re.Response.WriteHeader(http.StatusNoContent)
	return nil
}

func GetChallenge(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	challengeID := re.Request.PathValue("id")

	// Fetch the challenge from the database
	challenge := Challenge{}
	err := app.DB().NewQuery("SELECT * FROM challenges WHERE id = {:id}").Bind(dbx.Params{"id": challengeID}).One(&challenge)
	if err != nil {
		return apis.NewInternalServerError("Could not get challenge", err)
	}

	jsonData, err := json.Marshal(challenge)
	if err != nil {
		return apis.NewInternalServerError("Failed to serialize challenge", err)
	}

	re.Response.WriteHeader(http.StatusOK)
	re.Response.Write(jsonData)
	return nil
}

func CreateQuestion(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	rawQuestion := RawQuestion{}
	challengeID := re.Request.PathValue("id")
	if err := json.NewDecoder(re.Request.Body).Decode(&rawQuestion); err != nil {
		return apis.NewBadRequestError("Invalid request body", err)
	}

	// Insert the new question into the database
	_, err := app.DB().NewQuery("INSERT INTO questions (challenge_id, name, description, flag, case_sensitive, category, flag_mask, hints) VALUES ({:challenge_id}, {:name}, {:description}, {:flag}, {:case_sensitive}, {:category}, {:flag_mask}, {:hints})").Bind(dbx.Params{
		"challenge_id":   challengeID,
		"name":           rawQuestion.Name,
		"description":    rawQuestion.Description,
		"flag":           rawQuestion.Flag,
		"case_sensitive": rawQuestion.CaseSensitive,
		"category":       rawQuestion.Category,
		"flag_mask":      rawQuestion.FlagMask,
		"hints":          rawQuestion.Hints,
	}).Execute()

	if err != nil {
		fmt.Printf("Error inserting question: %v\n", err)
		return apis.NewInternalServerError("Could not create question", err)
	}

	re.Response.WriteHeader(http.StatusCreated)
	return nil
}

func DeleteQuestion(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	questionID := re.Request.PathValue("id")

	// Delete the question from the database
	_, err := app.DB().NewQuery("DELETE FROM questions WHERE id = {:id}").Bind(dbx.Params{"id": questionID}).Execute()
	if err != nil {
		return apis.NewInternalServerError("Could not delete question", err)
	}

	re.Response.WriteHeader(http.StatusNoContent)
	return nil
}

func GetQuestion(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	questionID := re.Request.PathValue("id")

	// Fetch the question from the database
	question := Question{}
	err := app.DB().NewQuery("SELECT * FROM questions WHERE id = {:id}").Bind(dbx.Params{"id": questionID}).One(&question)
	if err != nil {
		return apis.NewInternalServerError("Could not get question", err)
	}

	jsonData, err := json.Marshal(question)
	if err != nil {
		return apis.NewInternalServerError("Failed to serialize question", err)
	}

	re.Response.WriteHeader(http.StatusOK)
	re.Response.Write(jsonData)
	return nil
}
