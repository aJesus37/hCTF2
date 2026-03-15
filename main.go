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
	"embed"
	"io/fs"
	"log"

	"github.com/ajesus37/hCTF2/cmd"
)

// Server is a type alias to cmd.Server so that tests in package main continue to work.
type Server = cmd.Server

// version is set at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

//go:embed internal/views/templates/*
var templatesFS embed.FS

//go:embed internal/views/static
var embedFS embed.FS

//go:embed docs/openapi.yaml
var openapiSpec embed.FS

func main() {
	staticFS, err := fs.Sub(embedFS, "internal/views/static")
	if err != nil {
		log.Fatalf("Failed to create staticFS SubFS: %v", err)
	}

	cmd.SetAssets(cmd.Assets{
		TemplatesFS: templatesFS,
		StaticFS:    staticFS,
		OpenapiSpec: openapiSpec,
	})

	cmd.Execute(version)
}
