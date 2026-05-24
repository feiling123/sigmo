//go:build !esim_transfer

package esim

import "github.com/labstack/echo/v5"

func (h *Handler) RegisterTransferRoutes(group *echo.Group) {}
