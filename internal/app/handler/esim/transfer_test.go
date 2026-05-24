//go:build esim_transfer

package esim

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	sgp22 "github.com/damonto/euicc-go/v2"
	"github.com/damonto/sigmo/internal/pkg/config"
	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
	"github.com/damonto/ts43-go/sim"
	"github.com/damonto/ts43-go/ts43"
)

func TestTransferCandidateSupport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		candidate transferProfileCandidate
		wantOK    bool
	}{
		{
			name:      "enabled esim with entitlement",
			candidate: esimTransferCandidate(testTransferProfile(sgp22.ProfileEnabled)),
			wantOK:    true,
		},
		{
			name:      "disabled esim with entitlement is transferable",
			candidate: esimTransferCandidate(testTransferProfile(sgp22.ProfileDisabled)),
			wantOK:    true,
		},
		{
			name: "physical sim with entitlement",
			candidate: physicalTransferCandidate(sim.Identity{
				ICCID: "8900000000000000000",
				MCC:   "204",
				MNC:   "08",
			}),
			wantOK: true,
		},
		{
			name:      "esim without entitlement is unsupported",
			candidate: esimTransferCandidate(testUnsupportedTransferProfile()),
		},
		{
			name: "physical sim without entitlement is unsupported",
			candidate: physicalTransferCandidate(sim.Identity{
				ICCID: "8900000000000000000",
				MCC:   "999",
				MNC:   "99",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.candidate.response.Supported != tt.wantOK {
				t.Fatalf("Supported = %v, want %v", tt.candidate.response.Supported, tt.wantOK)
			}
		})
	}
}

func TestValidateTransferStartRequiresCCIDIMEI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		start   transferStart
		wantErr error
	}{
		{
			name: "ccid requires source imei when transfer starts",
			start: transferStart{
				SourceType: transferSourceCCID,
				SourceID:   "reader-1",
				ProfileID:  "profile-1",
			},
			wantErr: errSourceIMEIRequired,
		},
		{
			name: "ccid with source imei",
			start: transferStart{
				SourceType: transferSourceCCID,
				SourceID:   "reader-1",
				ProfileID:  "profile-1",
				SourceIMEI: "123456789012345",
			},
		},
		{
			name: "modem source does not require extra imei input",
			start: transferStart{
				SourceType: transferSourceModem,
				SourceID:   "modem-1",
				ProfileID:  "profile-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransferStart(tt.start)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateTransferStart() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTransferTargetRejectsSourceTargetModem(t *testing.T) {
	t.Parallel()

	target := &mmodem.Modem{EquipmentIdentifier: "target-imei"}
	tests := []struct {
		name    string
		start   transferStart
		wantErr error
	}{
		{
			name:    "same modem source and target",
			start:   transferStart{SourceType: transferSourceModem, SourceID: "target-imei"},
			wantErr: errTransferSourceIsTarget,
		},
		{
			name:  "different modem source",
			start: transferStart{SourceType: transferSourceModem, SourceID: "source-imei"},
		},
		{
			name:  "ccid source can target modem",
			start: transferStart{SourceType: transferSourceCCID, SourceID: "target-imei"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransferTarget(target, tt.start)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateTransferTarget() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestTransferModemName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   config.Config
		modem *mmodem.Modem
		want  string
	}{
		{
			name: "configured alias wins",
			cfg: config.Config{Modems: map[string]config.Modem{
				"imei-1": {Alias: "Office"},
			}},
			modem: &mmodem.Modem{
				EquipmentIdentifier: "imei-1",
				Model:               "RM520N-GL",
			},
			want: "Office",
		},
		{
			name: "model fallback matches modem overview",
			cfg:  config.Config{},
			modem: &mmodem.Modem{
				EquipmentIdentifier: "imei-2",
				Manufacturer:        "Quectel",
				Model:               "RM520N-GL",
			},
			want: "RM520N-GL",
		},
		{
			name: "empty model stays empty",
			cfg:  config.Config{},
			modem: &mmodem.Modem{
				EquipmentIdentifier: "imei-3",
				Manufacturer:        "Quectel",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transferModemName(&tt.cfg, tt.modem); got != tt.want {
				t.Fatalf("transferModemName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCCIDServiceUnavailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "pcsc no service code",
			err:  errors.New("scardEstablishContext() returned 0x8010001D"),
			want: true,
		},
		{
			name: "pcsc no service name",
			err:  errors.New("scard failure: SCARD_E_NO_SERVICE (Service not available.)"),
			want: true,
		},
		{
			name: "other ccid error",
			err:  errors.New("scard failure: SCARD_E_NO_READERS_AVAILABLE"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ccidServiceUnavailable(tt.err); got != tt.want {
				t.Fatalf("ccidServiceUnavailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSourceEndpointCloseOnce(t *testing.T) {
	t.Parallel()

	calls := 0
	source := &sourceEndpoint{
		release: func() {
			calls++
		},
	}

	source.Close()
	source.Close()

	if calls != 1 {
		t.Fatalf("release calls = %d, want 1", calls)
	}
}

func TestActiveTransferCloseAllowsMissingTargetClient(t *testing.T) {
	t.Parallel()

	calls := 0
	transfer := &activeTransfer{
		source: &sourceEndpoint{
			release: func() {
				calls++
			},
		},
	}

	transfer.Close()
	transfer.Close()

	if calls != 1 {
		t.Fatalf("source release calls = %d, want 1", calls)
	}
}

func TestTransferSessionCancelCancelsContext(t *testing.T) {
	t.Parallel()

	done := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = newTransferSession(conn, cancel)

		select {
		case <-ctx.Done():
			done <- nil
		case <-time.After(time.Second):
			done <- errors.New("transfer session did not cancel")
		}
	}))
	defer server.Close()

	conn := dialTransferTestWebSocket(t, server.URL)
	if err := conn.WriteJSON(transferClientMessage{Type: wsTypeCancel}); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestTransferSessionWaitForStartStopsOnDisconnect(t *testing.T) {
	t.Parallel()

	done := make(chan bool, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			done <- true
			return
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		session := newTransferSession(conn, cancel)
		_, ok := session.waitForStart(ctx)
		done <- ok
	}))
	defer server.Close()

	conn := dialTransferTestWebSocket(t, server.URL)
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if ok := <-done; ok {
		t.Fatal("waitForStart() ok = true, want false")
	}
}

func dialTransferTestWebSocket(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(rawURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	return conn
}

func TestTS43SourceSIMType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		profileType transferProfileType
		want        ts43.SIMType
	}{
		{name: "eSIM source", profileType: transferProfileESIM, want: ts43.SIMTypeESIM},
		{name: "pSIM source", profileType: transferProfilePhysical, want: ts43.SIMTypePSIM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ts43SourceSIMType(tt.profileType); got != tt.want {
				t.Fatalf("ts43SourceSIMType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSMDSDiscoveryEventFromDelayedDownload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event ts43.DelayedDownloadEvent
		want  ts43.SMDSDiscoveryEvent
	}{
		{
			name: "preserves delayed download fields",
			event: ts43.DelayedDownloadEvent{
				SourceICCID:        "8910000000000000000",
				TargetEID:          "89049032000001000000000000000000",
				TargetIMEI:         "222222222222222",
				SubscriptionResult: ts43.SubscriptionResultDelayedDownload,
			},
			want: ts43.SMDSDiscoveryEvent{
				SourceICCID:        "8910000000000000000",
				TargetEID:          "89049032000001000000000000000000",
				TargetIMEI:         "222222222222222",
				SubscriptionResult: ts43.SubscriptionResultDelayedDownload,
			},
		},
		{
			name: "keeps empty optional fields empty",
			event: ts43.DelayedDownloadEvent{
				SubscriptionResult: ts43.SubscriptionResultDelayedDownload,
			},
			want: ts43.SMDSDiscoveryEvent{
				SubscriptionResult: ts43.SubscriptionResultDelayedDownload,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := smdsDiscoveryEventFromDelayedDownload(tt.event); got != tt.want {
				t.Fatalf("smdsDiscoveryEventFromDelayedDownload() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func testTransferProfile(state sgp22.ProfileState) *sgp22.ProfileInfo {
	return &sgp22.ProfileInfo{
		ICCID:               sgp22.ICCID{0x98, 0x10},
		ProfileState:        state,
		ProfileNickname:     "Boost",
		ServiceProviderName: "Boost",
		ProfileOwner: sgp22.OperatorId{
			PLMN: []byte{0x02, 0xf8, 0x90},
			GID1: []byte{0x63, 0x32},
		},
	}
}

func testUnsupportedTransferProfile() *sgp22.ProfileInfo {
	profile := testTransferProfile(sgp22.ProfileEnabled)
	profile.ProfileOwner = sgp22.OperatorId{
		PLMN: []byte{0x99, 0xf9, 0x99},
	}
	return profile
}
