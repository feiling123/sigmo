package websheet

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrokerCreateRejectsUnsafeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "localhost", raw: "http://localhost/websheet"},
		{name: "loopback ip", raw: "http://127.0.0.1/websheet"},
		{name: "private ip", raw: "http://192.168.1.1/websheet"},
		{name: "non http scheme", raw: "file:///tmp/websheet.html"},
	}

	broker := New(Config{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := broker.Create(context.Background(), Request{URL: tt.raw}); err == nil {
				t.Fatal("Create() error is nil")
			}
		})
	}
}

func TestSessionCallbackAndDone(t *testing.T) {
	t.Parallel()

	broker := New(Config{AllowPrivateHosts: true})
	session, err := broker.Create(context.Background(), Request{URL: "https://example.com/websheet"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session.Callback(Callback{Event: "finishFlow", NextAction: "AcquireConfiguration"})
	callback, err := session.WaitCallback(context.Background())
	if err != nil {
		t.Fatalf("WaitCallback() error = %v", err)
	}
	if callback.Event != "finishFlow" || callback.NextAction != "AcquireConfiguration" {
		t.Fatalf("WaitCallback() = %+v", callback)
	}

	session.Done()
	if err := session.WaitDone(context.Background()); err != nil {
		t.Fatalf("WaitDone() error = %v", err)
	}
}

func TestBrokerExpiresSessions(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	broker := New(Config{
		TTL:               time.Minute,
		AllowPrivateHosts: true,
		Now: func() time.Time {
			return now
		},
	})
	session, err := broker.Create(context.Background(), Request{URL: "https://example.com/websheet"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := broker.Get(session.Info().ID); err != nil {
		t.Fatalf("Get() before expiry error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := broker.Get(session.Info().ID); err == nil {
		t.Fatal("Get() after expiry error is nil")
	}
}

func TestSessionMethodUsesContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  Request
		want string
	}{
		{
			name: "user data without content type is get",
			req:  Request{URL: "https://example.com/websheet", UserData: "token=abc"},
			want: http.MethodGet,
		},
		{
			name: "content type is post",
			req:  Request{URL: "https://example.com/websheet", UserData: "token=abc", ContentType: "application/x-www-form-urlencoded"},
			want: http.MethodPost,
		},
	}

	broker := New(Config{AllowPrivateHosts: true})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := broker.Create(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if got := session.Info().Method; got != tt.want {
				t.Fatalf("Info().Method = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProxyRewritesHTMLAndStripsFrameHeaders(t *testing.T) {
	t.Parallel()

	carrier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		w.Header().Set("Set-Cookie", "carrier_session=abc; Path=/")
		_, _ = w.Write([]byte(`<html><head></head><body><a href="/next">Next</a></body></html>`))
	}))
	defer carrier.Close()

	broker := New(Config{AllowPrivateHosts: true})
	session, err := broker.Create(context.Background(), Request{URL: carrier.URL})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/proxy?target="+carrier.URL+"&token=abc", nil)
	rec := httptest.NewRecorder()
	if err := session.Proxy(rec, req); err != nil {
		t.Fatalf("Proxy() error = %v", err)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want empty", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("Content-Security-Policy = %q, want empty", got)
	}
	if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("Set-Cookie = %q, want empty", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/api/v1/websheets/"+session.Info().ID+"/proxy") {
		t.Fatalf("proxied body missing rewritten link: %s", body)
	}
	if !strings.Contains(body, "ODSAServiceFlow") {
		t.Fatalf("proxied body missing bridge script: %s", body)
	}
	if !strings.Contains(body, "ts43ODSAServiceFlow") || !strings.Contains(body, "ts43-odsa-callback") {
		t.Fatalf("proxied body missing TS.43 ODSA callback adapter: %s", body)
	}
	for _, want := range []string{"VoWiFiWebServiceFlow", "WiFiCallingWebViewController", "NsdsWebSheetController"} {
		if !strings.Contains(body, want) {
			t.Fatalf("proxied body missing VoWiFi bridge %q: %s", want, body)
		}
	}
	if strings.Contains(body, callbackURLToken) {
		t.Fatalf("proxied body contains unresolved bridge token: %s", body)
	}
	if !strings.Contains(body, "mode: \"no-cors\"") {
		t.Fatalf("proxied body missing no-cors callback fetch: %s", body)
	}
}

func TestCallbackEndpointPayloadShape(t *testing.T) {
	t.Parallel()

	callback := Callback{
		Source:     "vowifi",
		Controller: "VoWiFiWebServiceFlow",
		Method:     "entitlementChanged",
		Event:      "entitlementChanged",
		ResultCode: "success",
		Href:       "https://example.com/wfc",
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(callback); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	var got Callback
	if err := json.NewDecoder(&body).Decode(&got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got != callback {
		t.Fatalf("Callback = %+v, want %+v", got, callback)
	}
}

func TestTS43CallbackEndpointPayloadShape(t *testing.T) {
	t.Parallel()

	callback := Callback{
		Event:          "profileReadyWithActivationCode",
		ActivationCode: "1$example.com$abc",
		ICCID:          "8901",
		IMEI:           "123456789012345",
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(callback); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	var got Callback
	if err := json.NewDecoder(&body).Decode(&got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got != callback {
		t.Fatalf("Callback = %+v, want %+v", got, callback)
	}
}
