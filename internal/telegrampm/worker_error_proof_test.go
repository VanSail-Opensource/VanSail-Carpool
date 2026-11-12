package telegrampm

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPMWorkerContradictoryResponsesNeverReplayAppendOnlyPhases(t *testing.T) {
	for _, failure := range []*telegram.APIError{
		{HTTPStatus: 503, ErrorCode: 400, Description: "Bad Request: message thread not found"},
		{HTTPStatus: 200, ErrorCode: 400, Description: "Bad Request: topic deleted"},
		{HTTPStatus: 503, ErrorCode: 429, Description: "Too Many Requests", RetryAfter: 3},
		{HTTPStatus: 400, ErrorCode: 400, Description: "Bad Request: message thread not found", RetryAfter: 3},
	} {
		for _, phase := range []string{"topic", "profile", "relay"} {
			t.Run(phase+"/"+failure.Error(), func(t *testing.T) {
				worker, repository, sender := newWorkerFixture()
				switch phase {
				case "topic":
					sender.createErr = failure
				case "profile":
					sender.profileErr = failure
				case "relay":
					sender.copyErr = failure
				}
				if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{Attempts: 1}); err != nil {
					t.Fatal(err)
				}
				if repository.operation.Receipt.Status != "pending_review" || repository.resets != 0 {
					t.Fatal("ambiguous append-only response did not retain uncertainty", repository.operation, repository.resets)
				}
				creates, profiles, copies := sender.creates, sender.profiles, sender.copies
				if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{Attempts: 2}); err != nil {
					t.Fatal(err)
				}
				if sender.creates != creates || sender.profiles != profiles || sender.copies != copies || repository.resets != 0 {
					t.Fatal("ambiguous phase replayed or erased topic references")
				}
			})
		}
	}
}

func TestPMWorkerAmbiguousEditDoesNotEraseKnownProfile(t *testing.T) {
	worker, repository, sender := newWorkerFixture()
	repository.conversation.TopicState = "ready"
	repository.conversation.TopicID = 12
	repository.conversation.ProfileMessageID = 61
	sender.profileErr = &telegram.APIError{HTTPStatus: 503, ErrorCode: 400, Description: "Bad Request: message to edit not found"}
	if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{Attempts: 1}); err == nil {
		t.Fatal("idempotent edit should remain retryable")
	}
	if repository.conversation.ProfileMessageID != 61 || sender.edits != 1 || sender.profiles != 0 || sender.copies != 0 || repository.items["profile"].Status != providerops.StatusQueued {
		t.Fatal("ambiguous edit erased the existing profile or copied early")
	}
	sender.profileErr = nil
	if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{Attempts: 2}); err != nil {
		t.Fatal(err)
	}
	if sender.edits != 2 || sender.profiles != 0 || sender.copies != 1 {
		t.Fatal("retry did not edit the existing profile exactly once before relay")
	}
}

func TestPMWorkerAmbiguousFooterCannotResetDeliveredRelay(t *testing.T) {
	worker, repository, sender := attributionFixture("copy-footer")
	repository.items["footer"] = providerops.Item{Key: "footer", TargetType: "pm_outbound_footer", TargetID: "-100123:4", Status: providerops.StatusQueued}
	sender.textErr = &telegram.APIError{HTTPStatus: 503, ErrorCode: 400, Description: "Bad Request: topic not found"}
	if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	if sender.copies != 1 || len(sender.texts) != 1 || repository.items["relay"].Status != providerops.StatusSucceeded || repository.operation.Receipt.Status != "pending_review" || repository.resets != 0 {
		t.Fatal("ambiguous footer did not retain the completed relay")
	}
	sender.textErr = nil
	if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{Attempts: 2}); err != nil {
		t.Fatal(err)
	}
	if sender.copies != 1 || len(sender.texts) != 1 || repository.resets != 0 {
		t.Fatal("ambiguous footer replayed a completed delivery")
	}
}

func TestPMMissingReferenceRequiresCanonicalDefinitiveProof(t *testing.T) {
	for _, description := range []string{
		"Bad Request: message thread not found elsewhere",
		"Bad Request: topic not found elsewhere",
		"Bad Request: topic deleted elsewhere",
		"Bad Request: message to edit not found elsewhere",
	} {
		failure := &telegram.APIError{HTTPStatus: 400, ErrorCode: 400, Description: description}
		if topicMissing(failure) || profileMissing(failure) {
			t.Fatal("noncanonical description erased a reference", description)
		}
	}
}
