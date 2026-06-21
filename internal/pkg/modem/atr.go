package modem

import (
	"context"
	"fmt"

	uiccmbim "github.com/damonto/uicc-go/mbim"
	"github.com/damonto/uicc-go/qcom/uim"
)

type mbimATRReader interface {
	QueryUiccATR(ctx context.Context) ([]byte, error)
	Close() error
}

var openMBIMATRReader = func(ctx context.Context, opts ...uiccmbim.Option) (mbimATRReader, error) {
	return uiccmbim.Open(ctx, opts...)
}

// SupportsEUICC detects eUICC support from modem-exposed card metadata without
// opening the ISD-R logical channel.
func SupportsEUICC(ctx context.Context, m *Modem) (bool, error) {
	if m == nil {
		return false, errModemRequired
	}
	switch m.PrimaryPortType() {
	case ModemPortTypeQmi:
		return supportsQMIEUICC(ctx, m)
	case ModemPortTypeMbim:
		return supportsMBIMEUICC(ctx, m)
	default:
		return false, nil
	}
}

func supportsQMIEUICC(ctx context.Context, m *Modem) (bool, error) {
	requestedSlot, err := qmiSIMSlot(m)
	if err != nil {
		return false, err
	}
	reader, err := openQMIUIMReader(ctx, m.PrimaryPort, requestedSlot)
	if err != nil {
		return false, fmt.Errorf("open QMI UIM reader: %w", err)
	}
	defer closeQMIUIMReader(reader)

	status, err := reader.SlotStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("read QMI UIM slot status: %w", err)
	}
	slot, ok := qmiEUICCSlot(status, m.PrimarySimSlot, requestedSlot)
	if !ok {
		return false, nil
	}
	m.Logger().Debug(
		"read QMI UICC ATR",
		"primarySlot", m.PrimarySimSlot,
		"activeSlot", status.ActiveSlot,
		"atr", formatATR(slot.ATR),
	)
	if atrSupportsEUICC(slot.ATR) {
		return true, nil
	}
	return false, nil
}

func qmiEUICCSlot(status uim.SlotStatus, primarySlot uint32, requestedSlot uint8) (uim.Slot, bool) {
	slot := requestedSlot
	if primarySlot == 0 && status.ActiveSlot != 0 {
		slot = status.ActiveSlot
	}
	index := int(slot) - 1
	if index < 0 || index >= len(status.Slots) {
		return uim.Slot{}, false
	}
	return status.Slots[index], true
}

func supportsMBIMEUICC(ctx context.Context, m *Modem) (bool, error) {
	slot := int(m.PrimarySimSlot)
	if slot == 0 {
		slot = 1
	}
	reader, err := openMBIMATRReader(ctx, uiccmbim.WithProxy(m.PrimaryPort), uiccmbim.WithSlot(slot))
	if err != nil {
		return false, fmt.Errorf("open MBIM reader: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			m.Logger().Debug("close MBIM reader", "error", err)
		}
	}()

	atr, err := reader.QueryUiccATR(ctx)
	if err != nil {
		return false, fmt.Errorf("query MBIM UICC ATR: %w", err)
	}
	m.Logger().Debug("read MBIM UICC ATR", "slot", slot, "atr", formatATR(atr))
	return atrSupportsEUICC(atr), nil
}

func formatATR(atr []byte) string {
	return fmt.Sprintf("% X", atr)
}

func atrSupportsEUICC(atr []byte) bool {
	tb, ok := atrT15GlobalTB(atr)
	return ok && tb&0x82 == 0x82
}

// ETSI TS 102 221 declares eUICC support in TB after a TD byte announces T=15.
func atrT15GlobalTB(atr []byte) (byte, bool) {
	if len(atr) < 2 {
		return 0, false
	}
	if atr[0] != 0x3B && atr[0] != 0x3F {
		return 0, false
	}

	y := atr[1] >> 4
	historicalLen := int(atr[1] & 0x0F)
	index := 2
	protocol := byte(0)
	group := 1
	var tb byte
	found := false
	needsChecksum := false

	for {
		if y&0x1 != 0 {
			if index >= len(atr) {
				return 0, false
			}
			index++
		}
		if y&0x2 != 0 {
			if index >= len(atr) {
				return 0, false
			}
			if protocol == 0x0F && group > 2 {
				tb = atr[index]
				found = true
			}
			index++
		}
		if y&0x4 != 0 {
			if index >= len(atr) {
				return 0, false
			}
			index++
		}
		if y&0x8 == 0 {
			break
		}
		if index >= len(atr) {
			return 0, false
		}
		td := atr[index]
		index++
		y = td >> 4
		protocol = td & 0x0F
		group++
		if protocol != 0 {
			needsChecksum = true
		}
	}
	end := index + historicalLen
	if needsChecksum {
		if end >= len(atr) || end+1 != len(atr) {
			return 0, false
		}
		var checksum byte
		for _, b := range atr[1:] {
			checksum ^= b
		}
		if checksum != 0 {
			return 0, false
		}
		return tb, found
	}
	if end != len(atr) {
		return 0, false
	}
	return tb, found
}
