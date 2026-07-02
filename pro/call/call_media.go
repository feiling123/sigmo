//go:build wifi_calling

package call

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
	"github.com/damonto/sigmo/internal/pkg/storage"
	"github.com/damonto/sigmo/pro/wificalling"
)

type callMedia struct {
	wifiCalling wifiCallingVoice
	records     *callRecords
}

func (m *callMedia) Open(ctx context.Context, modem *mmodem.Modem, callID string) (MediaSession, error) {
	call, err := m.records.callForAction(ctx, modem, callID)
	if err != nil {
		return nil, err
	}
	switch call.Route {
	case RouteWiFiCalling:
		session, err := m.wifiCalling.OpenCallMedia(ctx, modem, call.ID)
		if errors.Is(err, wificalling.ErrUnavailable) {
			m.endUnavailable(ctx, call)
		}
		if err := mapWiFiCallingMediaError(err); err != nil {
			return nil, err
		}
		return wifiCallingMediaSession{session: session}, nil
	case RouteModem:
		return nil, ErrModemCallingUnavailable
	default:
		return nil, ErrInvalidRoute
	}
}

func (m *callMedia) endUnavailable(ctx context.Context, call storage.Call) {
	if isTerminalCallState(call.State) {
		return
	}
	now := time.Now()
	call.State = StateEnded
	call.Reason = ErrMediaUnavailable.Error()
	call.EndedAt = now
	call.UpdatedAt = now
	if err := m.records.store.SaveCall(ctx, call); err != nil {
		slog.Warn("save Wi-Fi Calling call after media became unavailable",
			"call_id", call.ID,
			"modem_id", call.ModemID,
			"profile_id", call.ProfileID,
			"error", err,
		)
		return
	}
	m.records.events.publish(Event{Call: call})
}

type wifiCallingMediaSession struct {
	session wificalling.MediaSession
}

func (s wifiCallingMediaSession) Info() MediaInfo {
	info := s.session.Info()
	return MediaInfo{
		Codec:           info.Codec,
		PayloadType:     info.PayloadType,
		ClockRate:       info.ClockRate,
		Channels:        info.Channels,
		OctetAlign:      info.OctetAlign,
		DTMFPayloadType: info.DTMFPayloadType,
		DTMFClockRate:   info.DTMFClockRate,
		PTimeMillis:     info.PTimeMillis,
	}
}

func (s wifiCallingMediaSession) ReadPacket(ctx context.Context) ([]byte, error) {
	return s.session.ReadPacket(ctx)
}

func (s wifiCallingMediaSession) WritePacket(ctx context.Context, packet []byte) error {
	return s.session.WritePacket(ctx, packet)
}

func mapWiFiCallingMediaError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, wificalling.ErrUnsupportedCodec):
		return ErrUnsupportedCodec
	case errors.Is(err, wificalling.ErrUnavailable):
		return ErrMediaUnavailable
	case errors.Is(err, wificalling.ErrNotConnected):
		return ErrWiFiCallingNotConnected
	default:
		return fmt.Errorf("open Wi-Fi Calling media: %w", err)
	}
}
