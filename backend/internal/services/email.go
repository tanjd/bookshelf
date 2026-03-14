// Package services contains the business logic services for the bookshelf app.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// EmailService sends transactional emails via the Resend API.
type EmailService struct {
	apiKey string
	from   string
}

// NewEmailService creates a new EmailService.
func NewEmailService(apiKey, from string) *EmailService {
	return &EmailService{apiKey: apiKey, from: from}
}

type resendPayload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// SendEmail posts an email via Resend. If no API key is configured the call is
// silently skipped.
func (s *EmailService) SendEmail(to, subject, html string) error {
	if s.apiKey == "" {
		log.Printf("[email] skipping send (no RESEND_API_KEY): to=%s subject=%q", to, subject)
		return nil
	}

	payload := resendPayload{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		HTML:    html,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("email: send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("email: close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("email: resend returned status %d", resp.StatusCode)
	}

	return nil
}
