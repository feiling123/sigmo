package internet

import (
	"maps"
	"testing"

	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
)

func TestDefaultAPNsFromJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
		want map[string]apnProfile
	}{
		{
			name: "mcc mnc apn",
			json: `[{
				"mcc": "001",
				"mnc": "01",
				"apn": "phone"
			}]`,
			want: map[string]apnProfile{"00101": {APN: "phone"}},
		},
		{
			name: "trim fields",
			json: `[{
				"mcc": " 310 ",
				"mnc": " 260 ",
				"apn": " fast.t-mobile.com "
			}]`,
			want: map[string]apnProfile{"310260": {APN: "fast.t-mobile.com"}},
		},
		{
			name: "default apn credentials",
			json: `[{
				"mcc": "234",
				"mnc": "91",
				"apn": "wap.vodafone.co.uk",
				"protocol": "IPV4V6",
				"user": "wap",
				"pass": "*wap",
				"authType": 1
			}]`,
			want: map[string]apnProfile{"23491": {
				APN:      "wap.vodafone.co.uk",
				IPType:   "ipv4v6",
				Username: "wap",
				Password: "*wap",
				Auth:     "pap",
			}},
		},
		{
			name: "skip incomplete entries",
			json: `[
				{"mcc": "", "mnc": "01", "apn": "missing-mcc"},
				{"mcc": "001", "mnc": "", "apn": "missing-mnc"},
				{"mcc": "001", "mnc": "01", "apn": " "}
			]`,
			want: map[string]apnProfile{},
		},
		{
			name: "keep first apn",
			json: `[
				{"mcc": "001", "mnc": "01", "apn": "first"},
				{"mcc": "001", "mnc": "01", "apn": "second"}
			]`,
			want: map[string]apnProfile{"00101": {APN: "first"}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := defaultAPNsFromJSON([]byte(tt.json))
			if err != nil {
				t.Fatalf("defaultAPNsFromJSON() error = %v", err)
			}
			if !maps.Equal(got, tt.want) {
				t.Fatalf("defaultAPNsFromJSON() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDefaultAPNsFromJSONInvalid(t *testing.T) {
	t.Parallel()

	if _, err := defaultAPNsFromJSON([]byte("{")); err == nil {
		t.Fatal("defaultAPNsFromJSON() error = nil, want error")
	}
}

func TestAndroidAuthType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value *int
		want  string
	}{
		{name: "missing", want: ""},
		{name: "unknown", value: intPtr(-1), want: ""},
		{name: "none", value: intPtr(0), want: "none"},
		{name: "pap", value: intPtr(1), want: "pap"},
		{name: "chap", value: intPtr(2), want: "chap"},
		{name: "pap chap", value: intPtr(3), want: "pap|chap"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := androidAuthType(tt.value); got != tt.want {
				t.Fatalf("androidAuthType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func intPtr(value int) *int {
	return &value
}

func TestSelectAPN(t *testing.T) {
	t.Parallel()

	defaults := map[string]apnProfile{
		"00101": {APN: "json"},
	}
	tests := []struct {
		name      string
		selection apnSelection
		want      string
	}{
		{
			name: "requested wins",
			selection: apnSelection{
				Requested:          " user ",
				Bearer:             "bearer",
				Remembered:         "remembered",
				OperatorIdentifier: "00101",
			},
			want: "user",
		},
		{
			name: "bearer wins",
			selection: apnSelection{
				Bearer:             " bearer ",
				Remembered:         "remembered",
				OperatorIdentifier: "00101",
			},
			want: "bearer",
		},
		{
			name: "remembered wins over default",
			selection: apnSelection{
				Remembered:         " remembered ",
				OperatorIdentifier: "00101",
			},
			want: "remembered",
		},
		{
			name: "default fallback",
			selection: apnSelection{
				OperatorIdentifier: "00101",
			},
			want: "json",
		},
		{
			name: "missing default keeps empty",
			selection: apnSelection{
				OperatorIdentifier: "99999",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.selection.DefaultAPNs = defaults
			if got := selectAPN(tt.selection); got != tt.want {
				t.Fatalf("selectAPN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPreferencesWithDefaultAPNCredentials(t *testing.T) {
	t.Parallel()

	modem := modemAccess{modem: &mmodem.Modem{Sim: &mmodem.SIM{OperatorIdentifier: "23491"}}}
	prefs := Preferences{APN: "wap.vodafone.co.uk"}

	got := preferencesWithDefaultAPNCredentials(modem, prefs)
	if got.APNUsername != "wap" || got.APNPassword != "wap" || got.APNAuth != "pap" {
		t.Fatalf("preferencesWithDefaultAPNCredentials() = %#v, want Vodafone credentials", got)
	}
	if got.IPType != "ipv4v6" {
		t.Fatalf("preferencesWithDefaultAPNCredentials() IPType = %q, want ipv4v6", got.IPType)
	}

	manual := Preferences{
		APN:         "wap.vodafone.co.uk",
		IPType:      "ipv4",
		APNUsername: "custom",
		APNPassword: "secret",
		APNAuth:     "chap",
	}
	got = preferencesWithDefaultAPNCredentials(modem, manual)
	if got.APNUsername != "custom" || got.APNPassword != "secret" || got.APNAuth != "chap" {
		t.Fatalf("preferencesWithDefaultAPNCredentials() = %#v, want manual credentials", got)
	}
	if got.IPType != "ipv4" {
		t.Fatalf("preferencesWithDefaultAPNCredentials() IPType = %q, want ipv4", got.IPType)
	}
}

func TestPreferencesWithSelectedAPN(t *testing.T) {
	t.Parallel()

	modem := modemAccess{modem: &mmodem.Modem{Sim: &mmodem.SIM{OperatorIdentifier: "23491"}}}

	got := preferencesWithSelectedAPN(modem, Preferences{})
	if got.APN != "wap.vodafone.co.uk" {
		t.Fatalf("preferencesWithSelectedAPN() APN = %q, want Vodafone APN", got.APN)
	}
	if got.APNUsername != "wap" || got.APNPassword != "wap" || got.APNAuth != "pap" {
		t.Fatalf("preferencesWithSelectedAPN() = %#v, want Vodafone credentials", got)
	}
	if got.IPType != "ipv4v6" {
		t.Fatalf("preferencesWithSelectedAPN() IPType = %q, want ipv4v6", got.IPType)
	}
}

func TestAPNForModem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		modem      *mmodem.Modem
		remembered string
		want       string
	}{
		{
			name: "remembered wins",
			modem: &mmodem.Modem{
				Sim: &mmodem.SIM{OperatorIdentifier: "00101"},
			},
			remembered: "remembered",
			want:       "remembered",
		},
		{
			name: "default fallback from sim operator",
			modem: &mmodem.Modem{
				Sim: &mmodem.SIM{OperatorIdentifier: "00101"},
			},
			want: "default",
		},
		{
			name:  "missing sim keeps empty",
			modem: &mmodem.Modem{},
			want:  "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := apnForModem(modemAccess{modem: tt.modem}, "", "", tt.remembered); got != tt.want {
				t.Fatalf("apnForModem() = %q, want %q", got, tt.want)
			}
		})
	}
}
