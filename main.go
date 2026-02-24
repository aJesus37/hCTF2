// Note: This generates Swagger 2.0 (not OpenAPI 3.0). swaggo/swag only supports
// Swagger 2.0 natively. The Swagger UI at /api/openapi renders it correctly.
// Cookie-based auth (CookieAuth) is documented here despite being a 3.0 feature;
// swaggo emits it as an extension and Swagger UI displays it correctly.
// @title hCTF2 API
// @version 1.0.0
// @description Self-hosted CTF platform API. Most write endpoints require authentication via JWT cookie (auth_token).
// @host localhost:8090
// @BasePath /api
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name auth_token
// @tag.name Auth
// @tag.description Authentication endpoints (login, register, logout, password reset)
// @tag.name Challenges
// @tag.description Challenge listing, details, and flag submission
// @tag.name Teams
// @tag.description Team creation, joining, and management
// @tag.name Hints
// @tag.description Hint viewing and unlocking
// @tag.name Scoreboard
// @tag.description Scoreboard data
// @tag.name Admin
// @tag.description Admin-only CRUD for challenges, questions, hints, categories, users
// @tag.name SQL
// @tag.description SQL Playground snapshot data

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yourusername/hctf2/internal/auth"
	"github.com/yourusername/hctf2/internal/database"
	"github.com/yourusername/hctf2/internal/email"
	"github.com/yourusername/hctf2/internal/handlers"
	"github.com/yourusername/hctf2/internal/models"
	"github.com/yourusername/hctf2/internal/telemetry"
	"github.com/yourusername/hctf2/internal/utils"
)

//go:embed internal/views/templates/*
var templatesFS embed.FS

//go:embed internal/views/static
var embedFS embed.FS

//go:embed docs/openapi.yaml
var openapiSpec embed.FS

// staticFS is a SubFS starting at internal/views/static
var staticFS fs.FS

func init() {
	var err error
	staticFS, err = fs.Sub(embedFS, "internal/views/static")
	if err != nil {
		log.Fatalf("Failed to create staticFS SubFS: %v", err)
	}
}

// firstNonEmpty returns the first non-empty string from the provided values
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

type Server struct {
	db          *database.DB
	templates   *template.Template
	authH       *handlers.AuthHandler
	challengeH  *handlers.ChallengeHandler
	scoreboardH *handlers.ScoreboardHandler
	teamH       *handlers.TeamHandler
	hintH       *handlers.HintHandler
	sqlH        *handlers.SQLHandler
	profileH    *handlers.ProfileHandler
	settingsH   *handlers.SettingsHandler
	motd        string
}

// customFileHandler wraps the file server to set proper content types for WASM and workers
type customFileHandler struct {
	fs http.Handler
}

func (h *customFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set proper content types
	if strings.HasSuffix(r.URL.Path, ".wasm") {
		w.Header().Set("Content-Type", "application/wasm")
	} else if strings.HasSuffix(r.URL.Path, ".worker.js") {
		w.Header().Set("Content-Type", "application/javascript")
	}

	// CORS headers are handled by global middleware

	h.fs.ServeHTTP(w, r)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// corsMiddleware returns a middleware that handles CORS based on configuration
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowOrigin := ""
			if len(allowedOrigins) == 0 {
				// Same-origin only: only allow if no origin header (same-origin request)
				if origin == "" {
					allowOrigin = "*"
				}
			} else {
				// Check against allowed list
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
func main() {
	var (
		port             = flag.Int("port", 8090, "Server port")
		dbPath           = flag.String("db", "./hctf2.db", "Database path")
		adminEmail       = flag.String("admin-email", "", "Admin email for first-time setup")
		adminPass        = flag.String("admin-password", "", "Admin password for first-time setup")
		motd             = flag.String("motd", "", "Message of the Day displayed below login form")
		enablePrometheus = flag.Bool("metrics", false, "Enable Prometheus /metrics endpoint")
		otlpEndpoint     = flag.String("otel-otlp-endpoint", "", "OTLP exporter endpoint (e.g. localhost:4318)")
		smtpHost         = flag.String("smtp-host", "", "SMTP server host")
		smtpPort         = flag.Int("smtp-port", 587, "SMTP server port")
		smtpFrom         = flag.String("smtp-from", "", "SMTP from address")
		smtpUser         = flag.String("smtp-user", "", "SMTP username")
		smtpPass         = flag.String("smtp-password", "", "SMTP password")
		baseURL          = flag.String("base-url", "http://localhost:8090", "Base URL for links in emails")
		jwtSecret        = flag.String("jwt-secret", getEnv("JWT_SECRET", ""), "JWT signing secret (min 32 chars, required in production)")
		dev              = flag.Bool("dev", false, "Enable development mode (allows default JWT secret, relaxed security)")
		corsOrigins      = flag.String("cors-origins", getEnv("CORS_ORIGINS", ""), "Comma-separated list of allowed CORS origins (empty = same-origin only)")
	)
	flag.Parse()

	// Check if development mode is enabled via --dev flag
	devMode := *dev

	// Set JWT secret
	jwtSecretValue := *jwtSecret

	if jwtSecretValue == "" || jwtSecretValue == "change-this-secret-in-production" {
		if devMode {
			log.Println("WARNING: Using default JWT secret in development mode. DO NOT use in production!")
			jwtSecretValue = "change-this-secret-in-production"
		} else {
			log.Fatal("ERROR: JWT secret is required. Use --dev for development, or set --jwt-secret flag, JWT_SECRET env var. The secret must be at least 32 characters.")
		}
	}

	if err := auth.SetJWTSecret(jwtSecretValue); err != nil {
		log.Fatalf("ERROR: Invalid JWT secret: %v", err)
	}

	// Create email service
	emailSvc := email.NewService(email.Config{
		Host:     firstNonEmpty(*smtpHost, os.Getenv("SMTP_HOST")),
		Port:     *smtpPort,
		From:     firstNonEmpty(*smtpFrom, os.Getenv("SMTP_FROM")),
		Username: firstNonEmpty(*smtpUser, os.Getenv("SMTP_USER")),
		Password: firstNonEmpty(*smtpPass, os.Getenv("SMTP_PASSWORD")),
	})

	if !emailSvc.IsConfigured() {
		log.Println("Warning: SMTP not configured. Password reset links will be logged to console.")
	}

	// Initialize telemetry
	cleanupTelemetry, err := telemetry.Init(telemetry.Config{
		ServiceName:          "hctf2",
		ServiceVersion:       "0.5.0",
		Environment:          os.Getenv("ENVIRONMENT"),
		EnableStdoutExporter: os.Getenv("OTEL_EXPORTER_STDOUT") == "true",
		EnablePrometheus:     *enablePrometheus || os.Getenv("OTEL_METRICS_PROMETHEUS") == "true",
		OTLPEndpoint:         firstNonEmpty(*otlpEndpoint, os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize telemetry: %v", err)
	} else {
		defer cleanupTelemetry()
	}

	// Initialize database
	db, err := database.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create admin user if specified
	if *adminEmail != "" && *adminPass != "" {
		createAdminUser(db, *adminEmail, *adminPass)
	}

	// Parse templates
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"markdown":      utils.RenderMarkdown,
		"stripMarkdown": utils.StripMarkdown,
		"safeHTML":      func(s string) template.HTML { return template.HTML(s) },
		"mul":           func(a, b int) int { return a * b },
		"div":           func(a, b int) int { if b == 0 { return 0 }; return a / b },
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
	}).ParseFS(templatesFS, "internal/views/templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Initialize server
	s := &Server{
		db:          db,
		templates:   tmpl,
		authH:       handlers.NewAuthHandler(db, emailSvc, *baseURL),
		challengeH:  handlers.NewChallengeHandler(db),
		scoreboardH: handlers.NewScoreboardHandler(db),
		teamH:       handlers.NewTeamHandler(db),
		hintH:       handlers.NewHintHandler(db),
		sqlH:        handlers.NewSQLHandler(db),
		profileH:    handlers.NewProfileHandler(db),
		settingsH:   handlers.NewSettingsHandler(db),
		motd:        *motd,
	}

	// Parse CORS origins from CLI flag
	var allowedOrigins []string
	if *corsOrigins != "" {
		allowedOrigins = strings.Split(*corsOrigins, ",")
		// Trim spaces
		for i := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
		}
	}

	// Setup router
	r := chi.NewRouter()

	// CORS middleware for CDN resources and DuckDB WASM
	r.Use(corsMiddleware(allowedOrigins))

	// Note: COEP/COOP headers removed for /sql page as they block CDN resources
	// CodeMirror and other dependencies load from esm.sh and other CDNs
	// DuckDB WASM works without these headers for basic queries
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For static files, allow shared array buffers
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

	// Static files with proper content types for WASM and workers
	r.Handle("/static/*", http.StripPrefix("/static/", &customFileHandler{
		fs: http.FileServer(http.FS(staticFS)),
	}))

	// Health check endpoints (no auth, no middleware)
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)

	// Prometheus metrics endpoint
	if *enablePrometheus || os.Getenv("OTEL_METRICS_PROMETHEUS") == "true" {
		r.Handle("/metrics", telemetry.PrometheusHandler())
	}

	// Public routes
	r.Get("/", s.handleIndex)
	r.Get("/challenges", s.handleChallenges)
	r.Get("/challenges/{id}", s.handleChallengeDetail)
	r.Get("/scoreboard", s.handleScoreboard)
	r.Get("/sql", s.handleSQL)
	r.Get("/login", s.handleLoginPage)
	r.Get("/register", s.handleRegisterPage)
	r.Get("/forgot-password", s.handleForgotPasswordPage)
	r.Get("/reset-password", s.handleResetPasswordPage)

	// Protected team routes
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Get("/teams", s.handleTeams)
	})

	// Profile routes (public view, own profile requires auth)
	r.Get("/profile", s.handleOwnProfile)
	r.Get("/users/{id}", s.handleUserProfile)
	r.Get("/teams/{id}/profile", s.handleTeamProfile)

	// Admin UI routes
	r.Group(func(r chi.Router) {
		r.Use(s.requireAdmin)
		r.Get("/admin", s.handleAdminDashboard)
		r.Get("/admin/challenges/{id}/edit", s.handleEditChallenge)
		r.Get("/admin/challenges/{id}/view", s.handleViewChallenge)
		r.Get("/admin/questions/{id}/edit", s.handleEditQuestion)
		r.Get("/admin/questions/{id}/view", s.handleViewQuestion)
	})

	// API routes - Auth
	r.Post("/api/auth/register", s.authH.Register)
	r.Post("/api/auth/login", s.authH.Login)
	r.Post("/api/auth/forgot-password", s.authH.ForgotPassword)
	r.Post("/api/auth/reset-password", s.authH.ResetPassword)

	// Protected Auth routes
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/auth/logout", s.authH.Logout)
	})

	// API routes - Challenges (public read)
	r.Get("/api/challenges", s.challengeH.ListChallenges)
	r.Get("/api/challenges/{id}", s.challengeH.GetChallenge)
	r.Get("/api/challenges-dropdown", s.challengeH.GetChallengesDropdown)
	r.Get("/api/questions-dropdown", s.challengeH.GetQuestionsDropdown)
	r.Get("/api/questions/{questionId}/next-hint-order", s.challengeH.GetNextHintOrder)

	// API routes - Submissions (protected)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/questions/{id}/submit", s.challengeH.SubmitFlag)
	})

	// API routes - Teams (public read, protected write)
	r.Get("/api/teams", s.teamH.ListTeams)
	r.Get("/api/teams/{id}", s.teamH.GetTeam)
	r.Get("/api/teams/scoreboard", s.teamH.GetTeamScoreboard)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/teams", s.teamH.CreateTeam)
		r.Post("/api/teams/join/{invite_id}", s.teamH.JoinTeam)
		r.Post("/api/teams/leave", s.teamH.LeaveTeam)
		r.Post("/api/teams/transfer-ownership", s.teamH.TransferOwnership)
		r.Post("/api/teams/disband", s.teamH.DisbandTeam)
		r.Post("/api/teams/regenerate-invite", s.teamH.RegenerateInviteCode)
		r.Post("/api/teams/invite-permission", s.teamH.UpdateInvitePermission)
	})

	// API routes - Hints (public read, protected unlock)
	r.Get("/api/questions/{questionId}/hints", s.hintH.GetHints)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/api/hints/{id}/unlock", s.hintH.UnlockHint)
	})

	// API routes - Admin (protected)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAdmin)
		r.Post("/api/admin/challenges", s.challengeH.CreateChallenge)
		r.Put("/api/admin/challenges/{id}", s.challengeH.UpdateChallenge)
		r.Delete("/api/admin/challenges/{id}", s.challengeH.DeleteChallenge)
		r.Post("/api/admin/questions", s.challengeH.CreateQuestion)
		r.Put("/api/admin/questions/{id}", s.challengeH.UpdateQuestion)
		r.Delete("/api/admin/questions/{id}", s.challengeH.DeleteQuestion)
		r.Post("/api/admin/hints", s.challengeH.CreateHint)
		r.Put("/api/admin/hints/{id}", s.challengeH.UpdateHint)
		r.Delete("/api/admin/hints/{id}", s.challengeH.DeleteHint)
		r.Post("/api/admin/categories", s.settingsH.CreateCategory)
		r.Put("/api/admin/categories/{id}", s.settingsH.UpdateCategory)
		r.Delete("/api/admin/categories/{id}", s.settingsH.DeleteCategory)
		r.Post("/api/admin/difficulties", s.settingsH.CreateDifficulty)
		r.Put("/api/admin/difficulties/{id}", s.settingsH.UpdateDifficulty)
		r.Delete("/api/admin/difficulties/{id}", s.settingsH.DeleteDifficulty)
		r.Get("/api/admin/custom-code", s.settingsH.GetCustomCode)
		r.Put("/api/admin/custom-code", s.settingsH.UpdateCustomCode)
		r.Get("/api/admin/users", s.settingsH.ListUsers)
		r.Put("/api/admin/users/{id}/admin", s.settingsH.UpdateUserAdmin)
		r.Delete("/api/admin/users/{id}", s.settingsH.DeleteUser)
		r.Get("/api/categories-checkboxes", s.handleCategoriesCheckboxes)
		r.Get("/api/difficulties-dropdown", s.handleDifficultiesDropdown)
	})

	// API routes - SQL
	r.Get("/api/sql/snapshot", s.sqlH.GetSnapshot)

	// OpenAPI Spec
	r.Get("/api/openapi.yaml", s.handleOpenAPISpec)

	// OpenAPI Docs UI
	r.Get("/docs", s.handleDocsPage)

	// API routes - Scoreboard
	r.Get("/api/scoreboard", s.scoreboardH.GetScoreboard)

	// 404 handler for unmatched routes
	r.NotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.renderError(w, 404, "Page Not Found", "The page you're looking for doesn't exist.")
	}))

	// 405 handler for method not allowed
	r.MethodNotAllowed(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.renderError(w, 405, "Method Not Allowed", "The HTTP method used is not supported for this endpoint.")
	}))

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Channel to handle shutdown
	shutdownDone := make(chan struct{})

	// Handle signals gracefully
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("\nShutting down server...")
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
}

func createAdminUser(db *database.DB, email, password string) {
	// Check if admin exists
	if _, err := db.GetUserByEmail(email); err == nil {
		log.Printf("Admin user already exists: %s", email)
		return
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	_, err = db.CreateUser(email, passwordHash, "Admin", true)
	if err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	log.Printf("Admin user created: %s", email)
}

// Template rendering helper
func (s *Server) render(w http.ResponseWriter, name string, data interface{}) {
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Error handler for rendering error pages
func (s *Server) renderError(w http.ResponseWriter, statusCode int, title, message string) {
	w.WriteHeader(statusCode)
	data := map[string]interface{}{
		"Title":      title,
		"Page":       "error",
		"User":       nil,
		"StatusCode": statusCode,
		"Message":    message,
	}
	s.render(w, "base.html", data)
}

// requireAdmin middleware with proper error page rendering
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserFromContext(r.Context())
		if claims == nil || !claims.IsAdmin {
			s.renderError(w, 403, "Access Forbidden", "You don't have permission to access this page.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Page handlers
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Fetch statistics
	challenges, _ := s.db.GetChallengeCount()
	users, _ := s.db.GetUserCount()
	solves, _ := s.db.GetCorrectSubmissionCount()

	customCode, _ := s.db.GetCustomCode("index")

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
	s.render(w, "base.html", data)
}

func (s *Server) handleChallenges(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	visibleOnly := claims == nil || !claims.IsAdmin

	challenges, err := s.db.GetChallenges(visibleOnly)
	if err != nil {
		http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
		return
	}

	categories, _ := s.db.GetAllCategories()
	difficulties, _ := s.db.GetAllDifficulties()
	customCode, _ := s.db.GetCustomCode("challenges")

	data := map[string]interface{}{
		"Title":        "Challenges",
		"Page":         "challenges",
		"User":         claims,
		"Challenges":   challenges,
		"Categories":   categories,
		"Difficulties": difficulties,
		"CustomCode":   customCode,
	}

	// Get completion data for logged-in users
	if claims != nil {
		completions, _ := s.db.GetChallengeCompletionForUser(claims.UserID)
		data["Completions"] = completions
	} else {
		// Set empty completions for unauthenticated users
		data["Completions"] = make(map[string]*database.ChallengeCompletion)
	}

	s.render(w, "base.html", data)
}

func (s *Server) handleChallengeDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	challenge, err := s.db.GetChallengeByID(id)
	if err != nil {
		http.Error(w, "Challenge not found", http.StatusNotFound)
		return
	}

	questions, err := s.db.GetQuestionsByChallengeID(id)
	if err != nil {
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	// Hide flags from non-admin users
	if claims == nil || !claims.IsAdmin {
		for i := range questions {
			questions[i].Flag = ""
		}
	}

	// Get solved questions for user
	solvedQuestions := make(map[string]bool)
	if claims != nil {
		for _, q := range questions {
			solved, _ := s.db.HasUserSolved(q.ID, claims.UserID)
			if solved {
				solvedQuestions[q.ID] = true
			}
		}
	}

	customCode, _ := s.db.GetCustomCode("challenge")

	data := map[string]interface{}{
		"Title":           challenge.Name,
		"Page":            "challenge",
		"User":            claims,
		"Challenge":       challenge,
		"Questions":       questions,
		"SolvedQuestions": solvedQuestions,
		"CustomCode":      customCode,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleScoreboard(w http.ResponseWriter, r *http.Request) {
	entries, err := s.db.GetScoreboard(100)
	if err != nil {
		http.Error(w, "Failed to fetch scoreboard", http.StatusInternalServerError)
		return
	}

	customCode, _ := s.db.GetCustomCode("scoreboard")

	data := map[string]interface{}{
		"Title":      "Scoreboard",
		"Page":       "scoreboard",
		"User":       auth.GetUserFromContext(r.Context()),
		"Entries":    entries,
		"CustomCode": customCode,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := s.db.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	allTeams, err := s.db.GetAllTeams()
	if err != nil {
		allTeams = []models.Team{}
	}

	customCode, _ := s.db.GetCustomCode("teams")

	data := map[string]interface{}{
		"Title":      "Teams",
		"Page":       "teams",
		"User":       user,
		"AllTeams":   allTeams,
		"CustomCode": customCode,
	}

	// If user is in a team, add team details
	if user.TeamID != nil {
		team, err := s.db.GetTeamByID(*user.TeamID)
		if err == nil {
			data["Team"] = team

			// Determine if user can see the invite code
			canSeeInvite := team.OwnerID == claims.UserID ||
				(team.InvitePermission == "all_members")
			data["CanSeeInviteCode"] = canSeeInvite

			members, err := s.db.GetTeamMembers(*user.TeamID)
			if err == nil {
				data["Members"] = members
			}
		}
	}

	s.render(w, "base.html", data)
}

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	// Serve the OpenAPI specification YAML file
	w.Header().Set("Content-Type", "text/yaml")
	// CORS headers are handled by global middleware

	data, err := openapiSpec.ReadFile("docs/openapi.yaml")
	if err != nil {
		http.Error(w, "OpenAPI spec not found", http.StatusNotFound)
		return
	}
	w.Write(data)
}

// handleDocsPage serves the OpenAPI documentation UI
func (s *Server) handleDocsPage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	customCode, _ := s.db.GetCustomCode("docs")

	data := map[string]interface{}{
		"Title":      "API Documentation",
		"User":       claims,
		"Page":       "docs",
		"CustomCode": customCode,
	}

	s.render(w, "base.html", data)
}

func (s *Server) handleSQL(w http.ResponseWriter, r *http.Request) {
	customCode, _ := s.db.GetCustomCode("sql")

	data := map[string]interface{}{
		"Title":      "SQL Playground",
		"Page":       "sql",
		"User":       auth.GetUserFromContext(r.Context()),
		"CustomCode": customCode,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// Check for MOTD: flag takes priority, then database
	motdText := s.motd
	if motdText == "" {
		motdText, _ = s.db.GetSetting("motd")
	}

	customCode, _ := s.db.GetCustomCode("login")

	data := map[string]interface{}{
		"Title":      "Login",
		"Page":       "login",
		"User":       auth.GetUserFromContext(r.Context()),
		"CustomCode": customCode,
		"MOTD":       motdText,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	customCode, _ := s.db.GetCustomCode("register")

	data := map[string]interface{}{
		"Title":      "Register",
		"Page":       "register",
		"User":       auth.GetUserFromContext(r.Context()),
		"CustomCode": customCode,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	customCode, _ := s.db.GetCustomCode("forgot-password")

	data := map[string]interface{}{
		"Title":      "Forgot Password",
		"Page":       "forgot-password",
		"User":       auth.GetUserFromContext(r.Context()),
		"CustomCode": customCode,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	customCode, _ := s.db.GetCustomCode("reset-password")

	data := map[string]interface{}{
		"Title":      "Reset Password",
		"Page":       "reset-password",
		"User":       auth.GetUserFromContext(r.Context()),
		"ResetToken": token,
		"CustomCode": customCode,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())

	// Fetch all challenges (including hidden)
	challenges, err := s.db.GetChallenges(false)
	if err != nil {
		http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
		return
	}

	// Fetch all questions (including challenge names for admin forms)
	questionsWithChallenge, err := s.db.GetAllQuestionsWithChallenge()
	if err != nil {
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	// Fetch all hints
	var hints []models.Hint
	for _, q := range questionsWithChallenge {
		qHints, err := s.db.GetHintsByQuestionID(q.ID)
		if err == nil {
			hints = append(hints, qHints...)
		}
	}

	// Fetch categories, difficulties, and users
	categories, _ := s.db.GetAllCategories()
	difficulties, _ := s.db.GetAllDifficulties()
	users, _ := s.db.GetAllUsers()

	customCode, _ := s.db.GetCustomCode("admin")

	data := map[string]interface{}{
		"Title":        "Admin Dashboard",
		"Page":         "admin",
		"User":         claims,
		"Challenges":   challenges,
		"Questions":    questionsWithChallenge,
		"Hints":        hints,
		"Categories":   categories,
		"Difficulties": difficulties,
		"Users":        users,
		"CustomCode":   customCode,
	}
	s.render(w, "base.html", data)
}

// Edit handlers - return forms for editing challenges/questions
func (s *Server) handleEditChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	challenge, err := s.db.GetChallengeByID(id)
	if err != nil {
		http.Error(w, "Challenge not found", http.StatusNotFound)
		return
	}

	// Get dynamic categories and difficulties
	categories, _ := s.db.GetAllCategories()
	difficulties, _ := s.db.GetAllDifficulties()

	// Build category checkboxes (multi-select) - split current categories
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

	// Build difficulty options
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

	html := fmt.Sprintf(`<div id="challenge-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6 hover:border-purple-500 transition">
		<form hx-put="/api/admin/challenges/%s" hx-target="closest #challenge-%s" hx-swap="outerHTML" class="space-y-3">
			<div>
				<label class="block text-xs font-medium text-gray-300 mb-1">Name</label>
				<input type="text" name="name" value="%s" placeholder="e.g., Web Security 101" class="w-full px-4 py-2 bg-dark-bg border border-dark-border text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>
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
		id, id)

	w.Write([]byte(html))
}

// handleViewChallenge returns a challenge card view (for Cancel button)
func (s *Server) handleViewChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	challenge, err := s.db.GetChallengeByID(id)
	if err != nil {
		http.Error(w, "Challenge not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	difficultyColor := "text-gray-400"
	if d, err := s.db.GetDifficultyByName(challenge.Difficulty); err == nil {
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

	w.Write([]byte(html))
}

// Health check handlers
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check database connectivity
	if err := s.db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"not_ready","checks":{"database":"error: %s"}}`, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready","checks":{"database":"ok"}}`))
}

// Profile handlers
func (s *Server) handleOwnProfile(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	stats, err := s.db.GetUserStats(claims.UserID)
	if err != nil {
		http.Error(w, "Error loading profile", http.StatusInternalServerError)
		return
	}

	submissions, _ := s.db.GetUserRecentSubmissions(claims.UserID, 20)
	solved, _ := s.db.GetUserSolvedChallenges(claims.UserID)

	customCode, _ := s.db.GetCustomCode("profile")

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
	s.render(w, "base.html", data)
}

func (s *Server) handleUserProfile(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	stats, err := s.db.GetUserStats(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	submissions, _ := s.db.GetUserRecentSubmissions(userID, 20)
	solved, _ := s.db.GetUserSolvedChallenges(userID)

	customCode, _ := s.db.GetCustomCode("profile")

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
	s.render(w, "base.html", data)
}

func (s *Server) handleTeamProfile(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	claims := auth.GetUserFromContext(r.Context())

	// Get team details
	team, err := s.db.GetTeamByID(teamID)
	if err != nil {
		s.renderError(w, 404, "Team Not Found", "The team you're looking for doesn't exist.")
		return
	}

	// Get team members
	members, err := s.db.GetTeamMembers(teamID)
	if err != nil {
		http.Error(w, "Failed to fetch team members", http.StatusInternalServerError)
		return
	}

	// Get team owner details
	owner, err := s.db.GetUserByID(team.OwnerID)
	if err != nil {
		http.Error(w, "Failed to fetch team owner", http.StatusInternalServerError)
		return
	}

	// Get team stats from scoreboard
	var teamStats struct {
		TotalPoints int
		SolvedCount int
		Rank        int
	}
	
	// Get rank from team scoreboard
	scoreboard, err := s.db.GetTeamScoreboard(1000)
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

	// Get all team solved challenges (general activity)
	solvedChallenges, _ := s.db.GetTeamSolvedChallenges(teamID)

	// Get scoring team challenges (only first solves count toward team score)
	scoringChallenges, _ := s.db.GetTeamScoringChallenges(teamID)

	// Get all team recent submissions
	recentSubmissions, _ := s.db.GetTeamRecentSubmissions(teamID, 20)

	// Get scoring submissions only (first solves per question)
	scoringSubmissions, _ := s.db.GetTeamScoringSubmissions(teamID, 20)

	customCode, _ := s.db.GetCustomCode("team")

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
	s.render(w, "base.html", data)
}

// Helper handlers for dynamic dropdowns/checkboxes
func (s *Server) handleCategoriesCheckboxes(w http.ResponseWriter, r *http.Request) {
	categories, err := s.db.GetAllCategories()
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
	w.Write([]byte(html.String()))
}

func (s *Server) handleDifficultiesDropdown(w http.ResponseWriter, r *http.Request) {
	difficulties, err := s.db.GetAllDifficulties()
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
	w.Write([]byte(html.String()))
}

// handleEditQuestion returns an edit form for a question
func (s *Server) handleEditQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	question, err := s.db.GetQuestionByID(id)
	if err != nil {
		http.Error(w, "Question not found", http.StatusNotFound)
		return
	}

	// Get all challenges for dropdown
	challenges, _ := s.db.GetChallenges(false)
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
				<input type="text" name="name" value="%s" placeholder="e.g., Find the SQL Injection" class="w-full px-4 py-2 bg-white dark:bg-dark-bg border border-gray-300 dark:border-dark-border text-gray-900 dark:text-white rounded focus:outline-none focus:border-purple-500 text-sm" required>
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

	w.Write([]byte(html))
}

// handleViewQuestion returns a question card view (for Cancel button)
func (s *Server) handleViewQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	question, err := s.db.GetQuestionByID(id)
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

	w.Write([]byte(html))
}
