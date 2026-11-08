package accounts

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

func TestCleanupBotVerificationPausesBeforeGetMeAndRequiresConfiguredIdentity(t *testing.T) {
	for _, test := range []struct {
		name string
		id   int64
		err  error
		want []int64
	}{
		{"verified", 123, nil, []int64{0, 123}},
		{"other configured bot", 456, nil, []int64{0}},
		{"network failure", 0, errors.New("network timeout"), []int64{0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, repo, client := cleanupFixture()
			client.bot.ID = test.id
			client.meErr = test.err
			err := s.VerifyBot(context.Background())
			if (err == nil) != (test.name == "verified") {
				t.Fatalf("verification = %v", err)
			}
			if !reflect.DeepEqual(repo.botIDs, test.want) {
				t.Fatal("unverified bot restored cleanup admission")
			}
			calls := repo.trace.snapshot()
			if len(calls) < 2 || calls[0] != "bot-state" || calls[1] != "getMe" {
				t.Fatal("verification started before durable admission pause")
			}
		})
	}
}

func TestProbeAuthenticationFailurePausesGlobalAdmission(t *testing.T) {
	s, repo, client := cleanupFixture()
	client.action = false
	client.actionErr = &telegram.APIError{Method: "sendChatAction", HTTPStatus: 401, ErrorCode: 401, Description: "Unauthorized"}
	if err := s.HandleOutbox(context.Background(), cleanupFixtureJob()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repo.botIDs, []int64{0}) || len(repo.observations) != 1 || repo.observations[0].SendAction != "unknown" {
		t.Fatal("authentication failure became account evidence or failed to pause")
	}
}

func TestCleanupScheduleUsesBoundedExistingQueueAndNoTelegram(t *testing.T) {
	s, repo, _ := cleanupFixture()
	if err := s.Schedule(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if repo.limit != 64 || len(repo.trace.snapshot()) != 0 {
		t.Fatal("schedule made external calls or exceeded repository admission bound")
	}
}

func TestPositiveActionStillReachesCancellationAfterPairedAuthenticationFailure(t *testing.T) {
	s, repo, client := cleanupFixture()
	client.chat = telegram.ChatInfo{}
	client.chatErr = &telegram.APIError{Method: "getChat", HTTPStatus: 401, ErrorCode: 401, Description: "Unauthorized"}
	if err := s.HandleOutbox(context.Background(), cleanupFixtureJob()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repo.botIDs, []int64{0}) || len(repo.observations) != 1 || repo.observations[0].SendAction != "reachable" || repo.observations[0].GetChat != "unknown" {
		t.Fatal("positive cancellation was lost during identity suspension")
	}
}
