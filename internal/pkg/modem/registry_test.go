package modem

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestCreateModemKeepsSIMReferenceWhenPropertiesFail(t *testing.T) {
	registry := &Registry{
		dbusObject: &fakeBusObject{path: ModemManagerObjectPath},
	}
	data := map[string]dbus.Variant{
		"EquipmentIdentifier": dbus.MakeVariant("860588043408833"),
		"Model":               dbus.MakeVariant("RM520N"),
		"State":               dbus.MakeVariant(int32(ModemStateLocked)),
		"UnlockRequired":      dbus.MakeVariant(uint32(ModemLockSimPin)),
		"Sim":                 dbus.MakeVariant(dbus.ObjectPath("/org/freedesktop/ModemManager1/SIM/1")),
	}

	got, err := registry.createModem(context.Background(), "/org/freedesktop/ModemManager1/Modem/1", data)
	if err != nil {
		t.Fatalf("createModem() error = %v", err)
	}
	if got.Sim == nil {
		t.Fatal("SIM = nil, want path reference")
	}
	if got.Sim.Path != "/org/freedesktop/ModemManager1/SIM/1" {
		t.Fatalf("SIM path = %q, want primary SIM path", got.Sim.Path)
	}
}
