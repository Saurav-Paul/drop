package main

import (
	"net/http"

	"github.com/labstack/echo/v4"            // Echo — the web framework (like FastAPI)
	"github.com/labstack/echo/v4/middleware"   // Built-in middleware (CORS, logging, etc.)
)

func main() {
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
