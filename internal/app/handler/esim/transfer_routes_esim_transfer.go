//go:build esim_transfer

package esim

import "github.com/labstack/echo/v5"

func (h *Handler) RegisterTransferRoutes(group *echo.Group) {
	group.GET("/modems/:id/esim-transfer-sources", h.TransferSources)
	group.POST("/modems/:id/esim-transfer-profile-queries", h.TransferProfiles)
	group.GET("/modems/:id/esim-transfer-sessions", h.Transfer)
}
