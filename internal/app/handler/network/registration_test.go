package network

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
	"github.com/damonto/sigmo/internal/pkg/storage"
)

func TestRegistrationScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		modem *mmodem.Modem
		want  string
	}{
		{
			name: "profile scope wins",
			modem: &mmodem.Modem{
				EquipmentIdentifier: "imei-1",
				Sim:                 &mmodem.SIM{Identifier: "iccid-1"},
			},
			want: "profile:iccid-1",
		},
		{
			name: "modem fallback",
			modem: &mmodem.Modem{
				EquipmentIdentifier: "imei-1",
			},
			want: "modem:imei-1",
		},
		{
			name:  "empty identifiers",
			modem: &mmodem.Modem{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := registrationScope(tt.modem); got != tt.want {
				t.Fatalf("registrationScope() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunRegistrationRestoreRequiresStorage(t *testing.T) {
	t.Parallel()

	err := RunRegistrationRestore(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("RunRegistrationRestore() error = nil, want storage error")
	}
	if !errors.Is(err, errNetworkRegistrationStorageRequired) {
		t.Fatalf("RunRegistrationRestore() error = %v, want storage error", err)
	}
}

func TestNewNetworkRequiresDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		preferences func(t *testing.T) *mmodem.NetworkPreferences
		store       func(t *testing.T) *storage.Store
		want        error
	}{
		{
			name: "preferences required",
			store: func(t *testing.T) *storage.Store {
				t.Helper()
				return openNetworkTestStore(t)
			},
			want: errNetworkPreferencesRequired,
		},
		{
			name: "storage required",
			preferences: func(t *testing.T) *mmodem.NetworkPreferences {
				t.Helper()
				prefs, err := mmodem.NewNetworkPreferences(openNetworkTestStore(t))
				if err != nil {
					t.Fatalf("NewNetworkPreferences() error = %v", err)
				}
				return prefs
			},
			want: errNetworkRegistrationStorageRequired,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var preferences *mmodem.NetworkPreferences
			if tt.preferences != nil {
				preferences = tt.preferences(t)
			}
			var store *storage.Store
			if tt.store != nil {
				store = tt.store(t)
			}
			_, err := newNetwork(preferences, store)
			if err == nil {
				t.Fatal("newNetwork() error = nil, want dependency error")
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("newNetwork() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func openNetworkTestStore(t *testing.T) *storage.Store {
	t.Helper()

	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "sigmo.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return db
}
