//go:build wifi_calling

package call

import (
	"context"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
)

type callRoutes struct {
	wifiCalling wifiCallingVoice
}

func (r *callRoutes) selectRoute(ctx context.Context, modem *mmodem.Modem, requested string) (string, error) {
	switch requested {
	case RouteWiFiCalling:
		status, err := r.wifiCalling.Status(ctx, modem)
		if err != nil {
			return "", err
		}
		if !status.Connected {
			return "", ErrWiFiCallingNotConnected
		}
		return RouteWiFiCalling, nil
	case RouteModem:
		return RouteModem, nil
	}
	status, err := r.wifiCalling.Status(ctx, modem)
	if err != nil {
		return "", err
	}
	if status.Connected {
		return RouteWiFiCalling, nil
	}
	return "", ErrNoRouteAvailable
}
