package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"

	"github.com/ajesus37/hCTF2/internal/auth"
	"github.com/ajesus37/hCTF2/internal/database"
	"github.com/ajesus37/hCTF2/internal/email"
	"github.com/ajesus37/hCTF2/internal/handlers"
	"github.com/ajesus37/hCTF2/internal/models"
	"github.com/ajesus37/hCTF2/internal/ratelimit"
	"github.com/ajesus37/hCTF2/internal/scorerecorder"
	"github.com/ajesus37/hCTF2/internal/storage"
	"github.com/ajesus37/hCTF2/internal/telemetry"
	"github.com/ajesus37/hCTF2/internal/utils"
)

// Assets holds the embedded file systems that must be provided by main
// (because //go:embed paths are relative to the source file location).
type Assets struct {
	TemplatesFS fs.ReadFileFS
	StaticFS    fs.FS
	OpenapiSpec fs.ReadFileFS
}

var assets Assets

// SetAssets must be called from main before Execute.
func SetAssets(a Assets) {
	assets = a
}

// serve flags
var (
	servePort             int
	serveDB               string
	serveAdminEmail       string
	serveAdminPass        string
	serveMOTD             string
	servePrometheus       bool
	serveOTLP             string
	serveSMTPHost         string
	serveSMTPPort         int
	serveSMTPFrom         string
	serveSMTPUser         string
	serveSMTPPass         string
	serveBaseURL          string
	serveJWTSecret        string
	serveDev              bool
	serveCORSOrigins      string
	serveSubmitRateLimit  int
	serveUploadDir        string
	serveUmamiScriptURL   string
	serveUmamiWebsiteID   string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the hCTF2 HTTP server",
	Long:  "Start the hCTF2 HTTP server with all configured options.",
	RunE:  runServe,
}

func init() {
	f := serveCmd.Flags()
	f.IntVar(&servePort, "port", 8090, "Server port")
	f.StringVar(&serveDB, "db", "./hctf2.db", "Database path")
	f.StringVar(&serveAdminEmail, "admin-email", "", "Admin email for first-time setup")
	f.StringVar(&serveAdminPass, "admin-password", "", "Admin password for first-time setup")
	f.StringVar(&serveMOTD, "motd", "", "Message of the Day displayed below login form")
	f.BoolVar(&servePrometheus, "metrics", false, "Enable Prometheus /metrics endpoint")
	f.StringVar(&serveOTLP, "otel-otlp-endpoint", "", "OTLP exporter endpoint (e.g. localhost:4318)")
	f.StringVar(&serveSMTPHost, "smtp-host", "", "SMTP server host")
	f.IntVar(&serveSMTPPort, "smtp-port", 587, "SMTP server port")
	f.StringVar(&serveSMTPFrom, "smtp-from", "", "SMTP from address")
	f.StringVar(&serveSMTPUser, "smtp-user", "", "SMTP username")
	f.StringVar(&serveSMTPPass, "smtp-password", "", "SMTP password")
	f.StringVar(&serveBaseURL, "base-url", "http://localhost:8090", "Base URL for links in emails")
	f.StringVar(&serveJWTSecret, "jwt-secret", getEnv("JWT_SECRET", ""), "JWT signing secret (min 32 chars, required in production)")
	f.BoolVar(&serveDev, "dev", false, "Enable development mode (allows default JWT secret, relaxed security)")
	f.StringVar(&serveCORSOrigins, "cors-origins", getEnv("CORS_ORIGINS", ""), "Comma-separated list of allowed CORS origins (empty = same-origin only)")
	f.IntVar(&serveSubmitRateLimit, "submission-rate-limit", 5, "Max flag submissions per minute per user (0 = unlimited)")
	f.StringVar(&serveUploadDir, "upload-dir", "./uploads", "Directory for file uploads")
	f.StringVar(&serveUmamiScriptURL, "umami-script-url", getEnv("UMAMI_SCRIPT_URL", ""), "Umami analytics script URL (e.g. https://umami.example.com/script.js)")
	f.StringVar(&serveUmamiWebsiteID, "umami-website-id", getEnv("UMAMI_WEBSITE_ID", ""), "Umami analytics website ID")

	rootCmd.AddCommand(serveCmd)
}

// getEnv returns an env var or a fallback value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// firstNonEmpty returns the first non-empty string from the provided values.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Server holds all application dependencies.
// Fields are exported so that tests in package main can access them via type alias.
type Server struct {
	DB             *database.DB
	Templates      *template.Template
	AuthH          *handlers.AuthHandler
	ChallengeH     *handlers.ChallengeHandler
	ChallengeFileH *handlers.ChallengeFileHandler
	ScoreboardH    *handlers.ScoreboardHandler
	TeamH          *handlers.TeamHandler
	HintH          *handlers.HintHandler
	SQLH           *handlers.SQLHandler
	ProfileH       *handlers.ProfileHandler
	SettingsH      *handlers.SettingsHandler
	ImportExportH  *handlers.ImportExportHandler
	CompetitionH   *handlers.CompetitionHandler
	Motd            string
	SubmitLimiter   *ratelimit.Limiter
	Storage         storage.Storage
	ScoreRecorder   *scorerecorder.Recorder
	UmamiScriptURL  string
	UmamiWebsiteID  string
}

// customFileHandler wraps the file server to set proper content types for WASM and workers.
type customFileHandler struct {
	fs http.Handler
}

func (h *customFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, ".wasm") {
		w.Header().Set("Content-Type", "application/wasm")
	} else if strings.HasSuffix(r.URL.Path, ".worker.js") {
		w.Header().Set("Content-Type", "application/javascript")
	}
	h.fs.ServeHTTP(w, r)
}

// corsMiddleware returns a middleware that handles CORS based on configuration.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowOrigin := ""
			if len(allowedOrigins) == 0 {
				if origin == "" {
					allowOrigin = "*"
				}
			} else {
				for _, allowed := range allowedOrigins {
					if allowed == "*" || allowed == origin {
						allowOrigin = origin
						break
					}
				}
			}

			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func runServe(_ *cobra.Command, _ []string) error {
	// JWT secret
	jwtSecretValue := serveJWTSecret
	if jwtSecretValue == "" || jwtSecretValue == "change-this-secret-in-production" {
		if serveDev {
			log.Println("WARNING: Using default JWT secret in development mode. DO NOT use in production!")
			jwtSecretValue = "change-this-secret-in-production"
		} else {
			log.Fatal("ERROR: JWT secret is required. Use --dev for development, or set --jwt-secret flag, JWT_SECRET env var. The secret must be at least 32 characters.")
		}
	}

	if err := auth.SetJWTSecret(jwtSecretValue); err != nil {
		log.Fatalf("ERROR: Invalid JWT secret: %v", err)
	}

	// Email service
	emailSvc := email.NewService(email.Config{
		Host:     firstNonEmpty(serveSMTPHost, os.Getenv("SMTP_HOST")),
		Port:     serveSMTPPort,
		From:     firstNonEmpty(serveSMTPFrom, os.Getenv("SMTP_FROM")),
		Username: firstNonEmpty(serveSMTPUser, os.Getenv("SMTP_USER")),
		Password: firstNonEmpty(serveSMTPPass, os.Getenv("SMTP_PASSWORD")),
	})

	if !emailSvc.IsConfigured() {
		log.Println("Warning: SMTP not configured. Password reset links will be logged to console.")
	}

	// Telemetry
	cleanupTelemetry, err := telemetry.Init(telemetry.Config{
		ServiceName:          "hctf2",
		ServiceVersion:       "0.5.0",
		Environment:          os.Getenv("ENVIRONMENT"),
		EnableStdoutExporter: os.Getenv("OTEL_EXPORTER_STDOUT") == "true",
		EnablePrometheus:     servePrometheus || os.Getenv("OTEL_METRICS_PROMETHEUS") == "true",
		OTLPEndpoint:         firstNonEmpty(serveOTLP, os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize telemetry: %v", err)
	} else {
		defer cleanupTelemetry()
	}

	// Database
	db, err := database.New(serveDB)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Admin user creation
	if serveAdminEmail != "" && serveAdminPass != "" {
		createAdminUser(db, serveAdminEmail, serveAdminPass)
	}

	// Parse templates from the embedded FS provided by main
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"markdown":      utils.RenderMarkdown,
		"stripMarkdown": utils.StripMarkdown,
		"safeHTML":      func(s string) template.HTML { return template.HTML(s) },
		"mul":           func(a, b int) int { return a * b },
		"div":           func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"difficultyColor": func(name string) string {
			d, err := db.GetDifficultyByName(name)
			if err != nil {
				return "text-gray-400"
			}
			return d.TextColor
		},
		"difficultyBadge": func(name string) string {
			d, err := db.GetDifficultyByName(name)
			if err != nil {
				return "bg-gray-600 text-gray-100"
			}
			return d.Color
		},
		"splitCategories": func(csv string) []string {
			parts := strings.Split(csv, ",")
			var result []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					result = append(result, p)
				}
			}
			return result
		},
	}).ParseFS(assets.TemplatesFS, "internal/views/templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Storage
	stor := storage.NewLocal(serveUploadDir, "/uploads")

	// Score recorder
	recorder := scorerecorder.New(db, 15*time.Minute, 20)

	// Server
	s := &Server{
		DB:             db,
		Templates:      tmpl,
		AuthH:          handlers.NewAuthHandler(db, emailSvc, serveBaseURL),
		ChallengeH:     handlers.NewChallengeHandler(db, nil, stor, recorder),
		ChallengeFileH: handlers.NewChallengeFileHandler(db, stor),
		ScoreboardH:    handlers.NewScoreboardHandler(db, recorder),
		TeamH:          handlers.NewTeamHandler(db),
		HintH:          handlers.NewHintHandler(db),
		SQLH:           handlers.NewSQLHandler(db),
		ProfileH:       handlers.NewProfileHandler(db),
		SettingsH:      handlers.NewSettingsHandler(db),
		ImportExportH:  handlers.NewImportExportHandler(db),
		CompetitionH:   handlers.NewCompetitionHandler(db),
		ScoreRecorder:  recorder,
		Motd:           serveMOTD,
		Storage:        stor,
		UmamiScriptURL: serveUmamiScriptURL,
		UmamiWebsiteID: serveUmamiWebsiteID,
	}

	if serveSubmitRateLimit > 0 {
		s.SubmitLimiter = ratelimit.New(serveSubmitRateLimit, time.Minute)
		s.ChallengeH = handlers.NewChallengeHandler(db, s.SubmitLimiter, stor, recorder)
	}

	s.ScoreRecorder.Start()

	// Competition lifecycle watcher
	go func() {
		db.TickCompetitionLifecycle()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			db.TickCompetitionLifecycle()
		}
	}()

	// Parse CORS origins
	var allowedOrigins []string
	if serveCORSOrigins != "" {
		allowedOrigins = strings.Split(serveCORSOrigins, ",")
		for i := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
		}
	}

	// Router
	r := chi.NewRouter()
	r.Use(corsMiddleware(allowedOrigins))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/static/") {
				w.Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Use(telemetry.Middleware)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(auth.AuthMiddleware)

	r.Handle("/static/*", http.StripPrefix("/static/", &customFileHandler{
		fs: http.FileServer(http.FS(assets.StaticFS)),
	}))

	r.Get("/healthz", s.HandleHealthz)
	r.Get("/readyz", s.HandleReadyz)

	if servePrometheus || os.Getenv("OTEL_METRICS_PROMETHEUS") == "true" {
		r.Handle("/metrics", telemetry.PrometheusHandler())
	}

	// Public routes
	r.Get("/", s.HandleIndex)
	r.Get("/challenges", s.HandleChallenges)
	r.Get("/challenges/{id}", s.HandleChallengeDetail)
	r.Get("/scoreboard", s.HandleScoreboard)
	r.Get("/competitions", s.HandleCompetitionList)
	r.Get("/competitions/{id}", s.HandleCompetitionDetail)
	r.Get("/submissions", s.HandleSubmissionsPage)
	r.Get("/sql", s.HandleSQL)
	r.Get("/login", s.HandleLoginPage)
	r.Get("/register", s.HandleRegisterPage)
	r.Get("/forgot-password", s.HandleForgotPasswordPage)
	r.Get("/reset-password", s.HandleResetPasswordPage)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Get("/teams", s.HandleTeams)
	})

	r.Get("/profile", s.HandleOwnProfile)
	r.Get("/users/{id}", s.HandleUserProfile)
	r.Get("/teams/{id}/profile", s.HandleTeamProfile)

	r.Group(func(r chi.Router) {
		r.Use(s.RequireAdmin)
		r.Get("/admin", s.HandleAdminDashboard)
		r.Get("/admin/challenges/{id}/edit", s.HandleEditChallenge)
		r.Get("/admin/challenges/{id}/view", s.HandleViewChallenge)
		r.Get("/admin/questions/{id}/edit", s.HandleEditQuestion)
		r.Get("/admin/questions/{id}/view", s.HandleViewQuestion)
	})

	// API routes - Auth
	r.Post("/api/auth/register", s.AuthH.Register)
	r.Post("/api/auth/login", s.AuthH.Login)
	r.Post("/api/auth/forgot-password", s.AuthH.ForgotPassword)
	r.Post("/api/auth/reset-password", s.AuthH.ResetPassword)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/auth/logout", s.AuthH.Logout)
	})

	r.Get("/api/challenges", s.ChallengeH.ListChallenges)
	r.Get("/api/challenges/{id}", s.ChallengeH.GetChallenge)
	r.Get("/api/challenges-dropdown", s.ChallengeH.GetChallengesDropdown)
	r.Get("/api/questions-dropdown", s.ChallengeH.GetQuestionsDropdown)
	r.Get("/api/questions/{questionId}/next-hint-order", s.ChallengeH.GetNextHintOrder)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/questions/{id}/submit", s.ChallengeH.SubmitFlag)
		r.Get("/api/questions/{id}/solution", s.ChallengeH.GetQuestionSolution)
	})

	r.Get("/api/teams", s.TeamH.ListTeams)
	r.Get("/api/teams/{id}", s.TeamH.GetTeam)
	r.Get("/api/teams/scoreboard", s.TeamH.GetTeamScoreboard)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/competitions/{id}/register", s.CompetitionH.RegisterTeam)
		r.Post("/api/teams", s.TeamH.CreateTeam)
		r.Post("/api/teams/join/{invite_id}", s.TeamH.JoinTeam)
		r.Post("/api/teams/leave", s.TeamH.LeaveTeam)
		r.Post("/api/teams/transfer-ownership", s.TeamH.TransferOwnership)
		r.Post("/api/teams/disband", s.TeamH.DisbandTeam)
		r.Post("/api/teams/regenerate-invite", s.TeamH.RegenerateInviteCode)
		r.Post("/api/teams/invite-permission", s.TeamH.UpdateInvitePermission)
	})

	r.Get("/api/questions/{questionId}/hints", s.HintH.GetHints)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/hints/{id}/unlock", s.HintH.UnlockHint)
	})

	r.With(auth.RequireAuth).Get("/uploads/*", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "*")
		filename = filepath.Base(filename)
		http.ServeFile(w, r, filepath.Join(serveUploadDir, filename))
	})

	r.Group(func(r chi.Router) {
		r.Use(s.RequireAdmin)
		r.Post("/api/admin/challenges", s.ChallengeH.CreateChallenge)
		r.Put("/api/admin/challenges/{id}", s.ChallengeH.UpdateChallenge)
		r.Delete("/api/admin/challenges/{id}", s.ChallengeH.DeleteChallenge)
		r.Post("/api/admin/challenges/{id}/upload", s.ChallengeH.UploadChallengeFile)
		r.Post("/api/admin/challenges/{id}/file-url", s.ChallengeH.SetChallengeFileURLHandler)
		r.Delete("/api/admin/challenges/{id}/file", s.ChallengeH.DeleteChallengeFile)
		r.Get("/api/admin/challenges/{id}/files", s.ChallengeFileH.ListFiles)
		r.Post("/api/admin/challenges/{id}/files", s.ChallengeFileH.UploadFile)
		r.Post("/api/admin/challenges/{id}/files/url", s.ChallengeFileH.AddExternalURL)
		r.Post("/api/admin/challenges/{id}/files/batch", s.ChallengeFileH.BatchUpload)
		r.Delete("/api/admin/challenge-files/{file_id}", s.ChallengeFileH.DeleteFile)
		r.Post("/api/admin/questions", s.ChallengeH.CreateQuestion)
		r.Get("/api/admin/questions/{id}", s.ChallengeH.GetQuestion)
		r.Put("/api/admin/questions/{id}", s.ChallengeH.UpdateQuestion)
		r.Delete("/api/admin/questions/{id}", s.ChallengeH.DeleteQuestion)
		r.Post("/api/admin/hints", s.ChallengeH.CreateHint)
		r.Get("/api/admin/hints/{id}", s.ChallengeH.GetHint)
		r.Put("/api/admin/hints/{id}", s.ChallengeH.UpdateHint)
		r.Delete("/api/admin/hints/{id}", s.ChallengeH.DeleteHint)
		r.Post("/api/admin/categories", s.SettingsH.CreateCategory)
		r.Put("/api/admin/categories/{id}", s.SettingsH.UpdateCategory)
		r.Delete("/api/admin/categories/{id}", s.SettingsH.DeleteCategory)
		r.Post("/api/admin/difficulties", s.SettingsH.CreateDifficulty)
		r.Put("/api/admin/difficulties/{id}", s.SettingsH.UpdateDifficulty)
		r.Delete("/api/admin/difficulties/{id}", s.SettingsH.DeleteDifficulty)
		r.Get("/api/admin/custom-code", s.SettingsH.GetCustomCode)
		r.Put("/api/admin/custom-code", s.SettingsH.UpdateCustomCode)
		r.Get("/api/admin/users", s.SettingsH.ListUsers)
		r.Put("/api/admin/users/{id}/admin", s.SettingsH.UpdateUserAdmin)
		r.Delete("/api/admin/users/{id}", s.SettingsH.DeleteUser)
		r.Post("/api/admin/settings/freeze", s.SettingsH.SetScoreFreeze)
		r.Post("/api/admin/settings/admin-visibility", s.SettingsH.SetAdminVisibility)
		r.Get("/api/admin/export", s.ImportExportH.ExportChallenges)
		r.Post("/api/admin/import", s.ImportExportH.ImportChallenges)
		r.Get("/api/admin/config/export", s.ImportExportH.ExportConfig)
		r.Post("/api/admin/config/import", s.ImportExportH.ImportConfig)
		r.Post("/api/admin/scoreboard/force-record", s.ScoreboardH.ForceScoreRecord)
		r.Post("/api/admin/competitions", s.CompetitionH.CreateCompetition)
		r.Put("/api/admin/competitions/{id}", s.CompetitionH.UpdateCompetition)
		r.Delete("/api/admin/competitions/{id}", s.CompetitionH.DeleteCompetition)
		r.Post("/api/admin/competitions/{id}/challenges", s.CompetitionH.AddChallenge)
		r.Delete("/api/admin/competitions/{id}/challenges/{cid}", s.CompetitionH.RemoveChallenge)
		r.Get("/api/admin/competitions/{id}/teams", s.CompetitionH.ListTeams)
		r.Post("/api/admin/competitions/{id}/force-start", s.CompetitionH.ForceStart)
		r.Post("/api/admin/competitions/{id}/force-end", s.CompetitionH.ForceEnd)
		r.Post("/api/admin/competitions/{id}/freeze", s.CompetitionH.SetFreeze)
		r.Post("/api/admin/competitions/{id}/blackout", s.CompetitionH.SetBlackout)
		r.Get("/api/categories-checkboxes", s.HandleCategoriesCheckboxes)
		r.Get("/api/difficulties-dropdown", s.HandleDifficultiesDropdown)
	})

	r.Get("/api/sql/snapshot", s.SQLH.GetSnapshot)
	r.Get("/api/openapi.yaml", s.HandleOpenAPISpec)
	r.Get("/docs", s.HandleDocsPage)
	r.Get("/api/categories", s.SettingsH.ListCategories)
	r.Get("/api/difficulties", s.SettingsH.ListDifficulties)
	r.Get("/api/scoreboard", s.ScoreboardH.GetScoreboard)
	r.Get("/api/scoreboard/evolution", s.ScoreboardH.GetScoreEvolution)
	r.Get("/api/ctftime", s.ScoreboardH.CTFtimeExport)
	r.Get("/api/competitions", s.CompetitionH.ListCompetitions)
	r.Get("/api/competitions/{id}", s.CompetitionH.GetCompetition)
	r.Get("/api/competitions/{id}/scoreboard", s.CompetitionH.GetScoreboard)
	r.Get("/api/competitions/{id}/scoreboard/evolution", s.CompetitionH.GetCompetitionScoreEvolution)
	r.Get("/api/competitions/submissions", s.CompetitionH.GetGlobalSubmissionFeed)
	r.Get("/api/competitions/{id}/submissions", s.CompetitionH.GetSubmissionFeed)
	r.Get("/api/users/me/profile", s.HandleAPIUserProfile)
	r.Get("/api/users/{id}/profile", s.HandleAPIUserProfile)

	r.NotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.RenderError(w, 404, "Page Not Found", "The page you're looking for doesn't exist.")
	}))
	r.MethodNotAllowed(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.RenderError(w, 405, "Method Not Allowed", "The HTTP method used is not supported for this endpoint.")
	}))

	addr := fmt.Sprintf(":%d", servePort)
	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	shutdownDone := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("\nShutting down server...")
		s.ScoreRecorder.Stop()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		close(shutdownDone)
	}()

	log.Printf("Server starting on http://localhost%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	<-shutdownDone
	log.Println("Server stopped")
	return nil
}

func createAdminUser(db *database.DB, emailAddr, password string) {
	if _, err := db.GetUserByEmail(emailAddr); err == nil {
		log.Printf("Admin user already exists: %s", emailAddr)
		return
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	_, err = db.CreateUser(emailAddr, passwordHash, "Admin", true)
	if err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	log.Printf("Admin user created: %s", emailAddr)
}

// Template rendering helper
func (s *Server) Render(w http.ResponseWriter, name string, data interface{}) {
	if m, ok := data.(map[string]interface{}); ok {
		if s.UmamiScriptURL != "" && s.UmamiWebsiteID != "" {
			m["UmamiScriptURL"] = s.UmamiScriptURL
			m["UmamiWebsiteID"] = s.UmamiWebsiteID
		}
	}
	if err := s.Templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// renderError renders an error page with the given status code.
func (s *Server) RenderError(w http.ResponseWriter, statusCode int, title, message string) {
	w.WriteHeader(statusCode)
	data := map[string]interface{}{
		"Title":      title,
		"Page":       "error",
		"User":       nil,
		"StatusCode": statusCode,
		"Message":    message,
	}
	s.Render(w, "base.html", data)
}

// requireAdmin middleware with proper error page rendering.
func (s *Server) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserFromContext(r.Context())
		if claims == nil || !claims.IsAdmin {
			s.RenderError(w, 403, "Access Forbidden", "You don't have permission to access this page.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- Page handlers ----

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	challenges, _ := s.DB.GetChallengeCount()
	users, _ := s.DB.GetUserCount()
	solves, _ := s.DB.GetCorrectSubmissionCount()
	customCode, _ := s.DB.GetCustomCode("index")

	data := map[string]interface{}{
		"Title": "Home",
		"Page":  "index",
		"User":  auth.GetUserFromContext(r.Context()),
		"Stats": map[string]int{
			"Challenges": challenges,
			"Users":      users,
			"Solves":     solves,
		},
		"CustomCode": customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleChallenges(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	visibleOnly := claims == nil || !claims.IsAdmin

	challenges, err := s.DB.GetChallenges(visibleOnly)
	if err != nil {
		http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
		return
	}

	categories, _ := s.DB.GetAllCategories()
	difficulties, _ := s.DB.GetAllDifficulties()
	customCode, _ := s.DB.GetCustomCode("challenges")

	data := map[string]interface{}{
		"Title":        "Challenges",
		"Page":         "challenges",
		"User":         claims,
		"Challenges":   challenges,
		"Categories":   categories,
		"Difficulties": difficulties,
		"CustomCode":   customCode,
	}

	if claims != nil {
		completions, _ := s.DB.GetChallengeCompletionForUser(claims.UserID)
		data["Completions"] = completions
	} else {
		data["Completions"] = make(map[string]*database.ChallengeCompletion)
	}

	s.Render(w, "base.html", data)
}

func (s *Server) HandleChallengeDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	challenge, err := s.DB.GetChallengeByID(id)
	if err != nil {
		http.Error(w, "Challenge not found", http.StatusNotFound)
		return
	}

	questions, err := s.DB.GetQuestionsByChallengeID(id)
	if err != nil {
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	if claims == nil || !claims.IsAdmin {
		for i := range questions {
			questions[i].Flag = ""
		}
	}

	// solvedQuestions maps question ID to the flag the user submitted correctly (empty string if not solved)
	solvedQuestions := make(map[string]string)
	if claims != nil {
		for _, q := range questions {
			if flag := s.DB.GetUserCorrectSubmittedFlag(q.ID, claims.UserID); flag != "" {
				solvedQuestions[q.ID] = flag
			}
		}
	}

	customCode, _ := s.DB.GetCustomCode("challenge")
	challengeFiles, _ := s.DB.GetChallengeFiles(id)

	data := map[string]interface{}{
		"Title":           challenge.Name,
		"Page":            "challenge",
		"User":            claims,
		"Challenge":       challenge,
		"Questions":       questions,
		"SolvedQuestions": solvedQuestions,
		"ChallengeFiles":  challengeFiles,
		"CustomCode":      customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleScoreboard(w http.ResponseWriter, r *http.Request) {
	entries, err := s.DB.GetScoreboard(100)
	if err != nil {
		http.Error(w, "Failed to fetch scoreboard", http.StatusInternalServerError)
		return
	}

	customCode, _ := s.DB.GetCustomCode("scoreboard")

	data := map[string]interface{}{
		"Title":      "Scoreboard",
		"Page":       "scoreboard",
		"User":       auth.GetUserFromContext(r.Context()),
		"Entries":    entries,
		"CustomCode": customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleCompetitionList(w http.ResponseWriter, r *http.Request) {
	comps, err := s.DB.ListCompetitions()
	if err != nil {
		comps = []models.Competition{}
	}
	data := map[string]interface{}{
		"Title":        "Competitions",
		"Page":         "competitions",
		"User":         auth.GetUserFromContext(r.Context()),
		"Competitions": comps,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleCompetitionDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	comp, err := s.DB.GetCompetitionByID(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	claims := auth.GetUserFromContext(r.Context())
	var teamRegistered bool
	if claims != nil {
		user, err := s.DB.GetUserByID(claims.UserID)
		if err == nil && user.TeamID != nil {
			teamRegistered, _ = s.DB.IsTeamRegistered(id, *user.TeamID)
		}
	}
	challenges, _ := s.DB.GetCompetitionChallenges(id)
	isAdmin := claims != nil && claims.IsAdmin
	var entries []models.CompetitionScoreboardEntry
	if !comp.ScoreboardBlackout || isAdmin {
		entries, _ = s.DB.GetCompetitionScoreboard(id)
	}
	data := map[string]interface{}{
		"Title":          comp.Name,
		"Page":           "competition",
		"User":           claims,
		"Competition":    comp,
		"Challenges":     challenges,
		"Entries":        entries,
		"TeamRegistered": teamRegistered,
		"BlackedOut":     comp.ScoreboardBlackout && !isAdmin,
		"Completions":    make(map[string]*database.ChallengeCompletion),
	}
	if claims != nil {
		completions, _ := s.DB.GetChallengeCompletionForUser(claims.UserID)
		data["Completions"] = completions
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleSubmissionsPage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	competitions, _ := s.DB.ListCompetitions()
	data := map[string]interface{}{
		"Title":        "Live Submissions",
		"Page":         "submissions",
		"User":         claims,
		"Competitions": competitions,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleTeams(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := s.DB.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	allTeams, err := s.DB.GetAllTeams()
	if err != nil {
		allTeams = []models.Team{}
	}

	customCode, _ := s.DB.GetCustomCode("teams")

	data := map[string]interface{}{
		"Title":      "Teams",
		"Page":       "teams",
		"User":       user,
		"AllTeams":   allTeams,
		"CustomCode": customCode,
	}

	if user.TeamID != nil {
		team, err := s.DB.GetTeamByID(*user.TeamID)
		if err == nil {
			data["Team"] = team
			canSeeInvite := team.OwnerID == claims.UserID ||
				(team.InvitePermission == "all_members")
			data["CanSeeInviteCode"] = canSeeInvite

			members, err := s.DB.GetTeamMembers(*user.TeamID)
			if err == nil {
				data["Members"] = members
			}
		}
	}

	s.Render(w, "base.html", data)
}

func (s *Server) HandleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/yaml")
	data, err := assets.OpenapiSpec.ReadFile("docs/openapi.yaml")
	if err != nil {
		http.Error(w, "OpenAPI spec not found", http.StatusNotFound)
		return
	}
	if _, err := w.Write(data); err != nil {
		log.Printf("handleOpenAPISpec: write error: %v", err)
	}
}

func (s *Server) HandleDocsPage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	customCode, _ := s.DB.GetCustomCode("docs")

	data := map[string]interface{}{
		"Title":      "API Documentation",
		"User":       claims,
		"Page":       "docs",
		"CustomCode": customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleSQL(w http.ResponseWriter, r *http.Request) {
	customCode, _ := s.DB.GetCustomCode("sql")

	data := map[string]interface{}{
		"Title":      "SQL Playground",
		"Page":       "sql",
		"User":       auth.GetUserFromContext(r.Context()),
		"CustomCode": customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	motdText := s.Motd
	if motdText == "" {
		motdText, _ = s.DB.GetSetting("motd")
	}

	customCode, _ := s.DB.GetCustomCode("login")

	data := map[string]interface{}{
		"Title":      "Login",
		"Page":       "login",
		"User":       auth.GetUserFromContext(r.Context()),
		"CustomCode": customCode,
		"MOTD":       motdText,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleRegisterPage(w http.ResponseWriter, r *http.Request) {
	customCode, _ := s.DB.GetCustomCode("register")

	data := map[string]interface{}{
		"Title":      "Register",
		"Page":       "register",
		"User":       auth.GetUserFromContext(r.Context()),
		"CustomCode": customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	customCode, _ := s.DB.GetCustomCode("forgot-password")

	data := map[string]interface{}{
		"Title":      "Forgot Password",
		"Page":       "forgot-password",
		"User":       auth.GetUserFromContext(r.Context()),
		"CustomCode": customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	customCode, _ := s.DB.GetCustomCode("reset-password")

	data := map[string]interface{}{
		"Title":      "Reset Password",
		"Page":       "reset-password",
		"User":       auth.GetUserFromContext(r.Context()),
		"ResetToken": token,
		"CustomCode": customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())

	challenges, err := s.DB.GetChallenges(false)
	if err != nil {
		http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
		return
	}

	questionsWithChallenge, err := s.DB.GetAllQuestionsWithChallenge()
	if err != nil {
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	var hints []models.Hint
	for _, q := range questionsWithChallenge {
		qHints, err := s.DB.GetHintsByQuestionID(q.ID)
		if err == nil {
			hints = append(hints, qHints...)
		}
	}

	categories, _ := s.DB.GetAllCategories()
	difficulties, _ := s.DB.GetAllDifficulties()
	users, _ := s.DB.GetAllUsers()
	competitions, _ := s.DB.ListCompetitions()
	customCode, _ := s.DB.GetCustomCode("admin")

	freezeEnabled, freezeAt, _ := s.DB.GetScoreFreeze()
	freezeAtStr := ""
	if freezeAt != nil {
		freezeAtStr = freezeAt.Format("2006-01-02T15:04")
	}

	adminVisible := s.DB.GetAdminVisibleInScoreboard()

	data := map[string]interface{}{
		"Title":                    "Admin Dashboard",
		"Page":                     "admin",
		"User":                     claims,
		"Challenges":               challenges,
		"Questions":                questionsWithChallenge,
		"Hints":                    hints,
		"Categories":               categories,
		"Difficulties":             difficulties,
		"Users":                    users,
		"CustomCode":               customCode,
		"FreezeEnabled":            freezeEnabled,
		"Frozen":                   s.DB.IsFrozen(),
		"FreezeAt":                 freezeAtStr,
		"AdminVisibleInScoreboard": adminVisible,
		"Competitions":             competitions,
		"BaseURL": func() string {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			return scheme + "://" + r.Host
		}(),
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleEditChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	challenge, err := s.DB.GetChallengeByID(id)
	if err != nil {
		http.Error(w, "Challenge not found", http.StatusNotFound)
		return
	}

	categories, _ := s.DB.GetAllCategories()
	difficulties, _ := s.DB.GetAllDifficulties()

	currentCats := make(map[string]bool)
	for _, c := range strings.Split(challenge.Category, ",") {
		currentCats[strings.TrimSpace(c)] = true
	}
	categoryCheckboxes := ""
	for _, cat := range categories {
		checked := ""
		if currentCats[cat.Name] {
			checked = "checked"
		}
		categoryCheckboxes += fmt.Sprintf(`<label class="flex items-center text-sm text-gray-300 cursor-pointer">
			<input type="checkbox" name="category" value="%s" %s class="w-4 h-4 rounded border-dark-border bg-dark-bg cursor-pointer mr-2"> %s
		</label>`, cat.Name, checked, cat.Name)
	}

	difficultyOptions := ""
	for _, d := range difficulties {
		selected := ""
		if d.Name == challenge.Difficulty {
			selected = "selected"
		}
		difficultyOptions += fmt.Sprintf(`<option value="%s" %s>%s</option>`, d.Name, selected, d.Name)
	}

	w.Header().Set("Content-Type", "text/html")
	visibleChecked := ""
	if challenge.Visible {
		visibleChecked = "checked"
	}

	sqlEnabledChecked := ""
	if challenge.SQLEnabled {
		sqlEnabledChecked = "checked"
	}

	sqlDatasetURL := ""
	if challenge.SQLDatasetURL != nil {
		sqlDatasetURL = *challenge.SQLDatasetURL
	}

	sqlSchemaHint := ""
	if challenge.SQLSchemaHint != nil {
		sqlSchemaHint = *challenge.SQLSchemaHint
	}

	dynamicScoringChecked := ""
	if challenge.DynamicScoring {
		dynamicScoringChecked = "checked"
	}

	files, _ := s.DB.GetChallengeFiles(id)
	filesHTML := `<div class="space-y-2 mb-3">`
	for _, f := range files {
		sizeStr := ""
		if f.SizeBytes != nil && *f.SizeBytes > 0 {
			if *f.SizeBytes < 1024 {
				sizeStr = fmt.Sprintf(" (%d bytes)", *f.SizeBytes)
			} else if *f.SizeBytes < 1024*1024 {
				sizeStr = fmt.Sprintf(" (%.1f KB)", float64(*f.SizeBytes)/1024)
			} else {
				sizeStr = fmt.Sprintf(" (%.1f MB)", float64(*f.SizeBytes)/(1024*1024))
			}
		}
		filesHTML += fmt.Sprintf(`<div class="flex items-center justify-between bg-dark-bg border border-dark-border rounded p-2">
			<div class="flex items-center gap-2">
				<span class="text-green-400 text-sm">📎 %s%s</span>
				<a href="%s" class="text-blue-400 hover:text-blue-300 text-sm underline" target="_blank">Download</a>
			</div>
			<button hx-delete="/api/admin/challenge-files/%s" hx-target="#file-section-%s" hx-swap="outerHTML" class="text-red-400 hover:text-red-300 text-xs">Remove</button>
		</div>`, f.Filename, sizeStr, f.StoragePath, f.ID, id)
	}
	filesHTML += `</div>`

	fileSection := fmt.Sprintf(`%s
	<div x-data="{ files: [{source: 'none'}] }" class="border-t border-dark-border pt-3 mt-3">
		<p class="text-xs font-medium text-blue-400 mb-2">Add New Files</p>

		<template x-for="(file, index) in files" :key="index">
			<div class="mb-2 p-2 bg-dark-bg border border-dark-border rounded">
				<div class="flex gap-2 mb-1">
					<label class="flex items-center text-xs text-gray-300 cursor-pointer">
						<input type="radio" :name="'newfile_' + index + '_source'" value="none" x-model="file.source" class="mr-1" checked> Skip
					</label>
					<label class="flex items-center text-xs text-gray-300 cursor-pointer">
						<input type="radio" :name="'newfile_' + index + '_source'" value="upload" x-model="file.source" class="mr-1"> Upload
					</label>
					<label class="flex items-center text-xs text-gray-300 cursor-pointer">
						<input type="radio" :name="'newfile_' + index + '_source'" value="external" x-model="file.source" class="mr-1"> URL
					</label>
				</div>
				<div x-show="file.source === 'upload'">
					<input type="file" :name="'newfile_' + index + '_file'" class="text-xs text-gray-300 w-full">
				</div>
				<div x-show="file.source === 'external'" class="space-y-1">
					<input type="text" :name="'newfile_' + index + '_name'" placeholder="Filename (optional)" class="w-full px-2 py-1 bg-dark-bg border border-dark-border text-white rounded text-xs">
					<input type="url" :name="'newfile_' + index + '_url'" placeholder="https://example.com/file.zip" class="w-full px-2 py-1 bg-dark-bg border border-dark-border text-white rounded text-xs">
				</div>
			</div>
		</template>

		<div class="flex gap-2 mt-2">
			<button type="button" @click="files.push({source: 'none'})" class="text-blue-400 hover:text-blue-300 text-xs">+ Add another</button>
			<button type="button"
				hx-post="/api/admin/challenges/%s/files/batch"
				hx-target="#file-section-%s"
				hx-encoding="multipart/form-data"
				class="px-3 py-1 bg-green-600 hover:bg-green-700 text-white rounded text-xs">Upload All</button>
		</div>
	</div>`, filesHTML, id, id)

	html := fmt.Sprintf(`<div id="challenge-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
		<form hx-put="/api/admin/challenges/%s" hx-target="closest #challenge-%s" hx-swap="outerHTML" class="space-y-3">
			<div>
				<label class="block text-xs font-medium text-gray-300 mb-1">Name</label>
				<input autofocus type="text" name="name" value="%s" placeholder="e.g., Web Security 101" class="w-full px-4 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>
			</div>
			<div>
				<label class="block text-xs font-medium text-gray-300 mb-1">Description</label>
				<textarea name="description" placeholder="Challenge description..." class="w-full px-4 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>%s</textarea>
			</div>
			<div class="grid grid-cols-2 gap-3">
				<div>
					<label class="block text-xs font-medium text-gray-300 mb-1">Categories</label>
					<div class="space-y-1">%s</div>
				</div>
				<div>
					<label class="block text-xs font-medium text-gray-300 mb-1">Difficulty</label>
					<select name="difficulty" class="w-full px-4 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>
						%s
					</select>
				</div>
			</div>
			<label class="flex items-center text-sm text-gray-300 cursor-pointer">
				<input type="checkbox" name="visible" value="on" %s class="w-4 h-4 rounded border-dark-border bg-dark-bg cursor-pointer mr-2"> Visible to users
			</label>
			<!-- SQL Playground Section -->
			<div class="border-t border-dark-border pt-3 mt-3">
				<p class="text-xs font-medium text-purple-400 mb-2">SQL Playground</p>
				<label class="flex items-center text-sm text-gray-300 cursor-pointer mb-2">
					<input type="checkbox" name="sql_enabled" value="on" %s class="w-4 h-4 rounded border-dark-border bg-dark-bg cursor-pointer mr-2"> Enable SQL Playground
				</label>
				<div class="pl-6 space-y-2">
					<div>
						<label class="block text-xs font-medium text-gray-300 mb-1">Dataset URL (optional)</label>
						<input type="url" name="sql_dataset_url" value="%s" placeholder="https://example.com/dataset.csv" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm focus:outline-none focus:border-purple-500">
					</div>
					<div>
						<label class="block text-xs font-medium text-gray-300 mb-1">Schema Hint</label>
						<textarea name="sql_schema_hint" placeholder="-- Tables available..." rows="3" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm focus:outline-none focus:border-purple-500 font-mono">%s</textarea>
					</div>
				</div>
			</div>
			<!-- Dynamic Scoring Section -->
			<div class="border-t border-dark-border pt-3 mt-3">
				<p class="text-xs font-medium text-yellow-400 mb-2">Dynamic Scoring</p>
				<label class="flex items-center text-sm text-gray-300 cursor-pointer mb-2">
					<input type="checkbox" name="dynamic_scoring" value="on" %s class="w-4 h-4 rounded border-dark-border bg-dark-bg cursor-pointer mr-2"> Enable dynamic scoring
				</label>
				<p class="text-xs text-gray-500 mb-2 pl-6">Points decay linearly from Initial down to Minimum once Decay Threshold solves are reached.</p>
				<div class="grid grid-cols-3 gap-3 pl-6">
					<div>
						<label class="block text-xs font-medium text-gray-300 mb-1">Initial Points</label>
						<input type="number" name="initial_points" value="%d" min="1" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm focus:outline-none focus:border-purple-500">
						<p class="text-xs text-gray-500 mt-1">Max pts (0 solves)</p>
					</div>
					<div>
						<label class="block text-xs font-medium text-gray-300 mb-1">Minimum Points</label>
						<input type="number" name="minimum_points" value="%d" min="1" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm focus:outline-none focus:border-purple-500">
						<p class="text-xs text-gray-500 mt-1">Floor (never below)</p>
					</div>
					<div>
						<label class="block text-xs font-medium text-gray-300 mb-1">Decay Threshold</label>
						<input type="number" name="decay_threshold" value="%d" min="1" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm focus:outline-none focus:border-purple-500">
						<p class="text-xs text-gray-500 mt-1">Solves to reach min</p>
					</div>
				</div>
			</div>
		<!-- File Attachment Section -->
		<div class="border-t border-dark-border pt-3 mt-3">
			<p class="text-xs font-medium text-blue-400 mb-2">File Attachment</p>
			<div id="file-section-%s">%s</div>
		</div>
			<div class="flex gap-2">
				<button type="submit" class="px-3 py-1 bg-green-600 hover:bg-green-700 text-white rounded text-sm font-medium transition">Save</button>
				<button type="button" hx-get="/admin/challenges/%s/view" hx-target="closest #challenge-%s" hx-swap="outerHTML" class="px-3 py-1 bg-gray-600 hover:bg-gray-700 text-white rounded text-sm font-medium transition">Cancel</button>
			</div>
		</form>
	</div>`,
		id, id, id,
		challenge.Name,
		challenge.Description,
		categoryCheckboxes,
		difficultyOptions,
		visibleChecked,
		sqlEnabledChecked,
		sqlDatasetURL,
		sqlSchemaHint,
		dynamicScoringChecked,
		challenge.InitialPoints,
		challenge.MinimumPoints,
		challenge.DecayThreshold,
		id, fileSection, id, id)

	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("handleEditChallenge: write error: %v", err)
	}
}

func (s *Server) HandleViewChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	challenge, err := s.DB.GetChallengeByID(id)
	if err != nil {
		http.Error(w, "Challenge not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	difficultyColor := "text-gray-400"
	if d, err := s.DB.GetDifficultyByName(challenge.Difficulty); err == nil {
		difficultyColor = d.TextColor
	}

	hiddenBadge := ""
	if !challenge.Visible {
		hiddenBadge = `<span class="ml-2 text-xs bg-gray-700 text-gray-300 px-2 py-1 rounded">Hidden</span>`
	}

	html := fmt.Sprintf(`<div id="challenge-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
		<div class="flex justify-between items-start mb-3">
			<div>
				<h3 class="text-xl font-bold text-white">%s</h3>
				<p class="text-sm text-gray-400">
					Category: <span class="text-blue-400">%s</span> •
					Difficulty: <span class="font-medium %s">%s</span>
					%s
				</p>
			</div>
		</div>
		<p class="text-gray-300 mb-4">%s</p>
		<div class="flex gap-2">
			<button
				onclick="htmx.ajax('GET', '/admin/challenges/%s/edit', {target: '#challenge-%s', swap: 'outerHTML'})"
				class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">
				Edit
			</button>
			<button
				@click="if(confirm('Delete this challenge? This action cannot be undone.')) { htmx.trigger('#del-challenge-%s', 'click') }"
				class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">
				Delete
			</button>
			<button
				style="display:none"
				id="del-challenge-%s"
				hx-delete="/api/admin/challenges/%s"
				hx-target="#challenge-%s"
				hx-swap="outerHTML swap:1s">
			</button>
		</div>
	</div>`,
		id,
		challenge.Name,
		challenge.Category,
		difficultyColor,
		challenge.Difficulty,
		hiddenBadge,
		challenge.Description,
		id, id,
		id,
		id, id, id)

	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("handleViewChallenge: write error: %v", err)
	}
}

func (s *Server) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Printf("handleHealthz: write error: %v", err)
	}
}

func (s *Server) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := s.DB.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"not_ready","checks":{"database":"error: %s"}}`, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ready","checks":{"database":"ok"}}`)); err != nil {
		log.Printf("handleReadyz: write error: %v", err)
	}
}

func (s *Server) HandleOwnProfile(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	stats, err := s.DB.GetUserStats(claims.UserID)
	if err != nil {
		http.Error(w, "Error loading profile", http.StatusInternalServerError)
		return
	}

	submissions, _ := s.DB.GetUserRecentSubmissions(claims.UserID, 20)
	solved, _ := s.DB.GetUserSolvedChallenges(claims.UserID)
	customCode, _ := s.DB.GetCustomCode("profile")

	data := map[string]interface{}{
		"Title":             "My Profile",
		"Page":              "profile",
		"User":              claims,
		"Stats":             stats,
		"RecentSubmissions": submissions,
		"SolvedChallenges":  solved,
		"IsOwnProfile":      true,
		"CustomCode":        customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleUserProfile(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	stats, err := s.DB.GetUserStats(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	submissions, _ := s.DB.GetUserRecentSubmissions(userID, 20)
	solved, _ := s.DB.GetUserSolvedChallenges(userID)
	customCode, _ := s.DB.GetCustomCode("profile")

	data := map[string]interface{}{
		"Title":             "User Profile",
		"Page":              "profile",
		"User":              claims,
		"Stats":             stats,
		"RecentSubmissions": submissions,
		"SolvedChallenges":  solved,
		"IsOwnProfile":      false,
		"CustomCode":        customCode,
	}
	s.Render(w, "base.html", data)
}

// HandleAPIUserProfile godoc
// @Summary Get user profile stats as JSON
// @Tags Users
// @Security CookieAuth
// @Param id path string true "User ID (or 'me' for current user)"
// @Success 200 {object} database.UserStats
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /api/users/{id}/profile [get]
func (s *Server) HandleAPIUserProfile(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	idParam := chi.URLParam(r, "id")
	var userID string
	if idParam == "" || idParam == "me" {
		if claims == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		userID = claims.UserID
	} else {
		userID = idParam
	}
	stats, err := s.DB.GetUserStats(userID)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	// Hide email for non-owners and non-admins
	if claims == nil || (claims.UserID != userID && !claims.IsAdmin) {
		stats.Email = ""
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) HandleTeamProfile(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	team, err := s.DB.GetTeamByID(teamID)
	if err != nil {
		s.RenderError(w, 404, "Team Not Found", "The team you're looking for doesn't exist.")
		return
	}

	members, err := s.DB.GetTeamMembers(teamID)
	if err != nil {
		http.Error(w, "Failed to fetch team members", http.StatusInternalServerError)
		return
	}

	owner, err := s.DB.GetUserByID(team.OwnerID)
	if err != nil {
		http.Error(w, "Failed to fetch team owner", http.StatusInternalServerError)
		return
	}

	var teamStats struct {
		TotalPoints int
		SolvedCount int
		Rank        int
	}

	scoreboard, err := s.DB.GetTeamScoreboard(1000)
	if err == nil {
		for _, entry := range scoreboard {
			if entry.TeamID != nil && *entry.TeamID == teamID {
				teamStats.TotalPoints = entry.Points
				teamStats.SolvedCount = entry.SolveCount
				teamStats.Rank = entry.Rank
				break
			}
		}
	}

	solvedChallenges, _ := s.DB.GetTeamSolvedChallenges(teamID)
	scoringChallenges, _ := s.DB.GetTeamScoringChallenges(teamID)
	recentSubmissions, _ := s.DB.GetTeamRecentSubmissions(teamID, 20)
	scoringSubmissions, _ := s.DB.GetTeamScoringSubmissions(teamID, 20)
	customCode, _ := s.DB.GetCustomCode("team")

	data := map[string]interface{}{
		"Title":              team.Name + " - Team Profile",
		"Page":               "team-profile",
		"User":               claims,
		"Team":               team,
		"Members":            members,
		"Owner":              owner,
		"Stats":              teamStats,
		"SolvedChallenges":   solvedChallenges,
		"ScoringChallenges":  scoringChallenges,
		"RecentSubmissions":  recentSubmissions,
		"ScoringSubmissions": scoringSubmissions,
		"CustomCode":         customCode,
	}
	s.Render(w, "base.html", data)
}

func (s *Server) HandleCategoriesCheckboxes(w http.ResponseWriter, r *http.Request) {
	categories, err := s.DB.GetAllCategories()
	if err != nil {
		http.Error(w, "Failed to fetch categories", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	var html strings.Builder
	for _, cat := range categories {
		html.WriteString(fmt.Sprintf(`<label class="flex items-center text-sm text-gray-300 cursor-pointer">
			<input type="checkbox" name="category" value="%s" class="w-4 h-4 rounded border-dark-border bg-dark-bg cursor-pointer mr-2"> %s
		</label>`, cat.Name, cat.Name))
	}
	if _, err := w.Write([]byte(html.String())); err != nil {
		log.Printf("handleCategoriesCheckboxes: write error: %v", err)
	}
}

func (s *Server) HandleDifficultiesDropdown(w http.ResponseWriter, r *http.Request) {
	difficulties, err := s.DB.GetAllDifficulties()
	if err != nil {
		http.Error(w, "Failed to fetch difficulties", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	var html strings.Builder
	html.WriteString(`<option value="">Select difficulty</option>`)
	for _, diff := range difficulties {
		html.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, diff.Name, diff.Name))
	}
	if _, err := w.Write([]byte(html.String())); err != nil {
		log.Printf("handleDifficultiesDropdown: write error: %v", err)
	}
}

func (s *Server) HandleEditQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	question, err := s.DB.GetQuestionByID(id)
	if err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	challenges, _ := s.DB.GetChallenges(false)
	challengeOptions := ""
	for _, c := range challenges {
		selected := ""
		if c.ID == question.ChallengeID {
			selected = "selected"
		}
		challengeOptions += fmt.Sprintf(`<option value="%s" %s>%s</option>`, c.ID, selected, c.Name)
	}

	w.Header().Set("Content-Type", "text/html")
	caseSensitiveChecked := ""
	if question.CaseSensitive {
		caseSensitiveChecked = "checked"
	}

	flagMask := ""
	if question.FlagMask != nil {
		flagMask = *question.FlagMask
	}

	html := fmt.Sprintf(`<div id="question-%s" class="bg-white dark:bg-dark-surface border border-gray-200 dark:border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
		<form hx-put="/api/admin/questions/%s" hx-target="closest #question-%s" hx-swap="outerHTML" class="space-y-3">
			<div>
				<label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Challenge</label>
				<select name="challenge_id" class="w-full px-4 py-2 bg-white dark:bg-dark-bg border border-gray-300 dark:border-dark-border text-gray-900 dark:text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>
					%s
				</select>
			</div>
			<div>
				<label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Question Name</label>
				<input autofocus type="text" name="name" value="%s" placeholder="e.g., Find the SQL Injection" class="w-full px-4 py-2 bg-white dark:bg-dark-bg border border-gray-300 dark:border-dark-border text-gray-900 dark:text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>
			</div>
			<div>
				<label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
				<textarea name="description" placeholder="Question description and hints..." class="w-full px-4 py-2 bg-white dark:bg-dark-bg border border-gray-300 dark:border-dark-border text-gray-900 dark:text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>%s</textarea>
			</div>
			<div>
				<label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Flag</label>
				<input type="text" name="flag" value="%s" placeholder="flag{...}" class="w-full px-4 py-2 bg-white dark:bg-dark-bg border border-gray-300 dark:border-dark-border text-gray-900 dark:text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>
			</div>
			<div class="grid grid-cols-2 gap-3">
				<div>
					<label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Points</label>
					<input type="number" name="points" value="%d" placeholder="100" class="w-full px-4 py-2 bg-white dark:bg-dark-bg border border-gray-300 dark:border-dark-border text-gray-900 dark:text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>
				</div>
				<div>
					<label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Flag Mask</label>
					<input type="text" name="flag_mask" value="%s" placeholder="flag{****}" class="w-full px-4 py-2 bg-white dark:bg-dark-bg border border-gray-300 dark:border-dark-border text-gray-900 dark:text-white rounded focus:outline-none focus:border-purple-500 text-sm">
				</div>
			</div>
			<label class="flex items-center text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
				<input type="checkbox" name="case_sensitive" value="on" %s class="w-4 h-4 rounded border-gray-300 dark:border-dark-border bg-white dark:bg-dark-bg cursor-pointer mr-2"> Case sensitive flag
			</label>
			<div class="flex gap-2">
				<button type="submit" class="px-3 py-1 bg-green-600 hover:bg-green-700 text-white rounded text-sm font-medium transition">Save</button>
				<button type="button" hx-get="/admin/questions/%s/view" hx-target="closest #question-%s" hx-swap="outerHTML" class="px-3 py-1 bg-gray-400 dark:bg-gray-600 hover:bg-gray-500 dark:hover:bg-gray-700 text-white rounded text-sm font-medium transition">Cancel</button>
			</div>
		</form>
	</div>`,
		id, id, id,
		challengeOptions,
		question.Name,
		question.Description,
		question.Flag,
		question.Points,
		flagMask,
		caseSensitiveChecked,
		id, id)

	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("handleEditQuestion: write error: %v", err)
	}
}

func (s *Server) HandleViewQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	question, err := s.DB.GetQuestionByID(id)
	if err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	flagMaskDisplay := ""
	if question.FlagMask != nil && *question.FlagMask != "" {
		flagMaskDisplay = fmt.Sprintf(`<span class="ml-2">Mask: <code class="bg-dark-bg px-2 py-1 rounded text-yellow-400">%s</code></span>`, *question.FlagMask)
	}

	html := fmt.Sprintf(`<div id="question-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
		<div class="mb-3">
			<h3 class="text-xl font-bold text-white">%s</h3>
			<p class="text-sm text-gray-400">
				Challenge ID: <span class="text-blue-400">%s</span> •
				Points: <span class="text-green-400 font-medium">%d</span>
			</p>
		</div>
		<p class="text-gray-300 mb-2 text-sm">%s</p>
		<p class="text-gray-400 text-xs mb-4">
			Flag: <code class="bg-dark-bg px-2 py-1 rounded text-purple-400">%s</code>
			%s
		</p>
		<div class="flex gap-2">
			<button
				onclick="htmx.ajax('GET', '/admin/questions/%s/edit', {target: '#question-%s', swap: 'outerHTML'})"
				class="px-3 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm font-medium transition">
				Edit
			</button>
			<button
				@click="if(confirm('Delete this question? This action cannot be undone.')) { htmx.trigger('#del-question-%s', 'click') }"
				class="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition">
				Delete
			</button>
			<button
				style="display:none"
				id="del-question-%s"
				hx-delete="/api/admin/questions/%s"
				hx-target="#question-%s"
				hx-swap="outerHTML swap:1s">
			</button>
		</div>
	</div>`,
		id,
		question.Name,
		question.ChallengeID,
		question.Points,
		question.Description,
		question.Flag,
		flagMaskDisplay,
		id, id,
		id,
		id, id, id)

	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("handleViewQuestion: write error: %v", err)
	}
}
