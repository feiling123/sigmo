package modemstatus

import (
	"context"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
)

type Fields struct {
	WiFiCallingEnabled   bool `json:"wifiCallingEnabled"`
	WiFiCallingPreferred bool `json:"wifiCallingPreferred"`
	WiFiCallingConnected bool `json:"wifiCallingConnected"`
}

type Extension func(context.Context, *mmodem.Modem, *Fields) error
