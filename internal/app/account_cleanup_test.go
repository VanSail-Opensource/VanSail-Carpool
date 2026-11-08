package app

import (
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestCleanupBotPrefixFailureDoesNotExposeToken(t *testing.T) {
	for _, token := range []string{"private-token-without-prefix", "invalid:private-token", "9223372036854775808:private-token"} {
		_, err := newAccountCleanup(nil, &queuedTelegram{}, token, nil, outbox.NewWorker(nil))
		if err == nil || strings.Contains(err.Error(), "private-token") {
			t.Fatal("malformed token was accepted or exposed")
		}
	}
}
