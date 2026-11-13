package telegrampm

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type settledItemRepository struct {
	*workerRepo
	status providerops.Status
	writes int
}

func (r *settledItemRepository) BeginProviderOperationItemAttempt(_ context.Context, _, key string, _ time.Time) (providerops.Item, error) {
	item := r.items[key]
	item.Status = r.status
	r.items[key] = item
	return item, nil
}

func (r *settledItemRepository) CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error) {
	r.writes++
	return providerops.Item{}, nil
}

func (r *settledItemRepository) CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error) {
	r.writes++
	return r.operation, nil
}

func TestPMWorkerStopsWhenClaimReturnsChangedState(t *testing.T) {
	for _, status := range []providerops.Status{providerops.StatusQueued, providerops.StatusSucceeded, providerops.StatusFailed, providerops.StatusPendingReview} {
		t.Run(string(status), func(t *testing.T) {
			worker, original, sender := newWorkerFixture()
			repository := &settledItemRepository{workerRepo: original, status: status}
			worker.Repository = repository
			if err := worker.HandleProviderOperation(context.Background(), original.operation, model.OutboxJob{}); err != nil {
				t.Fatal(err)
			}
			if original.claims != 0 || sender.creates != 0 || sender.profiles != 0 || sender.edits != 0 || sender.copies != 0 || repository.writes != 0 {
				t.Fatal("changed item state allowed a provider call or receipt overwrite", original.claims, sender, repository.writes)
			}
			if original.items["topic"].Status != status {
				t.Fatal("competing receipt state was overwritten")
			}
		})
	}
}

type largeRepairProfile struct{}

func (largeRepairProfile) PMProfileFacts(context.Context, string) (ProfileFacts, error) {
	return ProfileFacts{OnboardingState: "complete", ActiveCombo: &model.Purchase{ComboName: "Core"},
		SquadNames: []string{strings.Repeat("Squad_[🌊] ", 600)}}, nil
}

type repairRequestSender struct {
	*workerSender
	request telegram.TopicProfileRequest
}

func (s *repairRequestSender) EditPMProfile(ctx context.Context, guard func(context.Context) (telegram.TopicProfileRequest, error)) (telegram.Message, error) {
	request, err := guard(ctx)
	if err != nil {
		return telegram.Message{}, err
	}
	s.request = request
	return telegram.Message{MessageID: request.MessageID, MessageThreadID: request.MessageThreadID, Chat: telegram.Chat{ID: request.ChatID}}, nil
}

func TestPMRepairKeepsVerificationProbeWithinMessageLimit(t *testing.T) {
	worker, repository, originalSender := newWorkerFixture()
	repository.operation.Receipt.Kind = providerops.KindTelegramPMRepair
	repository.operation.ActorUserID = "admin"
	repository.conversation.TopicState = "pending_review"
	repository.items = map[string]providerops.Item{"repair": {Key: "repair", TargetType: "pm_topic_recovery", TargetID: "conversation:90:91", Status: providerops.StatusQueued}}
	sender := &repairRequestSender{workerSender: originalSender}
	worker.Sender, worker.Profiles = sender, largeRepairProfile{}
	if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sender.request.Text, "Profile checked at ") || utf8.RuneCountInString(sender.request.Text) > 4096 {
		t.Fatal("repair probe was truncated or exceeded the profile transport limit", utf8.RuneCountInString(sender.request.Text))
	}
	if repository.operation.Receipt.Status != "succeeded" || repository.conversation.ProfileMessageID != 91 {
		t.Fatal("bounded repair did not bind the verified profile")
	}
}
