package modem

import (
	"log/slog"
	"strings"
)

func LoggerForIMEI(imei string) *slog.Logger {
	imei = strings.TrimSpace(imei)
	if imei == "" {
		return slog.Default()
	}
	return slog.Default().With("imei", imei)
}

func (m *Modem) Logger() *slog.Logger {
	if m == nil {
		return slog.Default()
	}
	return LoggerForIMEI(m.EquipmentIdentifier)
}
