package httpapi

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

func (s *Server) processTelegramPM(ctx context.Context, update telegram.Update) (bool, error) {
	if s.deps.PM == nil {
		return false, nil
	}
	if update.CallbackQuery != nil {
		return s.deps.PM.HandleCallback(ctx, update.UpdateID, update.CallbackQuery)
	}
	if update.Message != nil {
		return s.deps.PM.HandleMessage(ctx, update.UpdateID, update.Message)
	}
	return false, nil
}
