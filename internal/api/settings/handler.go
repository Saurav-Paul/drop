package settings

// This file defines the HTTP route handlers for the settings API.
// Go equivalent of backend/api/settings/controllers/settings_controller.py.

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/Saurav-Paul/drop/internal/config"
	mw "github.com/Saurav-Paul/drop/internal/middleware" // aliased to "mw" to avoid name clash with echo's middleware
)

// Register wires up the settings domain and attaches routes to the group.
// All settings routes require admin auth.
//
// Usage in main.go:
//
//	settings.Register(e.Group("/api/settings"), db, cfg)
func Register(g *echo.Group, db *gorm.DB, cfg *config.Config) {
	repo := NewRepository(db)
	service := NewService(repo)
	handler := &Handler{service: service}

	// Apply admin middleware to all routes in this group.
	// g.Use() attaches middleware to the group — every request to /api/settings/*
	// must pass admin auth before reaching the handler.
	//
	// Python equivalent:
	//   if not is_admin(request):
	//       raise HTTPException(status_code=401, detail="Admin access required")
	g.Use(mw.RequireAdmin(cfg))

	g.GET("", handler.getSettings)  // GET /api/settings
	g.PUT("", handler.putSettings)  // PUT /api/settings
}

// Handler holds the service dependency and provides HTTP handlers.
type Handler struct {
	service *Service
}

// getSettings returns all settings with defaults applied.
//
// Python equivalent:
//
//	@router.get("", response_model=SettingsResponse)
//	async def get_settings(request: Request):
//	    return settings_service.get_all()
func (h *Handler) getSettings(c echo.Context) error {
	result := h.service.GetAll()
	return c.JSON(http.StatusOK, result)
}

// putSettings updates settings from the request body (partial update).
// Echo's c.Bind() parses the JSON body into the struct — like Pydantic's model validation.
//
// Python equivalent:
//
//	@router.put("", response_model=SettingsResponse)
//	async def update_settings(request: Request, data: SettingsUpdate):
//	    return settings_service.update(data)
func (h *Handler) putSettings(c echo.Context) error {
	var req SettingsUpdate

	// Bind parses the JSON request body into the struct.
	// If the JSON is malformed, it returns a 400 error automatically.
	// Similar to how FastAPI validates the request body via Pydantic.
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	result := h.service.Update(req)
	return c.JSON(http.StatusOK, result)
}
