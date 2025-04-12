package api

import (
	"hCTF/internal/routes"
	"hCTF/internal/views"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/template"
)

// RegisterViewRoutes registers all view-related routes
func RegisterViewRoutes(se *core.ServeEvent, app *pocketbase.PocketBase, templateRegistry *template.Registry) {
	se.Router.GET("/", func(re *core.RequestEvent) error {
		return views.RenderHome(re, templateRegistry)
	})

	se.Router.GET("/challenges", func(re *core.RequestEvent) error {
		return views.RenderChallenges(app, re, templateRegistry)
	})

	se.Router.GET("/learn", func(re *core.RequestEvent) error {
		return views.RenderLearn(re, templateRegistry)
	})

}

// RegisterAPIRoutes registers all API-related routes
func RegisterAPIRoutes(se *core.ServeEvent, app *pocketbase.PocketBase, templateRegistry *template.Registry) {

	challengesGuest := se.Router.Group("/api/v1/challenges")
	challengesGuest.GET("/", func(re *core.RequestEvent) error {
		return routes.ListChallenges(app, re)
	})
	challengesGuest.GET("/{id}", func(re *core.RequestEvent) error {
		return routes.GetChallenge(app, re)
	})

	// Create group of routes for challenge management
	challengesManagement := se.Router.Group("/api/v1/challenges").Bind(apis.RequireAuth("_superusers"))
	challengesManagement.POST("/", func(re *core.RequestEvent) error {
		return routes.CreateChallenge(app, re)
	})
	challengesManagement.DELETE("/{id}", func(re *core.RequestEvent) error {
		return routes.DeleteChallenge(app, re)
	})
	challengesManagement.PUT("/{id}", func(re *core.RequestEvent) error {
		return routes.UpdateChallenge(app, re)
	})

	login := se.Router.Group("/api/v1/login").Bind(apis.RequireGuestOnly())

	login.POST("/sign-up", func(re *core.RequestEvent) error {
		return routes.SignUp(app, re)
	})

	se.Router.GET("/api/v1/challenges/{id}/questions", func(re *core.RequestEvent) error {
		return routes.ListChallengeQuestions(app, re)
	})

	se.Router.POST("/api/v1/questions/{id}/answers", func(re *core.RequestEvent) error {
		return routes.SubmitQuestionAnswer(app, templateRegistry, re)
	})

	se.Router.GET("/api/v1/questions/{id}", func(re *core.RequestEvent) error {
		return routes.GetQuestion(app, re)
	})

	se.Router.POST("/api/v1/challenges/{id}/questions", func(re *core.RequestEvent) error {
		return routes.CreateQuestion(app, re)
	}).Bind(apis.RequireAuth("_superusers"))

	se.Router.PUT("/api/v1/questions/{id}", func(re *core.RequestEvent) error {
		return routes.UpdateQuestion(app, re)
	}).Bind(apis.RequireAuth("_superusers"))

	se.Router.DELETE("/api/v1/questions/{id}", func(re *core.RequestEvent) error {
		return routes.DeleteQuestion(app, re)
	}).Bind(apis.RequireAuth("_superusers"))
}

func RegisterHooks(se *core.ServeEvent, app *pocketbase.PocketBase) {

	// These 2 will guarantee that if a non-admin creates a team, the team will be added to them
	app.OnRecordCreateRequest("teams").BindFunc(func(e *core.RecordRequestEvent) error {
		if e.Auth.IsSuperuser() {
			e.Record.Set("created_by_admin", e.Auth.Id)
		} else {
			e.Record.Set("created_by", e.Auth.Id)
			e.Record.Set("owner", e.Auth.Id)
		}

		return e.Next()
	})
	app.OnRecordAfterCreateSuccess("teams").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("created_by_admin") != "" {
			return e.Next()
		}

		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			_ = app.Delete(e.Record)
			return apis.NewInternalServerError("failed to find users collection", err)
		}

		user, err := app.FindRecordById(users, e.Record.GetString("created_by"))
		if err != nil {
			_ = app.Delete(e.Record)
			return apis.NewInternalServerError("creator not found in users or superusers", err)
		}

		user.Set("team", e.Record.GetString("id"))

		err = app.Save(user)
		if err != nil {
			_ = app.Delete(e.Record)
			return apis.NewInternalServerError("could not attach team to user", err)
		}

		return e.Next()
	})

	// Remove fields that are deemed not important or useful in most scenarios
	app.OnRecordEnrich("teams").BindFunc(func(e *core.RecordEnrichEvent) error {
		e.Record.Hide("collectionName", "updated", "created_by", "created_by_admin")

		return e.Next()
	})

	// Sets the "Set-Cookie" response header, so that the browser retains the token as a Cookie
	app.OnRecordAuthRequest().BindFunc(func(e *core.RecordAuthRequestEvent) error {
		e.Response.Header().Set("Set-Cookie", "Authorization="+e.Token)

		return e.Next()
	})
}
