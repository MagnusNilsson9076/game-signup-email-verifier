package signup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestInfraiEmailRequestAndRetry(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if req.Method != http.MethodPost || req.URL.String() != infraiEmailSend {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("Idempotency-Key") != "signup:player-7" {
			t.Fatalf("unexpected headers: %v", req.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"to", "subject", "html"} {
			if _, ok := body[key]; !ok {
				t.Fatalf("missing body field %q", key)
			}
		}
		if attempts == 1 {
			return response(http.StatusTooManyRequests, `{"ok":false,"data":{},"error":{"message":"retry later"}}`, "0"), nil
		}
		return response(http.StatusOK, `{"ok":true,"data":{"message_id":"msg_42"},"error":null}`, ""), nil
	})}
	mailer := InfraiEmail{APIKey: "test-key", Client: client, Sleep: func(context.Context, time.Duration) error { return nil }}
	messageID, err := mailer.SendVerification(context.Background(), "kai@example.com", "https://game.example/verify-email?token=abc", "signup:player-7")
	if err != nil {
		t.Fatal(err)
	}
	if messageID != "msg_42" || attempts != 2 {
		t.Fatalf("message=%q attempts=%d", messageID, attempts)
	}
}

func response(status int, body, retryAfter string) *http.Response {
	header := make(http.Header)
	if retryAfter != "" {
		header.Set("Retry-After", retryAfter)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}
