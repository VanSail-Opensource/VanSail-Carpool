package accounts

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestCleanupProbeLoadsCommittedNonceBeforeBothQueuedMethods(t *testing.T) {
	s, repo, client := cleanupFixture()
	client.action = false
	client.actionErr = &telegram.APIError{Method: "sendChatAction", HTTPStatus: 403, ErrorCode: 403, Description: "Forbidden: bot was blocked by the user"}
	if err := s.HandleOutbox(context.Background(), cleanupFixtureJob()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repo.trace.snapshot(), []string{"load-nonce", "action", "chat", "complete"}) {
		t.Fatal("method calls preceded durable nonce authorization")
	}
	if len(repo.observations) != 1 || repo.observations[0] != (model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}) {
		t.Fatal("expected paired blocked/private observation was not preserved")
	}
}

func TestCleanupTransientAndContradictoryResponsesNeverBecomeNegativeEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		chat telegram.ChatInfo
	}{
		{"timeout", context.DeadlineExceeded, telegram.ChatInfo{ID: 42, Type: "private"}},
		{"throttled", &telegram.APIError{Method: "sendChatAction", HTTPStatus: 429, ErrorCode: 429, RetryAfter: 10}, telegram.ChatInfo{ID: 42, Type: "private"}},
		{"foreign chat", errors.New("unknown"), telegram.ChatInfo{ID: 99, Type: "private"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, repo, client := cleanupFixture()
			client.action = false
			client.actionErr = test.err
			client.chat = test.chat
			if err := s.HandleOutbox(context.Background(), cleanupFixtureJob()); err != nil {
				t.Fatal(err)
			}
			if len(repo.observations) != 1 || repo.observations[0].SendAction != "unknown" {
				t.Fatal("transient response became account failure evidence")
			}
			if test.name == "foreign chat" && repo.observations[0].GetChat != "unknown" {
				t.Fatal("unrelated private identity was accepted")
			}
		})
	}
}

func TestCleanupProtectedOrCanceledProbeMakesNoTelegramCalls(t *testing.T) {
	for _, kind := range []string{"configured admin", "stored admin or activity cancellation", "wrong bot"} {
		t.Run(kind, func(t *testing.T) {
			s, repo, client := cleanupFixture()
			if kind == "configured admin" {
				s, _ = NewAccountCleanup(repo, client, 123, []int64{42})
			}
			if kind == "stored admin or activity cancellation" {
				repo.probe = nil
			}
			if kind == "wrong bot" {
				repo.probe.BotID = 456
			}
			if err := s.HandleOutbox(context.Background(), cleanupFixtureJob()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(repo.trace.snapshot(), []string{"load-nonce"}) || len(repo.observations) != 0 {
				t.Fatal("protected/canceled work reached Telegram or evidence completion")
			}
		})
	}
}

func TestOnlyOneCleanupProbeCanOccupyAnOutboxWorker(t *testing.T) {
	s, repo, client := cleanupFixture()
	started, release := make(chan struct{}), make(chan struct{})
	client.actionHook = func(ctx context.Context) (bool, error) {
		close(started)
		select {
		case <-release:
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() { done <- s.HandleOutbox(context.Background(), cleanupFixtureJob()) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	var wait *outbox.WaitError
	if err := s.HandleOutbox(context.Background(), cleanupFixtureJob()); !errors.As(err, &wait) {
		t.Fatalf("concurrent probe = %v", err)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("probe did not release")
	}
	if len(repo.observations) != 1 {
		t.Fatal("concurrent probe duplicated external work")
	}
}
