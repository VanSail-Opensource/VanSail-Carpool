package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func cleanupWait(reason string, err error) error {
	return &outbox.WaitError{Reason: reason, Cause: err, After: time.Minute}
}

func (s *AccountCleanupService) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	var input struct {
		UserID     string `json:"userId"`
		Generation int64  `json:"generation"`
		Nonce      string `json:"nonce"`
	}
	decoder := json.NewDecoder(strings.NewReader(job.Payload))
	decoder.DisallowUnknownFields()
	if job.Kind != AccountCleanupJob || len(job.Payload) > 1024 || decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || input.UserID == "" || input.Generation < 0 || input.Nonce == "" {
		return &outbox.TerminalError{Reason: "account_cleanup_probe_invalid", Cause: errors.New("invalid account cleanup probe")}
	}
	if !s.probeBusy.CompareAndSwap(false, true) {
		return cleanupWait("account_cleanup_probe_busy", nil)
	}
	defer s.probeBusy.Store(false)
	bounded, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	probe, err := s.repository.AccountCleanupProbe(bounded, input.UserID, input.Generation, input.Nonce)
	if err != nil {
		return cleanupWait("account_cleanup_probe_state_pending", err)
	}
	if probe == nil {
		return nil
	}
	if probe.UserID != input.UserID || probe.AccountGeneration != input.Generation || probe.Nonce != input.Nonce || probe.BotID != s.expectedBotID || probe.TelegramID <= 0 || probe.TelegramID > 1<<52-1 || s.protected[probe.TelegramID] {
		return nil
	}
	actionCtx, stopAction := context.WithTimeout(bounded, 2*time.Second)
	action, actionErr := s.telegram.SendChatAction(actionCtx, probe.TelegramID)
	stopAction()
	chatCtx, stopChat := context.WithTimeout(bounded, 2*time.Second)
	chat, chatErr := s.telegram.GetChat(chatCtx, probe.TelegramID)
	stopChat()
	observation := model.AccountCleanupObservation{SendAction: telegram.ClassifyChatAction(action, actionErr), GetChat: telegram.ClassifyPrivateChat(probe.TelegramID, chat, chatErr)}
	if telegram.BotIdentityUncertain(actionErr) || telegram.BotIdentityUncertain(chatErr) {
		if _, err = s.repository.SetAccountCleanupBot(bounded, 0, s.now().UTC()); err != nil {
			return cleanupWait("account_cleanup_identity_pending", err)
		}
	}
	_, err = s.repository.CompleteAccountCleanupProbe(bounded, *probe, observation, s.now().UTC())
	if err != nil {
		return cleanupWait("account_cleanup_observation_pending", err)
	}
	return nil
}
