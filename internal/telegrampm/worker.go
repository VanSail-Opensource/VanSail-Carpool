package telegrampm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type Worker struct {
	Repository    WorkerRepository
	Settings      Settings
	Sender        Sender
	Profiles      ProfileFactsReader
	PublicBaseURL string
	Timezone      string
	AdminIDs      map[int64]bool
}
type execution struct {
	operation      providerops.Operation
	job            model.OutboxJob
	conversationID string
	inbound        bool
}
type phaseResult struct {
	status          providerops.Status
	reference, code string
	deferred        bool
}

func (w *Worker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	items, err := w.Repository.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil {
		return err
	}
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		operation, err = w.Repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, time.Now().UTC())
		if err != nil {
			return err
		}
	}
	if operation.Receipt.Status != string(providerops.StatusProcessing) {
		return nil
	}
	run := execution{operation: operation, job: job}
	byKey := make(map[string]providerops.Item, len(items))
	for _, item := range items {
		byKey[item.Key] = item
		if item.TargetType == "pm_conversation" {
			run.conversationID = item.TargetID
		}
		if item.Key == "relay" {
			run.inbound = item.TargetType == "pm_inbound_message"
		}
		if item.Key == "repair" {
			run.conversationID, _, _ = strings.Cut(item.TargetID, ":")
		}
	}
	if run.conversationID == "" {
		return fmt.Errorf("PM operation has no conversation reference")
	}
	for _, key := range []string{"topic", "profile", "relay", "footer", "repair"} {
		item, exists := byKey[key]
		if !exists || item.Status == providerops.StatusSucceeded {
			continue
		}
		if providerops.Terminal(item.Status) {
			return w.complete(ctx, run, phaseResult{status: item.Status, code: item.ErrorCode})
		}
		interrupted := item.Status == providerops.StatusProcessing
		if !interrupted {
			item, err = w.Repository.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, key, time.Now().UTC())
			if err != nil {
				return err
			}
		}
		var result phaseResult
		switch key {
		case "topic":
			result, err = w.topic(ctx, run, item, interrupted)
		case "profile":
			result, err = w.profile(ctx, run, item, interrupted)
		case "relay":
			result, err = w.relay(ctx, run, item, interrupted)
		case "footer":
			result, err = w.footer(ctx, run, item, interrupted)
		case "repair":
			result, err = w.repair(ctx, run, item, interrupted)
		}
		if err != nil {
			return err
		}
		if !result.deferred {
			if result.status == providerops.StatusPendingReview && result.code == "PM_PROFILE_UNCERTAIN" {
				persist, cancel := completionContext(ctx)
				err = w.Repository.MarkPMProfileUncertain(persist, run.conversationID)
				cancel()
				if err != nil {
					return err
				}
			}
			persist, cancel := completionContext(ctx)
			_, err = w.Repository.CompleteProviderOperationItem(persist, operation.Receipt.ID, key, providerops.Completion{Status: result.status, ProviderReference: result.reference, ErrorCode: result.code}, time.Now().UTC())
			cancel()
			if err != nil {
				return err
			}
		}
		if result.status != providerops.StatusSucceeded {
			return w.complete(ctx, run, result)
		}
	}
	return w.complete(ctx, run, phaseResult{status: providerops.StatusSucceeded})
}

func (w *Worker) complete(ctx context.Context, run execution, result phaseResult) error {
	persist, cancel := completionContext(ctx)
	defer cancel()
	_, err := w.Repository.CompleteProviderOperation(persist, run.operation.Receipt.ID, providerops.Completion{Status: result.status, ErrorCode: result.code}, time.Now().UTC())
	return err
}

func completionContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() != nil {
		return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	}
	return context.WithCancel(ctx)
}

// current is invoked inside each queued provider callback, after any queue wait.
func (w *Worker) current(ctx context.Context, run execution) (model.PMConversation, error) {
	if err := ctx.Err(); err != nil {
		return model.PMConversation{}, err
	}
	conversation, err := w.Repository.PMConversation(ctx, run.conversationID)
	if err != nil {
		return conversation, err
	}
	enabled, group, err := Configuration(ctx, w.Settings)
	if err != nil {
		return conversation, err
	}
	if !enabled || group != conversation.ChatID {
		return conversation, ErrDisabled
	}
	if run.operation.Receipt.Kind == providerops.KindTelegramPMRelay {
		qualified, err := w.Repository.PanelEntryQualified(ctx, conversation.TelegramID)
		if err != nil {
			return conversation, err
		}
		if !qualified {
			return conversation, ErrUnqualified
		}
		if run.inbound && conversation.Blocked {
			return conversation, ErrBlocked
		}
	}
	if run.operation.Receipt.Kind != providerops.KindTelegramPMRelay || !run.inbound {
		actor, err := w.Repository.UserByID(ctx, run.operation.ActorUserID)
		if err != nil {
			return conversation, err
		}
		if !w.AdminIDs[actor.TelegramID] || actor.Role != "admin" {
			return conversation, ErrBlocked
		}
	}
	return conversation, nil
}
