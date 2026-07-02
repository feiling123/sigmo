package appinfo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestHandlerGet(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "build version", version: "v1.2.3", want: `{"version":"v1.2.3"}` + "\n"},
		{name: "empty version", version: "", want: `{"version":"dev"}` + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/app", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := New(tt.version).Get(c); err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("Get() status = %d, want %d", rec.Code, http.StatusOK)
			}
			if rec.Body.String() != tt.want {
				t.Fatalf("Get() body = %q, want %q", rec.Body.String(), tt.want)
			}
		})
	}
}
