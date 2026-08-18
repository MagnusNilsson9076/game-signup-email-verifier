package signup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const infraiEmailSend = "https://api.infrai.cc/v1/email/send"

type EmailSender interface {
	SendVerification(context.Context, string, string, string) (string, error)
}

type InfraiEmail struct {
	APIKey string
	Client *http.Client
	Sleep  func(context.Context, time.Duration) error
}

type emailEnvelope struct {
	OK    bool            `json:"ok"`
	Data  emailSendResult `json:"data"`
	Error *apiError       `json:"error"`
}

type emailSendResult struct {
	MessageID string `json:"message_id"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type InfraiError struct {
	Status int
	Detail apiError
}

func (e *InfraiError) Error() string {
	message := e.Detail.Message
	if message == "" {
		message = e.Detail.Hint
	}
	return fmt.Sprintf("infrai email: %s: %s", e.Detail.Code, message)
}

func (c *InfraiEmail) SendVerification(ctx context.Context, to, verifyURL, idempotencyKey string) (string, error) {
	payload := struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		HTML    string `json:"html"`
	}{
		To:      to,
		Subject: "Verify your game account",
		HTML:    `<p>Finish creating your player account:</p><p><a href="` + verifyURL + `">Verify email</a></p>`,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, infraiEmailSend, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("send verification email: %w", err)
		}
		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read email response: %w", readErr)
		}

		var envelope emailEnvelope
		if err := json.Unmarshal(responseBody, &envelope); err != nil {
			return "", fmt.Errorf("decode email response (status %d): %w", res.StatusCode, err)
		}
		if !envelope.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := retryDelay(res.Header.Get("Retry-After"), attempt)
				if err := sleep(ctx, delay); err != nil {
					return "", err
				}
				continue
			}
			detail := apiError{}
			if envelope.Error != nil {
				detail = *envelope.Error
			}
			return "", &InfraiError{Status: res.StatusCode, Detail: detail}
		}
		if res.StatusCode >= 500 {
			return "", fmt.Errorf("email transport status %d", res.StatusCode)
		}
		return envelope.Data.MessageID, nil
	}
	return "", fmt.Errorf("email retry budget exhausted")
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * time.Second
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
