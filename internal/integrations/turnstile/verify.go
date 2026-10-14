// Package turnstile validates first-entry challenges through the shared upstream queue.
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

const Action = "txc_first_entry"

var ErrRejected = errors.New("CAPTCHA token rejected or expired")
var ErrUnavailable = errors.New("CAPTCHA verification unavailable")

// Verifier holds no user token or secret between calls. Its transport is injectable.
type Verifier struct {
	Queue  *upstreamqueue.Queue
	Client *http.Client
}

func New(queue *upstreamqueue.Queue) *Verifier {
	return &Verifier{Queue: queue, Client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (v *Verifier) Verify(ctx context.Context, secret, token, hostname string) error {
	if len(token) == 0 || len(token) > 2048 {
		return ErrRejected
	}
	if secret == "" || hostname == "" {
		return ErrUnavailable
	}
	key, err := ids.New()
	if err != nil {
		return ErrUnavailable
	}
	return upstreamqueue.Execute(ctx, v.Queue, func(callCtx context.Context) error {
		form := url.Values{"secret": {secret}, "response": {token}, "idempotency_key": {key}}
		req, err := http.NewRequestWithContext(callCtx, http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", strings.NewReader(form.Encode()))
		if err != nil {
			return ErrUnavailable
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := v.Client.Do(req)
		if err != nil {
			return ErrUnavailable
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return ErrUnavailable
		}
		var result struct {
			Success  bool     `json:"success"`
			Hostname string   `json:"hostname"`
			Action   string   `json:"action"`
			Codes    []string `json:"error-codes"`
		}
		if json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil {
			return ErrUnavailable
		}
		for _, code := range result.Codes {
			if code == "internal-error" || code == "invalid-input-secret" || code == "missing-input-secret" {
				return ErrUnavailable
			}
		}
		if !result.Success || result.Action != Action || !strings.EqualFold(result.Hostname, hostname) {
			return ErrRejected
		}
		return nil
	})
}
