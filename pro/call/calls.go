//go:build wifi_calling

package call

import (
	"context"
	"errors"
	"slices"
	"strings"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
	"github.com/damonto/sigmo/internal/pkg/storage"
	"github.com/damonto/sigmo/pro/wificalling"
)

const (
	RouteAuto        = "auto"
	RouteWiFiCalling = "wifi_calling"
	RouteModem       = "modem"

	DirectionIncoming = "incoming"
	DirectionOutgoing = "outgoing"

	StateDialing    = "dialing"
	StateRinging    = "ringing"
	StateAnswering  = "answering"
	StateEarlyMedia = "early_media"
	StateActive     = "active"
	StateConfirmed  = "confirmed"
	StateEnding     = "ending"
	StateEnded      = "ended"
	StateFailed     = "failed"

	HoldNone        = "none"
	HoldLocal       = "local"
	HoldRemote      = "remote"
	HoldLocalRemote = "local_remote"

	ReasonBusy = "busy"
)

var (
	ErrNumberRequired          = errors.New("number is required")
	ErrInvalidNumber           = errors.New("invalid number")
	ErrUSSDDialString          = errors.New("ussd dial strings must use the USSD API")
	ErrInvalidRoute            = errors.New("route must be auto, wifi_calling, or modem")
	ErrNoRouteAvailable        = errors.New("no call route is available")
	ErrWiFiCallingNotConnected = errors.New("wifi calling is not connected")
	ErrModemCallingUnavailable = errors.New("modem calling is not available in this version")
	ErrCallNotFound            = errors.New("call not found")
	ErrInvalidCallState        = errors.New("call state must be active or ended")
	ErrCallRecordActive        = errors.New("active calls cannot be deleted")
	ErrMediaUnavailable        = errors.New("call media is not available")
	ErrUnsupportedCodec        = errors.New("call media codec is not supported")
	ErrUnsupportedDTMF         = errors.New("call dtmf is not supported")
	ErrInvalidCallHold         = errors.New("call hold must be local or none")
	ErrCallUpdateConflict      = errors.New("call update cannot change state and hold together")
	ErrDTMFDigitsRequired      = errors.New("dtmf digits are required")
	ErrInvalidDTMFDigit        = errors.New("dtmf digits must contain 0-9, *, #, or A-D")
	ErrInvalidDTMFCallState    = errors.New("call state must be early_media, active, or confirmed")
	ErrCallOnHold              = errors.New("call is on hold")
)

type Calls struct {
	voice   wifiCallingVoice
	events  *callEvents
	records *callRecords
	routes  *callRoutes
	actions *callActions
	media   *callMedia
}

type Event struct {
	Call storage.Call
}

type UpdateRequest struct {
	State  string
	Reason string
	Hold   string
}

type MediaInfo struct {
	Codec           string
	PayloadType     int
	ClockRate       int
	Channels        int
	OctetAlign      bool
	DTMFPayloadType int
	DTMFClockRate   int
	PTimeMillis     int
}

type MediaSession interface {
	Info() MediaInfo
	ReadPacket(context.Context) ([]byte, error)
	WritePacket(context.Context, []byte) error
}

type wifiCallingVoice interface {
	Status(context.Context, *mmodem.Modem) (wificalling.Status, error)
	DialCall(context.Context, *mmodem.Modem, string) (wificalling.VoiceCall, error)
	AnswerCall(context.Context, *mmodem.Modem, string) (wificalling.VoiceCall, error)
	RejectCall(context.Context, *mmodem.Modem, string) (wificalling.VoiceCall, error)
	HangupCall(context.Context, *mmodem.Modem, string) (wificalling.VoiceCall, error)
	HoldCall(context.Context, *mmodem.Modem, string) (wificalling.VoiceCall, error)
	ResumeCall(context.Context, *mmodem.Modem, string) (wificalling.VoiceCall, error)
	SendCallDTMF(context.Context, *mmodem.Modem, string, string) error
	OpenCallMedia(context.Context, *mmodem.Modem, string) (wificalling.MediaSession, error)
	SubscribeVoiceEvents(wificalling.VoiceEventFunc) func()
}

func New(store *storage.Store, wifiCalling wifiCallingVoice) *Calls {
	events := newCallEvents()
	records := &callRecords{store: store, events: events}
	routes := &callRoutes{wifiCalling: wifiCalling}
	actions := &callActions{wifiCalling: wifiCalling, records: records, routes: routes}
	media := &callMedia{wifiCalling: wifiCalling, records: records}
	return &Calls{
		voice:   wifiCalling,
		events:  events,
		records: records,
		routes:  routes,
		actions: actions,
		media:   media,
	}
}

func (c *Calls) Run(ctx context.Context) error {
	return runVoiceEvents(ctx, c.voice, c.records)
}

func (c *Calls) List(ctx context.Context, modem *mmodem.Modem, query string) ([]storage.Call, error) {
	return c.records.List(ctx, modem, query)
}

func (c *Calls) Dial(ctx context.Context, modem *mmodem.Modem, number string, route string) (storage.Call, error) {
	return c.actions.Dial(ctx, modem, number, route)
}

func (c *Calls) Answer(ctx context.Context, modem *mmodem.Modem, callID string) (storage.Call, error) {
	return c.actions.Answer(ctx, modem, callID)
}

func (c *Calls) Reject(ctx context.Context, modem *mmodem.Modem, callID string) (storage.Call, error) {
	return c.actions.Reject(ctx, modem, callID)
}

func (c *Calls) Update(ctx context.Context, modem *mmodem.Modem, callID string, req UpdateRequest) (storage.Call, error) {
	return c.actions.Update(ctx, modem, callID, req)
}

func (c *Calls) SetHold(ctx context.Context, modem *mmodem.Modem, callID string, hold string) (storage.Call, error) {
	return c.actions.SetHold(ctx, modem, callID, hold)
}

func (c *Calls) Hangup(ctx context.Context, modem *mmodem.Modem, callID string) (storage.Call, error) {
	return c.actions.Hangup(ctx, modem, callID)
}

func (c *Calls) SendDTMF(ctx context.Context, modem *mmodem.Modem, callID string, digits string) error {
	return c.actions.SendDTMF(ctx, modem, callID, digits)
}

func (c *Calls) Delete(ctx context.Context, modem *mmodem.Modem, callID string) error {
	return c.actions.Delete(ctx, modem, callID)
}

func (c *Calls) OpenMedia(ctx context.Context, modem *mmodem.Modem, callID string) (MediaSession, error) {
	return c.media.Open(ctx, modem, callID)
}

func (c *Calls) Subscribe(buffer int) (<-chan Event, func()) {
	return c.events.Subscribe(buffer)
}

func isTerminalCallState(state string) bool {
	return state == StateEnded || state == StateFailed
}

func normalizeHold(hold string) string {
	hold = strings.TrimSpace(hold)
	switch hold {
	case HoldLocal, HoldRemote, HoldLocalRemote:
		return hold
	default:
		return HoldNone
	}
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return RouteAuto
	}
	return route
}

func validRoute(route string) bool {
	return slices.Contains([]string{RouteAuto, RouteWiFiCalling, RouteModem}, route)
}
