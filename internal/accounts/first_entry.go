package accounts

import (
	"context"
	"fmt"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/turnstile"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type CaptchaState struct {
	Required bool   `json:"required"`
	SiteKey  string `json:"siteKey"`
	Action   string `json:"action"`
}

type FirstEntryRepository interface {
	RegisterPanelEntry(context.Context, int64, bool) error
	PanelEntryQualified(context.Context, int64) (bool, error)
	VerifyPanelEntry(context.Context, int64) error
}

type FirstEntry struct {
	Repository FirstEntryRepository
	Settings   interface {
		Optional(context.Context, string) (string, error)
	}
	Verifier interface {
		Verify(context.Context, string, string, string) error
	}
	Hostname         string
	AdminTelegramIDs []int64
}

// State initializes only during authenticated panel bootstrap, never for bot messages.
func (f *FirstEntry) State(ctx context.Context, user model.User, entered bool) (CaptchaState, error) {
	state := CaptchaState{Action: turnstile.Action}
	if user.Role == "admin" && slices.Contains(f.AdminTelegramIDs, user.TelegramID) {
		if entered {
			return state, f.Repository.RegisterPanelEntry(ctx, user.TelegramID, false)
		}
		return state, nil
	}
	enabled, err := f.Settings.Optional(ctx, "captcha.turnstile.enabled")
	if err != nil {
		return state, err
	}
	if enabled != "" && enabled != "true" && enabled != "false" {
		return state, turnstile.ErrUnavailable
	}
	if entered {
		if err := f.Repository.RegisterPanelEntry(ctx, user.TelegramID, enabled == "true"); err != nil {
			return state, err
		}
	}
	qualified, err := f.Repository.PanelEntryQualified(ctx, user.TelegramID)
	if err != nil {
		return state, err
	}
	state.Required = enabled == "true" && !qualified
	if state.Required {
		state.SiteKey, err = f.Settings.Optional(ctx, "captcha.turnstile.site_key")
		if err != nil || state.SiteKey == "" || f.Hostname == "" {
			return state, turnstile.ErrUnavailable
		}
	}
	return state, nil
}

func (f *FirstEntry) Verify(ctx context.Context, user model.User, token string) error {
	state, err := f.State(ctx, user, false)
	if err != nil || !state.Required {
		return err
	}
	secret, err := f.Settings.Optional(ctx, "captcha.turnstile.secret_key")
	if err != nil || f.Verifier == nil {
		return turnstile.ErrUnavailable
	}
	if err = f.Verifier.Verify(ctx, secret, token, f.Hostname); err != nil {
		return fmt.Errorf("verify first entry: %w", err)
	}
	return f.Repository.VerifyPanelEntry(ctx, user.TelegramID)
}
