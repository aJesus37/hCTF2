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
	"github.com/yourusername/hctf2/internal/handlers"
	"github.com/yourusername/hctf2/internal/models"
	"github.com/yourusername/hctf2/internal/utils"
)

//go:embed internal/views/templates/*
var templatesFS embed.FS

//go:embed internal/views/static
var embedFS embed.FS

// staticFS is a SubFS starting at internal/views/static
var staticFS fs.FS

func init() {
	var err error
	staticFS, err = fs.Sub(embedFS, "internal/views/static")
	if err != nil {
		log.Fatalf("Failed to create staticFS SubFS: %v", err)
	}
}

type Server struct {
	db        *database.DB
	templates *template.Template
	authH     *handlers.AuthHandler
	challengeH *handlers.ChallengeHandler
	scoreboardH *handlers.ScoreboardHandler
	teamH     *handlers.TeamHandler
	hintH     *handlers.HintHandler
	sqlH      *handlers.SQLHandler
	profileH  *handlers.ProfileHandler
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

	// Set CORS headers for all static files
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

	h.fs.ServeHTTP(w, r)
}

func main() {
	var (
		port        = flag.Int("port", 8090, "Server port")
		dbPath      = flag.String("db", "./hctf2.db", "Database path")
		adminEmail  = flag.String("admin-email", "", "Admin email for first-time setup")
		adminPass   = flag.String("admin-password", "", "Admin password for first-time setup")
	)
	flag.Parse()

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
		"mul":           func(a, b int) int { return a * b },
		"div":           func(a, b int) int { if b == 0 { return 0 }; return a / b },
	}).ParseFS(templatesFS, "internal/views/templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Initialize server
	s := &Server{
		db:          db,
		templates:   tmpl,
		authH:       handlers.NewAuthHandler(db),
		challengeH:  handlers.NewChallengeHandler(db),
		scoreboardH: handlers.NewScoreboardHandler(db),
		teamH:       handlers.NewTeamHandler(db),
		hintH:       handlers.NewHintHandler(db),
		sqlH:        handlers.NewSQLHandler(db),
		profileH:    handlers.NewProfileHandler(db),
	}

	// Setup router
	r := chi.NewRouter()

	// CORS middleware for CDN resources and DuckDB WASM
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow cross-origin requests for static files (needed for web workers)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Only set restrictive COEP/COOP headers for SQL page (DuckDB WASM)
			// These headers block external CDN resources, so we only use them where needed
			if r.URL.Path == "/sql" {
				w.Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
				w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			}

			// For static files, also allow shared array buffers
			if strings.HasPrefix(r.URL.Path, "/static/") {
				w.Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(auth.AuthMiddleware)

	// Static files with proper content types for WASM and workers
	r.Handle("/static/*", http.StripPrefix("/static/", &customFileHandler{
		fs: http.FileServer(http.FS(staticFS)),
	}))

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
	})

	// API routes - SQL
	r.Get("/api/sql/snapshot", s.sqlH.GetSnapshot)

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

	data := map[string]interface{}{
		"Title": "Home",
		"Page":  "index",
		"User":  auth.GetUserFromContext(r.Context()),
		"Stats": map[string]int{
			"Challenges": challenges,
			"Users":      users,
			"Solves":     solves,
		},
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

	data := map[string]interface{}{
		"Title":      "Challenges",
		"Page":       "challenges",
		"User":       claims,
		"Challenges": challenges,
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

	data := map[string]interface{}{
		"Title":           challenge.Name,
		"Page":            "challenge",
		"User":            claims,
		"Challenge":       challenge,
		"Questions":       questions,
		"SolvedQuestions": solvedQuestions,
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleScoreboard(w http.ResponseWriter, r *http.Request) {
	entries, err := s.db.GetScoreboard(100)
	if err != nil {
		http.Error(w, "Failed to fetch scoreboard", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":   "Scoreboard",
		"Page":    "scoreboard",
		"User":    auth.GetUserFromContext(r.Context()),
		"Entries": entries,
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

	data := map[string]interface{}{
		"Title":   "Teams",
		"Page":    "teams",
		"User":    user,
		"AllTeams": allTeams,
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

func (s *Server) handleSQL(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "SQL Playground",
		"Page":  "sql",
		"User":  auth.GetUserFromContext(r.Context()),
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Login",
		"Page":  "login",
		"User":  auth.GetUserFromContext(r.Context()),
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Register",
		"Page":  "register",
		"User":  auth.GetUserFromContext(r.Context()),
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleForgotPasswordPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Forgot Password",
		"Page":  "forgot-password",
		"User":  auth.GetUserFromContext(r.Context()),
	}
	s.render(w, "base.html", data)
}

func (s *Server) handleResetPasswordPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	data := map[string]interface{}{
		"Title":      "Reset Password",
		"Page":       "reset-password",
		"User":       auth.GetUserFromContext(r.Context()),
		"ResetToken": token,
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

	data := map[string]interface{}{
		"Title":      "Admin Dashboard",
		"Page":       "admin",
		"User":       claims,
		"Challenges": challenges,
		"Questions":  questionsWithChallenge,
		"Hints":      hints,
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

	w.Header().Set("Content-Type", "text/html")
	visibleChecked := ""
	if challenge.Visible {
		visibleChecked = "checked"
	}

	html := fmt.Sprintf(`<div id="challenge-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6">
		<div class="bg-dark-bg border border-dark-border rounded p-4 space-y-4 mb-4">
			<h4 class="text-lg font-bold text-white">Edit Challenge</h4>
			<form hx-put="/api/admin/challenges/%s" hx-target="closest #challenge-%s" hx-swap="outerHTML" class="space-y-3">
				<input type="text" name="name" value="%s" placeholder="Challenge name" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>
				<textarea name="description" placeholder="Description" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>%s</textarea>
				<select name="category" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>
					<option value="web" %s>Web</option>
					<option value="crypto" %s>Crypto</option>
					<option value="pwn" %s>Pwn</option>
					<option value="forensics" %s>Forensics</option>
					<option value="misc" %s>Misc</option>
				</select>
				<select name="difficulty" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>
					<option value="easy" %s>Easy</option>
					<option value="medium" %s>Medium</option>
					<option value="hard" %s>Hard</option>
				</select>
				<label class="flex items-center text-sm text-gray-300">
					<input type="checkbox" name="visible" value="on" %s class="mr-2"> Visible to users
				</label>
				<div class="flex gap-2">
					<button type="submit" class="px-3 py-1 bg-green-600 hover:bg-green-700 text-white rounded text-sm">Save</button>
					<button type="button" hx-get="/admin/challenges/%s/view" hx-target="closest #challenge-%s" hx-swap="outerHTML" class="px-3 py-1 bg-gray-600 hover:bg-gray-700 text-white rounded text-sm">Cancel</button>
				</div>
			</form>
		</div>
	</div>`,
		id, id, id,
		challenge.Name,
		challenge.Description,
		map[string]string{"web": "selected", "crypto": "", "pwn": "", "forensics": "", "misc": ""}[challenge.Category],
		map[string]string{"web": "", "crypto": "selected", "pwn": "", "forensics": "", "misc": ""}[challenge.Category],
		map[string]string{"web": "", "crypto": "", "pwn": "selected", "forensics": "", "misc": ""}[challenge.Category],
		map[string]string{"web": "", "crypto": "", "pwn": "", "forensics": "selected", "misc": ""}[challenge.Category],
		map[string]string{"web": "", "crypto": "", "pwn": "", "forensics": "", "misc": "selected"}[challenge.Category],
		map[string]string{"easy": "selected", "medium": "", "hard": ""}[challenge.Difficulty],
		map[string]string{"easy": "", "medium": "selected", "hard": ""}[challenge.Difficulty],
		map[string]string{"easy": "", "medium": "", "hard": "selected"}[challenge.Difficulty],
		visibleChecked,
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

	difficultyColor := map[string]string{
		"easy":   "text-green-400",
		"medium": "text-yellow-400",
		"hard":   "text-red-400",
	}[challenge.Difficulty]

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
				hx-get="/admin/challenges/%s/edit"
				hx-target="#challenge-%s"
				hx-swap="outerHTML"
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

	data := map[string]interface{}{
		"Title":             "My Profile",
		"Page":              "profile",
		"User":              claims,
		"Stats":             stats,
		"RecentSubmissions": submissions,
		"SolvedChallenges":  solved,
		"IsOwnProfile":      true,
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

	data := map[string]interface{}{
		"Title":             "User Profile",
		"Page":              "profile",
		"User":              claims,
		"Stats":             stats,
		"RecentSubmissions": submissions,
		"SolvedChallenges":  solved,
		"IsOwnProfile":      false,
	}
	s.render(w, "base.html", data)
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

	html := fmt.Sprintf(`<div id="question-%s" class="bg-dark-surface border border-dark-border rounded-lg p-6">
		<div class="bg-dark-bg border border-dark-border rounded p-4 space-y-4 mb-4">
			<h4 class="text-lg font-bold text-white">Edit Question</h4>
			<form hx-put="/api/admin/questions/%s" hx-target="closest #question-%s" hx-swap="outerHTML" class="space-y-3">
				<div>
					<label class="block text-xs font-medium text-gray-400 mb-1">Challenge</label>
					<select name="challenge_id" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>
						%s
					</select>
				</div>
				<div>
					<label class="block text-xs font-medium text-gray-400 mb-1">Question Name</label>
					<input type="text" name="name" value="%s" placeholder="e.g., Find the SQL Injection" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>
				</div>
				<div>
					<label class="block text-xs font-medium text-gray-400 mb-1">Description</label>
					<textarea name="description" placeholder="Question description and hints..." class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>%s</textarea>
				</div>
				<div>
					<label class="block text-xs font-medium text-gray-400 mb-1">Flag</label>
					<input type="text" name="flag" value="%s" placeholder="flag{...}" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>
				</div>
				<div>
					<label class="block text-xs font-medium text-gray-400 mb-1">Points</label>
					<input type="number" name="points" value="%d" placeholder="100" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm" required>
				</div>
				<div>
					<label class="block text-xs font-medium text-gray-400 mb-1">Flag Mask (leave empty to auto-generate)</label>
					<input type="text" name="flag_mask" value="%s" placeholder="flag{****}" class="w-full px-3 py-2 bg-dark-bg border border-dark-border text-white rounded text-sm">
				</div>
				<label class="flex items-center text-sm text-gray-300 cursor-pointer">
					<input type="checkbox" name="case_sensitive" value="on" %s class="mr-2"> Case sensitive flag
				</label>
				<div class="flex gap-2 pt-2">
					<button type="submit" class="px-3 py-1 bg-green-600 hover:bg-green-700 text-white rounded text-sm">Save</button>
					<button type="button" hx-get="/admin/questions/%s/view" hx-target="closest #question-%s" hx-swap="outerHTML" class="px-3 py-1 bg-gray-600 hover:bg-gray-700 text-white rounded text-sm">Cancel</button>
				</div>
			</form>
		</div>
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
				hx-get="/admin/questions/%s/edit"
				hx-target="#question-%s"
				hx-swap="outerHTML"
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
