//go:build wifi_calling

package call

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
	"github.com/damonto/sigmo/internal/pkg/storage"
	"github.com/damonto/sigmo/pro/wificalling"
)

const wifiCallingHangupCleanupTimeout = 3 * time.Second

type callActions struct {
	wifiCalling wifiCallingVoice
	records     *callRecords
	routes      *callRoutes
}

func (a *callActions) Dial(ctx context.Context, modem *mmodem.Modem, number string, route string) (storage.Call, error) {
	number = strings.TrimSpace(number)
	if number == "" {
		return storage.Call{}, ErrNumberRequired
	}
	route = normalizeRoute(route)
	if !validRoute(route) {
		return storage.Call{}, ErrInvalidRoute
	}
	if isUSSDDialString(number) {
		return storage.Call{}, ErrUSSDDialString
	}
	number, err := normalizeDialString(number)
	if err != nil {
		return storage.Call{}, err
	}
	selected, err := a.routes.selectRoute(ctx, modem, route)
	if err != nil {
		return storage.Call{}, err
	}
	switch selected {
	case RouteWiFiCalling:
		call, err := a.wifiCalling.DialCall(ctx, modem, number)
		if err != nil {
			if errors.Is(err, wificalling.ErrNotConnected) {
				return storage.Call{}, ErrWiFiCallingNotConnected
			}
			if call.ID != "" {
				failedCall := callFromWiFiCalling(call)
				if _, saveErr := a.records.saveAndPublish(ctx, failedCall); saveErr != nil {
					return storage.Call{}, errors.Join(fmt.Errorf("dial Wi-Fi Calling: %w", err), fmt.Errorf("save failed call: %w", saveErr))
				}
			}
			return storage.Call{}, fmt.Errorf("dial Wi-Fi Calling: %w", err)
		}
		stored := callFromWiFiCalling(call)
		stored, err = a.records.saveAndPublish(ctx, stored)
		if err != nil {
			return storage.Call{}, err
		}
		return stored, nil
	case RouteModem:
		return storage.Call{}, ErrModemCallingUnavailable
	default:
		return storage.Call{}, ErrNoRouteAvailable
	}
}

func (a *callActions) Answer(ctx context.Context, modem *mmodem.Modem, callID string) (storage.Call, error) {
	call, err := a.records.callForAction(ctx, modem, callID)
	if err != nil {
		return storage.Call{}, err
	}
	switch call.Route {
	case RouteWiFiCalling:
		updated, err := a.wifiCalling.AnswerCall(ctx, modem, call.ID)
		if err := mapWiFiCallingActionError("answer", err); err != nil {
			return storage.Call{}, err
		}
		return a.records.saveAndPublish(ctx, callFromWiFiCalling(updated))
	case RouteModem:
		return storage.Call{}, ErrModemCallingUnavailable
	default:
		return storage.Call{}, ErrInvalidRoute
	}
}

func (a *callActions) Reject(ctx context.Context, modem *mmodem.Modem, callID string) (storage.Call, error) {
	call, err := a.records.callForAction(ctx, modem, callID)
	if err != nil {
		return storage.Call{}, err
	}
	switch call.Route {
	case RouteWiFiCalling:
		updated, err := a.wifiCalling.RejectCall(ctx, modem, call.ID)
		if err := mapWiFiCallingActionError("reject", err); err != nil {
			return storage.Call{}, err
		}
		return a.records.saveAndPublish(ctx, callFromWiFiCalling(updated))
	case RouteModem:
		return storage.Call{}, ErrModemCallingUnavailable
	default:
		return storage.Call{}, ErrInvalidRoute
	}
}

func (a *callActions) Update(ctx context.Context, modem *mmodem.Modem, callID string, req UpdateRequest) (storage.Call, error) {
	req.State = strings.TrimSpace(req.State)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Hold = strings.TrimSpace(req.Hold)
	if req.State != "" && req.Hold != "" {
		return storage.Call{}, ErrCallUpdateConflict
	}
	if req.Hold != "" {
		return a.SetHold(ctx, modem, callID, req.Hold)
	}
	switch req.State {
	case StateActive:
		return a.Answer(ctx, modem, callID)
	case StateEnded:
		if req.Reason == ReasonBusy {
			return a.Reject(ctx, modem, callID)
		}
		return a.Hangup(ctx, modem, callID)
	default:
		return storage.Call{}, ErrInvalidCallState
	}
}

func (a *callActions) SetHold(ctx context.Context, modem *mmodem.Modem, callID string, hold string) (storage.Call, error) {
	hold = strings.TrimSpace(hold)
	if hold != HoldLocal && hold != HoldNone {
		return storage.Call{}, ErrInvalidCallHold
	}
	call, err := a.records.callForAction(ctx, modem, callID)
	if err != nil {
		return storage.Call{}, err
	}
	switch call.Route {
	case RouteWiFiCalling:
		var updated wificalling.VoiceCall
		if hold == HoldLocal {
			updated, err = a.wifiCalling.HoldCall(ctx, modem, call.ID)
		} else {
			updated, err = a.wifiCalling.ResumeCall(ctx, modem, call.ID)
		}
		if err := mapWiFiCallingActionError("update hold", err); err != nil {
			return storage.Call{}, err
		}
		return a.records.saveAndPublish(ctx, callFromWiFiCalling(updated))
	case RouteModem:
		return storage.Call{}, ErrModemCallingUnavailable
	default:
		return storage.Call{}, ErrInvalidRoute
	}
}

func (a *callActions) Hangup(ctx context.Context, modem *mmodem.Modem, callID string) (storage.Call, error) {
	call, err := a.records.callForAction(ctx, modem, callID)
	if err != nil {
		return storage.Call{}, err
	}
	switch call.Route {
	case RouteWiFiCalling:
		ended, err := a.endWiFiCallingCall(ctx, call)
		if err != nil {
			return storage.Call{}, err
		}
		a.cleanupWiFiCallingHangup(ctx, modem, call.ID)
		return ended, nil
	case RouteModem:
		return storage.Call{}, ErrModemCallingUnavailable
	default:
		return storage.Call{}, ErrInvalidRoute
	}
}

func (a *callActions) endWiFiCallingCall(ctx context.Context, call storage.Call) (storage.Call, error) {
	if isTerminalCallState(call.State) {
		return call, nil
	}
	now := time.Now()
	call.State = StateEnded
	call.EndedAt = now
	call.UpdatedAt = now
	return a.records.saveAndPublish(ctx, call)
}

func (a *callActions) cleanupWiFiCallingHangup(ctx context.Context, modem *mmodem.Modem, callID string) {
	go func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), wifiCallingHangupCleanupTimeout)
		defer cancel()
		if _, err := a.wifiCalling.HangupCall(cleanupCtx, modem, callID); err != nil {
			if errors.Is(err, wificalling.ErrNotConnected) || errors.Is(err, wificalling.ErrUnavailable) {
				slog.Debug("clean up Wi-Fi Calling hangup", "call_id", callID, "error", err)
				return
			}
			slog.Warn("clean up Wi-Fi Calling hangup", "call_id", callID, "error", err)
		}
	}()
}

func (a *callActions) SendDTMF(ctx context.Context, modem *mmodem.Modem, callID string, digits string) error {
	digits = strings.TrimSpace(digits)
	if digits == "" {
		return ErrDTMFDigitsRequired
	}
	if !validDTMFDigits(digits) {
		return ErrInvalidDTMFDigit
	}
	call, err := a.records.callForAction(ctx, modem, callID)
	if err != nil {
		return err
	}
	if !dtmfCallState(call.State) {
		return ErrInvalidDTMFCallState
	}
	if call.Hold == HoldLocal || call.Hold == HoldLocalRemote {
		return ErrCallOnHold
	}
	switch call.Route {
	case RouteWiFiCalling:
		if err := a.wifiCalling.SendCallDTMF(ctx, modem, call.ID, digits); err != nil {
			return mapWiFiCallingActionError("send DTMF", err)
		}
		return nil
	case RouteModem:
		return ErrModemCallingUnavailable
	default:
		return ErrInvalidRoute
	}
}

func (a *callActions) Delete(ctx context.Context, modem *mmodem.Modem, callID string) error {
	call, err := a.records.callForAction(ctx, modem, callID)
	if err != nil {
		return err
	}
	return a.records.deleteCall(ctx, call)
}

func dtmfCallState(state string) bool {
	return state == StateEarlyMedia || state == StateActive || state == StateConfirmed
}

func validDTMFDigits(digits string) bool {
	for _, digit := range digits {
		if !validDTMFDigit(digit) {
			return false
		}
	}
	return true
}

func validDTMFDigit(digit rune) bool {
	return digit >= '0' && digit <= '9' ||
		digit == '*' ||
		digit == '#' ||
		digit >= 'A' && digit <= 'D' ||
		digit >= 'a' && digit <= 'd'
}

func mapWiFiCallingActionError(action string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, wificalling.ErrNotConnected):
		return ErrWiFiCallingNotConnected
	case errors.Is(err, wificalling.ErrUnavailable):
		return ErrCallNotFound
	case errors.Is(err, wificalling.ErrUnsupportedDTMF):
		return ErrUnsupportedDTMF
	default:
		return fmt.Errorf("%s Wi-Fi Calling: %w", action, err)
	}
}
