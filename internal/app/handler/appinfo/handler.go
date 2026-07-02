package appinfo

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	version string
}

type Response struct {
	Version string `json:"version"`
}

func New(version string) *Handler {
	if version == "" {
		version = "dev"
	}
	return &Handler{version: version}
}

func (h *Handler) Get(c *echo.Context) error {
	return c.JSON(http.StatusOK, Response{Version: h.version})
}
