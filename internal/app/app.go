package app

import (
	"hCTF/internal/api"
	"hCTF/internal/config"
	"hCTF/internal/static"
	_ "hCTF/migrations"
	"io/fs"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/template"
)

// App encapsulates the application
type App struct {
	pb               *pocketbase.PocketBase
	templateRegistry *template.Registry
	staticFS         fs.FS
	config           *config.Config
}

// New creates a new application instance
func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	app := &App{
		pb:               pocketbase.New(),
		templateRegistry: template.NewRegistry(),
		staticFS:         static.StaticFS,
		config:           cfg,
	}

	app.setupRoutes()

	migratecmd.MustRegister(app.pb, app.pb.RootCmd, migratecmd.Config{})

	return app, nil
}

// Start starts the application
func (a *App) Start() error {
	return a.pb.Start()
}

// setupRoutes configures all application routes
func (a *App) setupRoutes() {
	a.pb.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Register static assets
		se.Router.GET("/static/{path...}", apis.Static(a.staticFS, true))

		// Register view routes
		api.RegisterViewRoutes(se, a.pb, a.templateRegistry)

		// Register API routes
		api.RegisterAPIRoutes(se, a.pb, a.templateRegistry)

		// Register Hooks
		api.RegisterHooks(se, a.pb)

		return se.Next()
	})
}
