package httpapi

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/botcommands"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func isPMAdminCommand(name botcommands.Name) bool {
	return name == botcommands.Refund || name == botcommands.AddTXB || name == botcommands.DeductTXB
}

func pmRefundAmount(quote admin.EntitlementRefundQuote) (int64, bool) {
	if quote.SuggestedRefund == nil {
		return 0, false
	}
	amount, err := strconv.ParseInt(quote.SuggestedRefund.Minor, 10, 64)
	return amount, err == nil && amount > 0
}

func (s *Server) telegramPMCommandTarget(ctx context.Context, message *telegram.Message) (model.PMConversation, model.User, bool) {
	if message == nil || message.From == nil || message.Chat.Type != "supergroup" || message.MessageThreadID <= 1 || !s.isAdminTelegramID(message.From.ID) {
		return model.PMConversation{}, model.User{}, false
	}
	conversation, found, err := s.deps.Store.PMConversationByTopic(ctx, message.Chat.ID, message.MessageThreadID)
	if err != nil || !found {
		return model.PMConversation{}, model.User{}, false
	}
	actor, err := s.deps.Store.UserByTelegramID(ctx, message.From.ID)
	if err != nil || actor.Role != "admin" {
		return model.PMConversation{}, model.User{}, false
	}
	return conversation, actor, true
}

func (s *Server) processTelegramPMAdminCommand(ctx context.Context, updateID int64, message *telegram.Message, command botcommands.Command) {
	if message == nil || message.From == nil || message.MessageThreadID <= 1 || !s.isAdminTelegramID(message.From.ID) {
		return
	}
	copy := botcommands.PMAdminTexts(botcommands.LanguageFor(message.From.LanguageCode))
	if updateID <= 0 {
		s.sendTelegramReply(ctx, message, botcommands.FormatPMAdminRejected(copy))
		return
	}
	if command.Name == botcommands.Refund && len(command.Args) != 0 || command.Name != botcommands.Refund && len(command.Args) != 1 {
		s.sendTelegramReply(ctx, message, botcommands.FormatPMAdminUsage(copy))
		return
	}
	conversation, actor, found := s.telegramPMCommandTarget(ctx, message)
	if !found {
		s.sendTelegramReply(ctx, message, botcommands.FormatPMAdminRejected(copy))
		return
	}
	reference := fmt.Sprintf("telegram-pm:%s:%d", command.Name, updateID)
	reason := fmt.Sprintf("Telegram PM /%s in topic %d update %d", command.Name, message.MessageThreadID, updateID)
	switch command.Name {
	case botcommands.AddTXB, botcommands.DeductTXB:
		s.adjustPMBalance(ctx, message, conversation.UserID, actor.ID, command, reference, reason, copy)
	case botcommands.Refund:
		s.refundPMCombo(ctx, message, conversation.UserID, actor.ID, updateID, reason, copy)
	}
}

func (s *Server) adjustPMBalance(ctx context.Context, message *telegram.Message, userID, actorID string, command botcommands.Command, reference, reason string, copy botcommands.PMAdminCopy) {
	amount, err := billing.ParseTXBMajor(command.Args[0])
	if err != nil || amount <= 0 {
		s.sendTelegramReply(ctx, message, botcommands.FormatPMAdminUsage(copy))
		return
	}
	added := command.Name == botcommands.AddTXB
	var entry model.LedgerEntry
	if added {
		entry, err = s.deps.Admin.AdjustBalanceWithReference(ctx, actorID, userID, amount, reference, reason)
	} else {
		entry, err = s.deps.Admin.DeductBalanceWithReference(ctx, actorID, userID, amount, reference, reason)
	}
	if err != nil {
		s.deps.Logger.Warn("Telegram PM balance command rejected", "actor_id", actorID, "target_id", userID, "command", command.Name, "error", err)
		s.sendTelegramReply(ctx, message, botcommands.FormatPMAdminRejected(copy))
		return
	}
	credited := amount
	if entry.DeltaTXBMinor < 0 {
		credited = -entry.DeltaTXBMinor
	}
	s.queuePMProfileRefresh(ctx, message, userID, actorID)
	s.sendTelegramReply(ctx, message, botcommands.FormatPMAdminBalance(copy, added, model.TXBMoney(credited), entry.BalanceAfter))
}

func (s *Server) refundPMCombo(ctx context.Context, message *telegram.Message, userID, actorID string, updateID int64, reason string, copy botcommands.PMAdminCopy) {
	detail, err := s.deps.AdminUsers.UserDetail(ctx, userID)
	if err != nil || detail.ActiveCombo == nil {
		s.sendTelegramReply(ctx, message, botcommands.FormatPMRefundUnavailable(copy))
		return
	}
	purchase := *detail.ActiveCombo
	quote, err := s.deps.AdminUsers.RefundEntitlementQuote(ctx, userID, purchase.ID)
	if err != nil {
		s.sendTelegramReply(ctx, message, botcommands.FormatPMRefundUnavailable(copy))
		return
	}
	amount, ok := pmRefundAmount(quote)
	if !ok {
		s.sendTelegramReply(ctx, message, botcommands.FormatPMRefundUnavailable(copy))
		return
	}
	key := fmt.Sprintf("telegram-pm-refund:%d", updateID)
	receipt, err := s.deps.AdminUsers.RefundEntitlement(ctx, actorID, userID, purchase.ID, key, reason, amount)
	if err != nil {
		s.deps.Logger.Warn("Telegram PM combo refund rejected", "actor_id", actorID, "target_id", userID, "purchase_id", purchase.ID, "error", err)
		s.sendTelegramReply(ctx, message, botcommands.FormatPMAdminRejected(copy))
		return
	}
	s.queuePMProfileRefresh(ctx, message, userID, actorID)
	s.sendTelegramReply(ctx, message, botcommands.FormatPMRefundQueued(copy, model.TXBMoney(amount), receipt.ID))
}

func (s *Server) queuePMProfileRefresh(ctx context.Context, message *telegram.Message, userID, actorID string) {
	conversation, found, err := s.deps.Store.PMConversationByTopic(ctx, message.Chat.ID, message.MessageThreadID)
	if err != nil || !found || conversation.UserID != userID {
		s.deps.Logger.Warn("resolve PM profile refresh target", "target_id", userID, "error", err)
		return
	}
	key := fmt.Sprintf("pm-command-refresh:%d", message.MessageID)
	if _, err := s.deps.Store.QueuePMProfileRefresh(ctx, actorID, key, conversation.ID, 0, time.Now().UTC()); err != nil {
		s.deps.Logger.Warn("queue PM profile refresh", "target_id", userID, "error", err)
	}
}
