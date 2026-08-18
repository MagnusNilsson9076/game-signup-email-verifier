package signup

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

type Asset struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	SubmittedBy string `json:"submitted_by"`
}

type LiveEvent struct {
	ID        string `json:"id"`
	StartsAt  string `json:"starts_at"`
	CreatedBy string `json:"created_by"`
}

type SignupRequest struct {
	PlayerID   string      `json:"player_id"`
	Email      string      `json:"email"`
	Assets     []Asset     `json:"assets"`
	LiveEvents []LiveEvent `json:"live_events"`
}

type SignupResult struct {
	PlayerID        string      `json:"player_id"`
	AccountState    string      `json:"account_state"`
	Assets          []Asset     `json:"assets"`
	LiveEvents      []LiveEvent `json:"live_events"`
	ModerationQueue []string    `json:"moderation_queue"`
	EmailMessageID  string      `json:"email_message_id"`
}

type TokenSource func() (string, error)

type Flow struct {
	Email     EmailSender
	PublicURL string
	NewToken  TokenSource
}

func (f Flow) Register(ctx context.Context, input SignupRequest) (SignupResult, error) {
	if strings.TrimSpace(input.PlayerID) == "" {
		return SignupResult{}, errors.New("player_id is required")
	}
	if _, err := mail.ParseAddress(input.Email); err != nil {
		return SignupResult{}, errors.New("email must be valid")
	}
	for _, asset := range input.Assets {
		if asset.ID == "" || asset.SubmittedBy != input.PlayerID {
			return SignupResult{}, errors.New("each asset must belong to the player")
		}
	}
	for _, event := range input.LiveEvents {
		if event.ID == "" || event.CreatedBy != input.PlayerID {
			return SignupResult{}, errors.New("each live event must belong to the player")
		}
	}

	newToken := f.NewToken
	if newToken == nil {
		newToken = randomToken
	}
	token, err := newToken()
	if err != nil {
		return SignupResult{}, fmt.Errorf("create verification token: %w", err)
	}
	verifyURL := strings.TrimRight(f.PublicURL, "/") + "/verify-email?token=" + url.QueryEscape(token)
	messageID, err := f.Email.SendVerification(ctx, input.Email, verifyURL, "signup:"+input.PlayerID)
	if err != nil {
		return SignupResult{}, err
	}

	queue := make([]string, 0, len(input.Assets))
	for _, asset := range input.Assets {
		queue = append(queue, asset.ID)
	}
	return SignupResult{
		PlayerID: input.PlayerID, AccountState: "pending_email_verification",
		Assets: input.Assets, LiveEvents: input.LiveEvents,
		ModerationQueue: queue, EmailMessageID: messageID,
	}, nil
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
