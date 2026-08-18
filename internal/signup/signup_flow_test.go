package signup

import (
	"context"
	"testing"
)

type sentEmail struct {
	to, verifyURL, key string
}

type recordingSender struct {
	sent []sentEmail
}

func (s *recordingSender) SendVerification(_ context.Context, to, verifyURL, key string) (string, error) {
	s.sent = append(s.sent, sentEmail{to: to, verifyURL: verifyURL, key: key})
	return "msg_42", nil
}

func TestRegisterDecision(t *testing.T) {
	tests := []struct {
		name      string
		input     SignupRequest
		wantError bool
		wantQueue []string
	}{
		{
			name: "queues player assets and schedules verification",
			input: SignupRequest{
				PlayerID: "player-7", Email: "kai@example.com",
				Assets:     []Asset{{ID: "skin-9", Kind: "skin", SubmittedBy: "player-7"}},
				LiveEvents: []LiveEvent{{ID: "raid-2", StartsAt: "2026-08-20T19:00:00Z", CreatedBy: "player-7"}},
			},
			wantQueue: []string{"skin-9"},
		},
		{
			name: "rejects an asset owned by another player",
			input: SignupRequest{
				PlayerID: "player-7", Email: "kai@example.com",
				Assets: []Asset{{ID: "skin-9", Kind: "skin", SubmittedBy: "player-8"}},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &recordingSender{}
			flow := Flow{Email: sender, PublicURL: "http://localhost:8080", NewToken: func() (string, error) { return "token-abc", nil }}
			result, err := flow.Register(context.Background(), tt.input)
			if tt.wantError {
				if err == nil || len(sender.sent) != 0 {
					t.Fatalf("expected rejection before email, err=%v sends=%d", err, len(sender.sent))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.AccountState != "pending_email_verification" || len(result.LiveEvents) != 1 {
				t.Fatalf("unexpected signup state: %+v", result)
			}
			if len(result.ModerationQueue) != len(tt.wantQueue) || result.ModerationQueue[0] != tt.wantQueue[0] {
				t.Fatalf("moderation queue = %v, want %v", result.ModerationQueue, tt.wantQueue)
			}
			if len(sender.sent) != 1 || sender.sent[0].key != "signup:player-7" || sender.sent[0].verifyURL != "http://localhost:8080/verify-email?token=token-abc" {
				t.Fatalf("unexpected email call: %+v", sender.sent)
			}
		})
	}
}
