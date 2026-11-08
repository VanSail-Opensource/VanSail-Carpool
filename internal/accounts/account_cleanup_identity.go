package accounts

import (
	"context"
	"errors"
	"time"
)

// VerifyBot pauses durable admission before checking the configured token.
// Token-prefix comparison supplements authenticated getMe; it never replaces it.
func (s *AccountCleanupService) VerifyBot(ctx context.Context) error {
	if !s.verifyBusy.CompareAndSwap(false, true) {
		return nil
	}
	defer s.verifyBusy.Store(false)
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := s.repository.SetAccountCleanupBot(bounded, 0, s.now().UTC()); err != nil {
		return err
	}
	bot, err := s.telegram.GetMe(bounded)
	if err != nil {
		return errors.New("account cleanup bot verification unavailable")
	}
	if !bot.IsBot || bot.ID != s.expectedBotID {
		return errors.New("account cleanup bot identity mismatch")
	}
	_, err = s.repository.SetAccountCleanupBot(bounded, bot.ID, s.now().UTC())
	return err
}
