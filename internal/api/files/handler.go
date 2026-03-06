package files

// This file defines the HTTP route handlers for the files admin API.
// Go equivalent of backend/api/files/controllers/files_controller.py.

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/Saurav-Paul/drop/internal/config"
	mw "github.com/Saurav-Paul/drop/internal/middleware"
)

// Register wires up the files domain and attaches routes.
// All file admin routes require admin auth.
//
// Usage in main.go:
//
//	files.Register(e.Group("/api/files"), db, cfg)
func Register(g *echo.Group, db *gorm.DB, cfg *config.Config) {
	repo := NewRepository(db)
	service := NewService(repo)
	handler := &Handler{service: service}

	// All routes in this group require admin auth
	g.Use(mw.RequireAdmin(cfg))

	g.GET("", handler.listFiles)       // GET /api/files
	g.GET("/:code", handler.getFile)   // GET /api/files/:code
	g.DELETE("/:code", handler.deleteFile) // DELETE /api/files/:code
}

// Handler holds the service dependency and provides HTTP handlers.
type Handler struct {
	service *Service
}

// listFiles returns all files.
//
// Python equivalent:
//
//	@router.get("", response_model=list[FileResponse])
//	async def list_files(request: Request):
//	    return files_service.list_files()
func (h *Handler) listFiles(c echo.Context) error {
	files := h.service.ListFiles()

	// Return empty array instead of null when no files exist
	// Without this, Go would serialize nil slice as "null" in JSON
	if files == nil {
		files = []FileResponse{}
	}

	return c.JSON(http.StatusOK, files)
}

// getFile returns a single file by code.
//
// Python equivalent:
//
//	@router.get("/{code}", response_model=FileResponse)
//	async def get_file(request: Request, code: str):
//	    file = files_service.get_file(code)
//	    if not file:
//	        raise HTTPException(status_code=404)
func (h *Handler) getFile(c echo.Context) error {
	// c.Param() extracts the path parameter — like FastAPI's path parameter
	code := c.Param("code")

	file := h.service.GetFile(code)
	if file == nil {
		return echo.NewHTTPError(http.StatusNotFound, "File not found")
	}

	return c.JSON(http.StatusOK, file)
}

// deleteFile removes a file by code.
//
// Python equivalent:
//
//	@router.delete("/{code}", status_code=status.HTTP_204_NO_CONTENT)
//	async def delete_file(request: Request, code: str):
//	    if not files_service.delete_file(code):
//	        raise HTTPException(status_code=404)
func (h *Handler) deleteFile(c echo.Context) error {
	code := c.Param("code")

	if !h.service.DeleteFile(code) {
		return echo.NewHTTPError(http.StatusNotFound, "File not found")
	}

	// 204 No Content — successful deletion with no response body
	return c.NoContent(http.StatusNoContent)
}
