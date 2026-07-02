package lpa

import (
	"bytes"
	"testing"
)

func TestESTKmeSEsForSKU(t *testing.T) {
	tests := []struct {
		name    string
		skuName string
		wantOK  bool
		wantIDs []string
	}{
		{
			name:    "ESTKme Max",
			skuName: "ESTKme Max",
			wantOK:  true,
			wantIDs: []string{SEID0, SEID1},
		},
		{
			name:    "ESTKme Plus+",
			skuName: " ESTKme Plus+ ",
			wantOK:  true,
			wantIDs: []string{SEID0, SEID1},
		},
		{
			name:    "short Max is not dual SE",
			skuName: "Max",
		},
		{
			name:    "unsupported",
			skuName: "Mini",
		},
		{
			name:    "short plus suffix is not dual SE",
			skuName: "Plus+",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := estkmeSEsForSKU(tt.skuName)
			if ok != tt.wantOK {
				t.Fatalf("estkmeSEsForSKU() ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("len(SEs) = %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, se := range got {
				if se.ID != tt.wantIDs[i] {
					t.Fatalf("SE[%d].ID = %q, want %q", i, se.ID, tt.wantIDs[i])
				}
				if len(se.AID) == 0 {
					t.Fatalf("SE[%d].AID is empty", i)
				}
			}
		})
	}
}

func TestDecodeESTKString(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
		want     string
		wantOK   bool
	}{
		{name: "valid", response: []byte{'M', 'a', 'x', 0x90, 0x00}, want: "Max", wantOK: true},
		{name: "missing status", response: []byte{'M', 'a', 'x'}},
		{name: "wrong status", response: []byte{'M', 'a', 'x', 0x6F, 0x00}},
		{name: "too short", response: []byte{0x90}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeESTKString(tt.response)
			if ok != tt.wantOK {
				t.Fatalf("decodeESTKString() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("decodeESTKString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsESTKmeATR(t *testing.T) {
	tests := []struct {
		name string
		atr  []byte
		want bool
	}{
		{name: "known ESTKme", atr: bytes.Clone(estkmeATRs[0]), want: true},
		{name: "empty"},
		{name: "ordinary", atr: []byte{0x3B, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isESTKmeATR(tt.atr); got != tt.want {
				t.Fatalf("isESTKmeATR() = %v, want %v", got, tt.want)
			}
		})
	}
}
