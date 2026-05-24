package capability

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/damonto/sigmo/internal/app/features"
)

type Handler struct{}

type Response struct {
	Features []string `json:"features"`
}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) List(c *echo.Context) error {
	return c.JSON(http.StatusOK, Response{Features: features.List()})
}
