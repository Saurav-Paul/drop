package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"           // Echo — the web framework (like FastAPI)
	"github.com/labstack/echo/v4/middleware" // Built-in middleware (CORS, logging, etc.)

	"github.com/Saurav-Paul/drop/internal/config"   // App configuration from env vars
	"github.com/Saurav-Paul/drop/internal/database" // Database setup and migrations
)

func main() {
	// Load configuration from environment variables
	// Returns an error if directory creation fails
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection and run migrations
	// This creates/opens the SQLite database and applies any pending schema changes
	db, err := database.Setup(cfg)
	if err != nil {
		log.Fatalf("Failed to setup database: %v", err)
	}

	// The underscore (_) means we're intentionally not using these variables yet.
	// They'll be passed to route handlers in later phases.
	// In Go, unused variables cause a compile error — unlike Python which just ignores them.
	_ = cfg
	_ = db

	// Create a new Echo instance — this is the app, like FastAPI() in Python
	e := echo.New()

	// Enable CORS middleware — allows requests from any origin
	// Same as: app.add_middleware(CORSMiddleware, allow_origins=["*"], ...)
	e.Use(middleware.CORS())

	// Register a health check endpoint
	// echo.Map is a shorthand for map[string]interface{} — similar to a dict in Python
	// c.JSON() serializes the map to JSON and sets Content-Type header automatically
	e.GET("/api/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	})

	// Start the server on port 8802
	// e.Logger.Fatal() logs the error and exits if the server fails to start
	e.Logger.Fatal(e.Start(":8802"))
}
