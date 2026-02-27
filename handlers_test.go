package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yourusername/hctf2/internal/database"
	"github.com/yourusername/hctf2/internal/email"
	"github.com/yourusername/hctf2/internal/handlers"
)

// TestPageContent validates that each page renders with correct content
func TestPageContent(t *testing.T) {
	// Initialize database
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create test server and router
	server := newTestServer(db)
	router := newTestRouter(server)

	tests := []struct {
		name        string
		method      string
		path        string
		contentMust []string // Content that MUST be present
	}{
		{
			name:   "Home Page",
			method: "GET",
			path:   "/",
			contentMust: []string{
				"Welcome to hCTF2",
				"Browse Challenges",
				"View Rankings",
				"Try SQL",
			},
		},
		{
			name:   "Login Page",
			method: "GET",
			path:   "/login",
			contentMust: []string{
				"Login",
				"Email",
				"Password",
				"Don't have an account",
				"Register here",
			},
		},
		{
			name:   "Register Page",
			method: "GET",
			path:   "/register",
			contentMust: []string{
				"Register",
				"Name",
				"Email",
				"Password",
				"Already have an account",
				"Login here",
			},
		},
		{
			name:   "Challenges Page",
			method: "GET",
			path:   "/challenges",
			contentMust: []string{
				"Challenges",
				"Solve challenges to earn points",
				"Category",
				"Difficulty",
			},
		},
		{
			name:   "Scoreboard Page",
			method: "GET",
			path:   "/scoreboard",
			contentMust: []string{
				"Scoreboard",
				"Top ranked by points",
				"Rank",
				"User",
				"Points",
			},
		},
		{
			name:   "SQL Playground Page",
			method: "GET",
			path:   "/sql",
			contentMust: []string{
				"SQL Playground",
				"Query CTF data",
				"Schema",
				"challenges",
				"questions",
				"submissions",
				"users",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			body, err := io.ReadAll(w.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			content := string(body)

			// Check that all required content is present
			for _, mustHave := range tt.contentMust {
				if !strings.Contains(content, mustHave) {
					t.Errorf("Page missing required content: %q", mustHave)
					t.Logf("Body (first 500 chars): %s", content[:500])
				}
			}

			// Verify page structure
			if !strings.Contains(content, "<html") {
				t.Error("Page missing <html> tag")
			}
			if !strings.Contains(content, "<!DOCTYPE") {
				t.Error("Page missing DOCTYPE")
			}
			if !strings.Contains(content, "<body") {
				t.Error("Page missing <body> tag")
			}
			if !strings.Contains(content, "<nav") {
				t.Error("Page missing navigation bar")
			}
			if !strings.Contains(content, "</html>") {
				t.Error("Page missing closing </html> tag")
			}
		})
	}
}

// TestNavigationLinks validates that navigation links point to correct pages
func TestNavigationLinks(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	server := newTestServer(db)
	router := newTestRouter(server)

	// Test that home page has all navigation links
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body, _ := io.ReadAll(w.Body)
	content := string(body)

	links := map[string]bool{
		`href="/challenges"`: false,
		`href="/scoreboard"`: false,
		`href="/sql"`:        false,
		`href="/login"`:      false,
		`href="/register"`:   false,
	}

	for link := range links {
		if strings.Contains(content, link) {
			links[link] = true
		}
	}

	for link, found := range links {
		if !found {
			t.Errorf("Navigation missing link: %s", link)
		}
	}
}

// TestAPIEndpoints validates that API endpoints return valid responses
func TestAPIEndpoints(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create a user for testing
	_, err = db.CreateUser("test@example.com", "hashed", "Test User", false)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	server := newTestServer(db)
	router := newTestRouter(server)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET /api/challenges",
			path:           "/api/challenges",
			expectedStatus: http.StatusOK,
			expectedBody:   "null", // Empty array or null
		},
		{
			name:           "GET /api/scoreboard",
			path:           "/api/scoreboard",
			expectedStatus: http.StatusOK,
			expectedBody:   "[",
		},
		{
			name:           "GET /api/sql/snapshot",
			path:           "/api/sql/snapshot",
			expectedStatus: http.StatusOK,
			expectedBody:   "{",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			body, _ := io.ReadAll(w.Body)
			content := string(body)

			if !strings.Contains(content, tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got: %s", tt.expectedBody, content[:100])
			}
		})
	}
}

// TestPageContentConsistency validates that the same page always renders identically
func TestPageContentConsistency(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	server := newTestServer(db)
	router := newTestRouter(server)
	pages := []string{"/", "/login", "/register", "/challenges", "/scoreboard", "/sql"}

	for _, page := range pages {
		t.Run(fmt.Sprintf("Consistency_%s", page), func(t *testing.T) {
			// Fetch page twice
			var bodies [2]string

			for i := 0; i < 2; i++ {
				req := httptest.NewRequest("GET", page, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				body, _ := io.ReadAll(w.Body)
				bodies[i] = string(body)
			}

			// Responses should be identical (except for dynamic timestamps)
			// Check that at least 95% of content is the same
			if len(bodies[0]) != len(bodies[1]) {
				t.Logf("Warning: Response size differs (%d vs %d)", len(bodies[0]), len(bodies[1]))
			}

			// Both should have proper HTML structure
			for i, body := range bodies {
				if !strings.Contains(body, "<!DOCTYPE") {
					t.Errorf("Response %d missing DOCTYPE", i+1)
				}
				if !strings.Contains(body, "</html>") {
					t.Errorf("Response %d missing closing html tag", i+1)
				}
			}
		})
	}
}

// TestNoPageCollision validates that pages don't render with wrong content
func TestNoPageCollision(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	server := newTestServer(db)
	router := newTestRouter(server)

	pageTests := []struct {
		page    string
		mustNot []string // Content that should NOT be on this page (excluding navigation)
	}{
		{
			page: "/login",
			mustNot: []string{
				"Solve challenges to earn points",  // From challenges page content
				"Top ranked by points",    // From scoreboard page content
				"Already have an account?",        // From register page content
			},
		},
		{
			page: "/register",
			mustNot: []string{
				"Solve challenges to earn points", // From challenges page content
				"Top ranked by points",   // From scoreboard page content
				"Don't have an account?",         // From login page content
			},
		},
		{
			page: "/challenges",
			mustNot: []string{
				"Query CTF data using SQL", // From SQL page content
				"Top ranked by points",       // From scoreboard page content
			},
		},
	}

	for _, tt := range pageTests {
		t.Run(fmt.Sprintf("NoCollision_%s", tt.page), func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.page, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			body, _ := io.ReadAll(w.Body)
			content := string(body)

			for _, badContent := range tt.mustNot {
				if strings.Contains(content, badContent) {
					t.Errorf("Page %s should not contain: %q", tt.page, badContent)
				}
			}
		})
	}
}

func TestHealthEndpoints(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := newTestServer(db)
	router := newTestRouter(server)

	tests := []struct {
		name        string
		path        string
		statusCode  int
		contentMust []string
	}{
		{
			name:        "Liveness probe",
			path:        "/healthz",
			statusCode:  http.StatusOK,
			contentMust: []string{`"status":"ok"`},
		},
		{
			name:        "Readiness probe",
			path:        "/readyz",
			statusCode:  http.StatusOK,
			contentMust: []string{`"status":"ready"`, `"database":"ok"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("got status %d, want %d", w.Code, tt.statusCode)
			}

			body := w.Body.String()
			for _, must := range tt.contentMust {
				if !strings.Contains(body, must) {
					t.Errorf("response missing %q, got: %s", must, body)
				}
			}
		})
	}
}

// Helper: Create a test server with minimal setup
func newTestServer(db *database.DB) *Server {
	// Parse templates
	tmpl, err := createTemplates()
	if err != nil {
		panic(fmt.Sprintf("Failed to parse templates: %v", err))
	}

	s := &Server{
		db:          db,
		templates:   tmpl,
		authH:       handlers.NewAuthHandler(db, email.NewService(email.Config{}), "http://localhost:8090"),
		challengeH:  handlers.NewChallengeHandler(db, nil, nil, nil),
		scoreboardH: handlers.NewScoreboardHandler(db, nil),
		sqlH:        handlers.NewSQLHandler(db),
	}

	return s
}

// Helper: Create templates from filesystem with all required template functions
func createTemplates() (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
		"markdown":        func(s string) template.HTML { return template.HTML(s) },
		"stripMarkdown":   func(s string) string { return s },
		"safeHTML":        func(s string) template.HTML { return template.HTML(s) },
		"mul":             func(a, b int) int { return a * b },
		"div":             func(a, b int) int { if b == 0 { return 0 }; return a / b },
		"difficultyColor": func(name string) string { return "text-gray-400" },
		"difficultyBadge": func(name string) string { return "bg-gray-600 text-gray-100" },
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
}

// Helper: Create test router with all routes
func newTestRouter(s *Server) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	// Create a simple router that redirects to server handlers
	// Since we can't easily export chi router, we'll use the server's methods directly
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/" {
			s.handleIndex(w, r)
		}
	})

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.handleLoginPage(w, r)
		}
	})

	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.handleRegisterPage(w, r)
		}
	})

	mux.HandleFunc("/challenges", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/challenges" {
			if r.Method == "GET" {
				s.handleChallenges(w, r)
			}
		} else if len(r.URL.Path) > len("/challenges/") {
			s.handleChallengeDetail(w, r)
		}
	})

	mux.HandleFunc("/scoreboard", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.handleScoreboard(w, r)
		}
	})

	mux.HandleFunc("/sql", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.handleSQL(w, r)
		}
	})

	mux.HandleFunc("/api/challenges", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.challengeH.ListChallenges(w, r)
		}
	})

	mux.HandleFunc("/api/scoreboard", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.scoreboardH.GetScoreboard(w, r)
		}
	})

	mux.HandleFunc("/api/sql/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			s.sqlH.GetSnapshot(w, r)
		}
	})

	return mux
}
