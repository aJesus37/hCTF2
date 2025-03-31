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
	"github.com/pocketbase/pocketbase/tools/types"
)

type Result struct {
	Success     bool `json:"success"`
	RawQuestion *core.Record
}

type Challenge struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Tags        types.JSONArray[string] `json:"tags"`
}

// ValidateFlag validates a submitted flag
func SubmitQuestionAnswer(app *pocketbase.PocketBase, templateRegistry *template.Registry, re *core.RequestEvent) error {
	flag := re.Request.FormValue("flag")
	questionId := re.Request.PathValue("id")

	questions, err := app.FindCollectionByNameOrId("questions")
	if err != nil {
		return apis.NewInternalServerError("could not find table", err)
	}

	question, err := app.FindRecordById(questions, questionId)
	if err != nil {
		return apis.NewBadRequestError("invalid question id", err)
	}

	result := Result{
		Success:     false,
		RawQuestion: question,
	}

	// Check if the provided flag matches the stored flag
	if question.GetBool("case_sensitive") {
		fmt.Printf("Case sensitive flag check: %s vs %s\n", flag, question.GetString("flag"))
		if flag == question.GetString("flag") {
			result = Result{
				Success:     true,
				RawQuestion: question,
			}
		}
	} else {
		fmt.Printf("Case insensitive flag check: %s vs %s\n", strings.ToLower(flag), strings.ToLower(question.GetString("flag")))
		if strings.EqualFold(flag, question.GetString("flag")) {
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
	query := re.Request.URL.Query()

	questions, err := app.FindCollectionByNameOrId("questions")
	if err != nil {
		return apis.NewInternalServerError("could not find table", err)
	}

	challengeQuestions, err := app.FindRecordsByFilter(questions, "challenge_id = {:challenge_id}", "", -1, 0, dbx.Params{"challenge_id": challengeID})
	if err != nil {
		return apis.NewBadRequestError("could not find challenge", err)
	}

	fmt.Printf("Query: %v, == flag %v\n", query.Get("q"), query.Get("q") == "flag")
	fmt.Printf("SuperUser? %v\n", re.HasSuperuserAuth())

	for _, challenge := range challengeQuestions {

		challenge.Hide("collectionId", "collectionName")
		if !(query.Get("q") == "flag") || !re.HasSuperuserAuth() {
			challenge.Hide("flag")
		}
	}

	re.JSON(http.StatusOK, challengeQuestions)
	return nil
}

func CreateChallenge(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	challenge := Challenge{}
	if err := json.NewDecoder(re.Request.Body).Decode(&challenge); err != nil {
		return apis.NewBadRequestError("Invalid request body", err)
	}

	challenges, err := app.FindCollectionByNameOrId("challenges")
	if err != nil {
		return apis.NewInternalServerError("could not find table", err)
	}

	record := core.NewRecord(challenges)

	record.Set("name", challenge.Name)
	record.Set("description", challenge.Description)
	record.Set("tags", challenge.Tags)

	err = app.Save(record)
	if err != nil {
		return apis.NewInternalServerError("could not save challenge", err)
	}

	re.Response.WriteHeader(http.StatusCreated)
	return nil
}

func DeleteChallenge(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	challengeID := re.Request.PathValue("id")

	challenges, err := app.FindCollectionByNameOrId("challenges")
	if err != nil {
		return apis.NewInternalServerError("could not find table", err)
	}

	record, err := app.FindRecordById(challenges, challengeID)
	if err != nil {
		return apis.NewBadRequestError("invalid challenge id", err)
	}

	err = app.Delete(record)
	if err != nil {
		return apis.NewInternalServerError("could not delete challenge", err)
	}

	re.Response.WriteHeader(http.StatusNoContent)
	return nil
}

func GetChallenge(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	record, err := app.FindRecordById("challenges", re.Request.PathValue("id"))
	if err != nil {
		return apis.NewBadRequestError("Invalid challenge ID", err)
	}

	record = record.Hide("collectionId", "collectionName")
	re.JSON(http.StatusOK, record)
	return nil
}

func CreateQuestion(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	rawQuestion := RawQuestion{}
	challengeID := re.Request.PathValue("id")
	if err := json.NewDecoder(re.Request.Body).Decode(&rawQuestion); err != nil {
		return apis.NewBadRequestError("Invalid request body", err)
	}

	collection, err := app.FindCollectionByNameOrId("questions")
	if err != nil {
		return err
	}

	record := core.NewRecord(collection)
	record.Load(map[string]any{
		"name":           rawQuestion.Name,
		"description":    rawQuestion.Description,
		"flag":           rawQuestion.Flag,
		"case_sensitive": rawQuestion.CaseSensitive,
		"category":       rawQuestion.Category,
		"flag_mask":      rawQuestion.FlagMask,
		"hints":          rawQuestion.Hints,
		"challenge_id":   challengeID,
	})

	err = app.Save(record)
	if err != nil {
		fmt.Printf("Error inserting question: %v\n", err)
		return apis.NewInternalServerError("Could not create question", err)
	}

	re.Response.WriteHeader(http.StatusCreated)
	return nil
}

func DeleteQuestion(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	questionID := re.Request.PathValue("id")

	questions, err := app.FindCollectionByNameOrId("questions")
	if err != nil {
		return apis.NewInternalServerError("Could not find table", err)
	}

	question, err := app.FindRecordById(questions, questionID)
	if err != nil {
		return apis.NewBadRequestError("Invalid question ID", err)
	}

	err = app.Delete(question)
	if err != nil {
		return apis.NewInternalServerError("Failed to remove question", err)
	}

	re.Response.WriteHeader(http.StatusNoContent)
	return nil
}

func GetQuestion(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	questionID := re.Request.PathValue("id")

	questions, err := app.FindCollectionByNameOrId("questions")
	if err != nil {
		return apis.NewInternalServerError("Failed to get table", err)
	}

	record, err := app.FindRecordById(questions, questionID)
	if err != nil {
		return apis.NewBadRequestError("Invalid question ID", err)
	}

	record = record.Hide("collectionId", "collectionName", "flag")

	re.JSON(http.StatusOK, record)
	return nil
}

func UpdateQuestion(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	questionID := re.Request.PathValue("id")
	rawQuestion := RawQuestion{}

	questions, err := app.FindCollectionByNameOrId("questions")
	if err != nil {
		return apis.NewInternalServerError("Failed to get table", err)
	}

	if err := json.NewDecoder(re.Request.Body).Decode(&rawQuestion); err != nil {
		return apis.NewBadRequestError("Invalid request body", err)
	}

	record, err := app.FindRecordById(questions, questionID)
	if err != nil {
		return apis.NewBadRequestError("Invalid question ID", err)
	}

	record.Set("name", rawQuestion.Name)
	record.Set("description", rawQuestion.Description)
	record.Set("flag", rawQuestion.Flag)
	record.Set("case_sensitive", rawQuestion.CaseSensitive)
	record.Set("category", rawQuestion.Category)
	record.Set("flag_mask", rawQuestion.FlagMask)
	record.Set("hints", rawQuestion.Hints)

	err = app.Save(record)
	if err != nil {
		fmt.Printf("Error updating question: %v\n", err)
		return apis.NewInternalServerError("Could not update question", err)
	}

	re.Response.WriteHeader(http.StatusNoContent)
	return nil
}

func UpdateChallenge(app *pocketbase.PocketBase, re *core.RequestEvent) error {
	challenge := Challenge{}
	challengeID := re.Request.PathValue("id")

	challenges, err := app.FindCollectionByNameOrId("challenges")
	if err != nil {
		return apis.NewInternalServerError("Failed to get table", err)
	}

	if err := json.NewDecoder(re.Request.Body).Decode(&challenge); err != nil {
		return apis.NewBadRequestError("Invalid request body", err)
	}

	record, err := app.FindRecordById(challenges, challengeID)
	if err != nil {
		return apis.NewBadRequestError("Invalid challenge ID", err)
	}

	record.Set("name", challenge.Name)
	record.Set("description", challenge.Description)
	record.Set("tags", challenge.Tags)

	err = app.Save(record)
	if err != nil {
		fmt.Printf("Error updating challenge: %v\n", err)
		return apis.NewInternalServerError("Could not update challenge", err)
	}

	re.Response.WriteHeader(http.StatusNoContent)
	return nil
}
