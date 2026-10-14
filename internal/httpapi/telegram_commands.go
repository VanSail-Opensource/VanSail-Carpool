package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/botcommands"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegrampm"
)

func (s *Server) processTelegramGroupMessage(ctx context.Context, message *telegram.Message) {
	s.processTelegramGroupMessageUpdate(ctx, 0, message)
}

func (s *Server) processTelegramGroupMessageUpdate(ctx context.Context, updateID int64, message *telegram.Message) {
	if message == nil || message.From == nil || message.From.IsBot || message.Chat.ID == 0 {
		return
	}
	commandText, addressed := normalizeTelegramCommand(message.Text, s.deps.Affiliates.BotUsername())
	if !addressed {
		return
	}
	command, isCommand := botcommands.Parse(commandText)
	if isCommand && message.Chat.Type == "supergroup" && message.MessageThreadID > 1 {
		enabled, pmGroup, pmErr := telegrampm.Configuration(ctx, s.deps.Settings)
		if pmErr != nil {
			s.deps.Logger.Warn("load PM command destination", "error", pmErr)
			return
		}
		if enabled && message.Chat.Type == "supergroup" && message.Chat.ID == pmGroup {
			if message.MessageThreadID > 1 && isPMAdminCommand(command.Name) {
				s.processTelegramPMAdminCommand(ctx, updateID, message, command)
			}
			return
		}
	}
	groupID, configuredGroup := s.telegramGroupID(ctx)
	if configuredGroup && message.Chat.ID == groupID {
		user, lookupErr := s.deps.Store.UserByTelegramID(ctx, message.From.ID)
		if lookupErr == nil {
			_, matched, joinErr := s.deps.Activity.JoinRaffle(ctx, user.ID, message.Chat.ID, message.MessageID, commandText)
			if matched {
				if joinErr != nil {
					if errors.Is(joinErr, database.ErrInsufficientBalance) {
						s.sendTelegramReply(ctx, message, telegramDrawInsufficient(message.From.LanguageCode))
					} else {
						s.sendTelegramReply(ctx, message, telegramDrawUnavailable(message.From.LanguageCode))
						s.deps.Logger.Warn("raffle entry rejected", "chat_id", message.Chat.ID, "message_id", message.MessageID, "error", joinErr)
					}
				}
				return
			}
		}
	}
	allowedChat := message.Chat.Type == "private" || configuredGroup && message.Chat.ID == groupID
	if isCommand {
		if allowedChat {
			s.processTelegramCommand(ctx, message, command)
		}
		return
	}
	if !configuredGroup || message.Chat.ID != groupID {
		return
	}
	now := time.Now().UTC()
	config, configErr := s.activityConfig(ctx)
	localDate := telegramGroupMessageDate(config.Timezone, configErr, now)
	if err := s.deps.Store.BufferGroupMessageFact(message.Chat.ID, message.MessageID, localDate, now); err != nil {
		s.deps.Logger.Warn("buffer raw Telegram group message", "chat_id", message.Chat.ID, "message_id", message.MessageID, "error", err)
	}
	user, err := s.deps.Store.UserByTelegramID(ctx, message.From.ID)
	if err != nil {
		return
	}
	if configErr != nil {
		s.deps.Logger.Warn("load group-message reward configuration", "telegram_id", message.From.ID, "error", configErr)
		return
	}
	if _, err := s.deps.Activity.RecordGroupMessage(ctx, user.ID, message.Chat.ID, message.MessageID, activity.GroupMessageRewardConfig{
		Timezone: config.Timezone, Threshold: config.GroupMessageThreshold, RewardMinor: config.GroupMessageRewardMinor, BoostCount: message.SenderBoostCount,
	}); err != nil {
		s.deps.Logger.Warn("process Telegram group message reward", "telegram_id", message.From.ID, "message_id", message.MessageID, "error", err)
	}
}
func telegramDrawInsufficient(language string) string {
	if strings.HasPrefix(strings.ToLower(language), "zh") {
		return "余额不足，无法参与本次抽奖。"
	}
	return "Insufficient TXB balance to enter this raffle\\."
}
func telegramDrawUnavailable(language string) string {
	if strings.HasPrefix(strings.ToLower(language), "zh") {
		return "当前无法参与本次抽奖，请稍后再试。"
	}
	return "This raffle entry is unavailable right now\\."
}

func telegramGroupMessageDate(configuredZone string, configErr error, now time.Time) string {
	zoneName := configuredZone
	if configErr != nil || strings.TrimSpace(zoneName) == "" {
		zoneName = defaultActivityTimezone
	}
	location, err := time.LoadLocation(zoneName)
	if err != nil {
		location = time.UTC
	}
	return now.In(location).Format(time.DateOnly)
}

func (s *Server) processTelegramCommand(ctx context.Context, message *telegram.Message, command botcommands.Command) {
	copy := botcommands.Text(botcommands.LanguageFor(message.From.LanguageCode))
	if command.Name == botcommands.Deduct {
		s.processTelegramDeduction(ctx, message, command, copy)
		return
	}
	if !command.Known || len(command.Args) > 0 && command.Name != botcommands.Start {
		s.sendTelegramReply(ctx, message, botcommands.FormatUnknown(copy))
		return
	}
	if command.Name == botcommands.Start {
		if message.Chat.Type == "private" && len(command.Args) == 1 {
			inviterID, err := parsePositiveTelegramID(command.Args[0])
			if err == nil && inviterID > 0 {
				name, accepted, acceptErr := s.deps.Affiliates.AcceptReferral(ctx, message.From.ID, inviterID)
				if acceptErr != nil {
					s.deps.Logger.Warn("accept Telegram referral", "telegram_id", message.From.ID, "error", acceptErr)
				}
				if accepted {
					welcome := botcommands.FormatStart(copy)
					if name != "" {
						welcome = affiliates.FormatReferralWelcome(affiliates.NormalizeLocale(message.From.LanguageCode), name) + "\n\n" + welcome
					}
					s.sendTelegramReply(ctx, message, welcome)
					return
				}
			}
		}
		s.sendTelegramReply(ctx, message, botcommands.FormatStart(copy))
		return
	}
	target := message.From
	if command.AllowsReplyTarget() && message.ReplyToMessage != nil && message.ReplyToMessage.From != nil && !message.ReplyToMessage.From.IsBot {
		target = message.ReplyToMessage.From
	}
	user, err := s.deps.Store.UserByTelegramID(ctx, target.ID)
	if err != nil {
		s.sendTelegramReply(ctx, message, botcommands.FormatUnavailable(copy))
		return
	}
	formatMoney := s.telegramDisplayFormatter(ctx, user)
	var reply string
	switch command.Name {
	case botcommands.Balance:
		money, balanceErr := s.deps.Store.Balance(ctx, user.ID)
		if balanceErr == nil {
			reply = botcommands.FormatBalance(copy, formatMoney(money))
		}
	case botcommands.SignIn:
		config, configErr := s.activityConfig(ctx)
		if configErr == nil {
			result, checkInErr := s.deps.Activity.CheckIn(ctx, user.ID, config)
			if checkInErr == nil {
				reply = botcommands.FormatCheckInWithMoney(copy, result, s.telegramCheckInAverage(ctx), formatMoney(model.TXBMoney(result.RewardMinor)), formatMoney(model.TXBMoney(result.BalanceAfterMinor)))
			} else if errors.Is(checkInErr, activity.ErrGroupBoostRequired) {
				reply = botcommands.FormatGroupBoostRequired(copy)
			} else if errors.Is(checkInErr, activity.ErrGroupBoostUnavailable) {
				reply = botcommands.FormatGroupBoostUnavailable(copy)
			}
		}
	case botcommands.Sub:
		reply = s.telegramSubscriptionReply(ctx, user, copy)
	case botcommands.MyCombo:
		reply = s.telegramComboReply(ctx, user, copy, formatMoney)
	}
	if strings.TrimSpace(reply) == "" {
		reply = botcommands.FormatUnavailable(copy)
	}
	s.sendTelegramReply(ctx, message, reply)
}

func (s *Server) sendTelegramReply(ctx context.Context, message *telegram.Message, reply string) {
	if err := s.deps.Telegram.SendMarkdownV2Message(ctx, message.Chat.ID, message.MessageID, botcommands.Limit(reply)); err != nil {
		s.deps.Logger.Warn("send Telegram command reply", "chat_id", message.Chat.ID, "message_id", message.MessageID, "error", err)
	}
}

func (s *Server) telegramGroupID(ctx context.Context) (int64, bool) {
	value, err := s.deps.Settings.Optional(ctx, "telegram.group_chat_id")
	if err != nil {
		s.deps.Logger.Warn("load Telegram group chat for message processing", "error", err)
		return 0, false
	}
	groupID, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return groupID, err == nil && groupID != 0
}
