package views

import (
	"fmt"
	"hCTF/internal/routes"
	"hCTF/internal/templates"
	"log"
	"net/http"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/template"
)

// RenderHome renders the home page
func RenderHome(re *core.RequestEvent, templateRegistry *template.Registry) error {
	// Render home page in HTML
	html, err := templateRegistry.LoadFS(templates.TemplateFS, "html/home.html", "html/navbar.html", "html/head.html").Render(nil)
	if err != nil {
		log.Printf("Template error details: %v", err)
		return apis.NewBadRequestError("Template error", err)
	}
	return re.HTML(http.StatusOK, html)
}

// RenderChallenges renders the challenges page
func RenderChallenges(app *pocketbase.PocketBase, re *core.RequestEvent, templateRegistry *template.Registry) error {
	// Load challenges from database
	rawQuestions := []routes.RawQuestion{}
	questions := []routes.Question{}

	err := app.DB().NewQuery("SELECT * FROM questions").All(&rawQuestions)
	if err != nil {
		log.Printf("Database error details: %v", err)
		return apis.NewInternalServerError("Database error", err)
	}

	fmt.Printf("Questions: %v\n", rawQuestions)

	for _, rawQuestion := range rawQuestions {
		// Set the flag mask for each question
		question := routes.SetMask(&rawQuestion)
		questions = append(questions, question)
	}

	// Render challenges in HTML
	html, err := templateRegistry.LoadFS(
		templates.TemplateFS,
		"html/challenges.html",
		"html/challenge-card.html",
		"html/navbar.html",
		"html/head.html").Render(map[string]interface{}{"Questions": questions})
	if err != nil {
		log.Printf("Template error details: %v", err)
		return apis.NewBadRequestError("Template error", err)
	}
	return re.HTML(http.StatusOK, html)
}

// RenderLearn renders the learn page
func RenderLearn(re *core.RequestEvent, templateRegistry *template.Registry) error {
	return apis.NewNotFoundError("Learn page not found", nil)
}
