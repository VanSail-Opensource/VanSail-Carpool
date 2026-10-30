package telegram

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
)

// WebhookSecretHeader is the header Telegram uses for configured webhook secrets.
const WebhookSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

// WebhookConfig describes Telegram webhook setup.
type WebhookConfig struct {
	URL                string   `json:"url"`
	SecretToken        string   `json:"secret_token,omitempty"`
	AllowedUpdates     []string `json:"allowed_updates,omitempty"`
	DropPendingUpdates bool     `json:"drop_pending_updates,omitempty"`
	MaxConnections     int      `json:"max_connections,omitempty"`
}

// DefaultAllowedUpdates returns the update kinds required by onboarding and Stars payments.
func DefaultAllowedUpdates() []string {
	return []string{"message", "chat_member", "my_chat_member", "chat_join_request", "pre_checkout_query", "chat_boost", "callback_query"}
}

// VerifyWebhookSecret compares a received webhook header with the configured secret in constant time.
func VerifyWebhookSecret(provided, expected string) bool {
	if provided == "" || expected == "" || len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// SetWebhook configures Telegram webhook delivery.
func (c *Client) SetWebhook(ctx context.Context, config WebhookConfig) error {
	if err := validateHTTPSURL(config.URL); err != nil {
		return fmt.Errorf("telegram webhook URL: %w", err)
	}
	if config.SecretToken != "" && !validWebhookSecret(config.SecretToken) {
		return errors.New("telegram webhook secret must contain 1-256 ASCII letters, digits, underscores, or hyphens")
	}
	if config.MaxConnections < 0 || config.MaxConnections > 100 {
		return errors.New("telegram webhook max connections must be in 1..100 when set")
	}
	var result bool
	if err := c.call(ctx, "setWebhook", config, &result); err != nil {
		return err
	}
	if !result {
		return errors.New("telegram setWebhook returned false")
	}
	return nil
}

// SetChatMenuButton configures the default private-chat button to open the Mini App.
func (c *Client) SetChatMenuButton(ctx context.Context, text, webAppURL string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("telegram menu button text is empty")
	}
	if err := validateHTTPSURL(webAppURL); err != nil {
		return fmt.Errorf("telegram menu Web App URL: %w", err)
	}
	payload := struct {
		MenuButton struct {
			Type   string `json:"type"`
			Text   string `json:"text"`
			WebApp struct {
				URL string `json:"url"`
			} `json:"web_app"`
		} `json:"menu_button"`
	}{}
	payload.MenuButton.Type = "web_app"
	payload.MenuButton.Text = text
	payload.MenuButton.WebApp.URL = webAppURL
	var result bool
	if err := c.call(ctx, "setChatMenuButton", payload, &result); err != nil {
		return err
	}
	if !result {
		return errors.New("telegram setChatMenuButton returned false")
	}
	return nil
}
