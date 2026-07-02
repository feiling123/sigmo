package main

import "testing"

func TestBuildVersion(t *testing.T) {
	old := BuildVersion
	t.Cleanup(func() {
		BuildVersion = old
	})

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "build version", version: "v1.2.3", want: "v1.2.3"},
		{name: "empty version", version: "", want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			BuildVersion = tt.version

			if got := buildVersion(); got != tt.want {
				t.Fatalf("buildVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
