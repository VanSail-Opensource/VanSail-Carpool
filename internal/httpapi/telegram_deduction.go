package httpapi

import (
	"context"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/botcommands"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Server) processTelegramDeduction(ctx context.Context, message *telegram.Message, command botcommands.Command, copy botcommands.Copy) {
	if len(command.Args) != 1 {
		s.sendTelegramReply(ctx, message, botcommands.FormatDeductUsage(copy))
		return
	}
	amount, err := billing.ParseTXBMajor(command.Args[0])
	if err != nil || amount <= 0 {
		s.sendTelegramReply(ctx, message, botcommands.FormatDeductUsage(copy))
		return
	}
	if message.MessageID <= 0 || message.Chat.ID == 0 || message.From == nil || !s.isAdminTelegramID(message.From.ID) || message.ReplyToMessage == nil || message.ReplyToMessage.From == nil || message.ReplyToMessage.From.IsBot {
		s.sendTelegramReply(ctx, message, botcommands.FormatDeductRejected(copy))
		return
	}
	actor, err := s.deps.Store.UserByTelegramID(ctx, message.From.ID)
	if err != nil || actor.Role != "admin" {
		s.deps.Logger.Warn("load Telegram administrator for deduction", "error", err)
		s.sendTelegramReply(ctx, message, botcommands.FormatDeductRejected(copy))
		return
	}
	target, err := s.deps.Store.UserByTelegramID(ctx, message.ReplyToMessage.From.ID)
	if err != nil {
		s.sendTelegramReply(ctx, message, botcommands.FormatDeductRejected(copy))
		return
	}
	reason := fmt.Sprintf("Telegram /deduct in chat %d on message %d", message.Chat.ID, message.ReplyToMessage.MessageID)
	// Telegram can redeliver the same update after the ledger commits but the
	// webhook response is lost. A message ID is stable within its chat.
	reference := fmt.Sprintf("telegram-deduct:%d:%d", message.Chat.ID, message.MessageID)
	if _, err := s.deps.Admin.DeductBalanceWithReference(ctx, actor.ID, target.ID, amount, reference, reason); err != nil {
		s.deps.Logger.Warn("Telegram TXB deduction rejected", "actor_id", actor.ID, "target_id", target.ID, "amount_minor", amount, "error", err)
		s.sendTelegramReply(ctx, message, botcommands.FormatDeductRejected(copy))
		return
	}
	s.sendTelegramReply(ctx, message, botcommands.FormatDeductSucceeded(copy, s.telegramDisplayFormatter(ctx, target)(model.TXBMoney(amount))))
}

func telegramDeductCommand(text string) (string, bool) {
	command, ok := botcommands.Parse(text)
	if !ok || command.Name != botcommands.Deduct || len(command.Args) != 1 {
		return "", false
	}
	amount, err := billing.ParseTXBMajor(command.Args[0])
	return command.Args[0], err == nil && amount > 0
}
