package main

// @title hCTF API
// @version 1.0
// @description API documentation for hCTF.
// @host localhost:8080
// @BasePath /api/v1

import (
	"hCTF/internal/app"
	"log"
)

func main() {
	app, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
