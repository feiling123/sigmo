//go:build wifi_calling

package call

import (
	"context"
	"log/slog"

	"github.com/damonto/sigmo/pro/wificalling"
)

func runVoiceEvents(ctx context.Context, wifiCalling wifiCallingVoice, records *callRecords) error {
	if wifiCalling == nil {
		<-ctx.Done()
		return nil
	}
	unsubscribe := wifiCalling.SubscribeVoiceEvents(func(event wificalling.VoiceEvent) {
		if event.Call.ID == "" {
			return
		}
		call := callFromWiFiCalling(event.Call)
		if _, err := records.saveAndPublish(ctx, call); err != nil {
			slog.Warn("save Wi-Fi Calling voice event",
				"call_id", call.ID,
				"modem_id", call.ModemID,
				"profile_id", call.ProfileID,
				"state", call.State,
				"error", err,
			)
			records.events.publish(Event{Call: call})
			return
		}
	})
	defer unsubscribe()
	<-ctx.Done()
	return nil
}
