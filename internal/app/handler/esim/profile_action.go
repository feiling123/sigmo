package esim

import (
	"context"
	"errors"
	"fmt"

	sgp22 "github.com/damonto/euicc-go/v2"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
)

func (h *Handler) EnableProfile(ctx context.Context, modem *mmodem.Modem, seID string, iccid sgp22.ICCID) error {
	session, err := h.lifecycle.PrepareEnable(modem, seID, iccid)
	if err != nil {
		if errors.Is(err, errProfileAlreadyActive) {
			return nil
		}
		return err
	}
	defer session.Close()

	sessionCtx, cancel := context.WithTimeout(ctx, enableTimeout)
	defer cancel()
	if err := h.restoreInternetBeforeProfileEnable(sessionCtx, modem); err != nil {
		return fmt.Errorf("restore internet connection: %w", err)
	}
	return session.Enable(sessionCtx)
}

func (h *Handler) DeleteProfile(modem *mmodem.Modem, seID string, iccid sgp22.ICCID) error {
	return h.lifecycle.Delete(modem, seID, iccid)
}
