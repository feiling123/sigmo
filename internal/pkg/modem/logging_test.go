package modem

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLoggerForIMEI(t *testing.T) {
	tests := []struct {
		name     string
		imei     string
		wantIMEI bool
	}{
		{name: "with IMEI", imei: " 860588043408833 ", wantIMEI: true},
		{name: "blank IMEI", imei: " "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			defer slog.SetDefault(previous)

			LoggerForIMEI(tt.imei).Info("test log")
			got := logs.String()
			hasIMEI := strings.Contains(got, "imei=860588043408833")
			if hasIMEI != tt.wantIMEI {
				t.Fatalf("logs = %s, imei field present = %v, want %v", got, hasIMEI, tt.wantIMEI)
			}
		})
	}
}
