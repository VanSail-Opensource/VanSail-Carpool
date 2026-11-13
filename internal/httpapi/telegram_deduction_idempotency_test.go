package httpapi

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/botcommands"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestTelegramDeductionReplaysOnceAndRechecksRole(t *testing.T) {
	ctx := context.Background()
	s, _, actor := testBrowserHTTP(t)
	s.deps.Admin = admin.NewService(s.deps.Store, nil, nil, nil, nil)
	s.deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.deps.Telegram = &boostReplyCapture{}
	target, _, err := s.deps.Store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: actor.TelegramID + 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.deps.Store.AdjustAdminBalance(ctx, actor.ID, target.ID, 1000, "deduction-seed", "fixture", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	message := &telegram.Message{MessageID: 11, Chat: telegram.Chat{ID: -10051, Type: "supergroup"},
		From: &telegram.User{ID: actor.TelegramID}, ReplyToMessage: &telegram.Message{MessageID: 9, From: &telegram.User{ID: target.TelegramID}}}
	command, _ := botcommands.Parse("/deduct 2.50")
	for range 2 {
		s.processTelegramDeduction(ctx, message, command, botcommands.Text(botcommands.English))
	}
	assertBalance := func(want string) {
		t.Helper()
		balance, err := s.deps.Store.Balance(ctx, target.ID)
		if err != nil || balance.Minor != want {
			t.Fatalf("balance = %s, %v; want %s", balance.Minor, err, want)
		}
	}
	assertBalance("750")
	changed, _ := botcommands.Parse("/deduct 3.00")
	s.processTelegramDeduction(ctx, message, changed, botcommands.Text(botcommands.English))
	assertBalance("750")
	message.MessageID++
	s.processTelegramDeduction(ctx, message, command, botcommands.Text(botcommands.English))
	assertBalance("500")
	if _, err := s.deps.Store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	message.MessageID++
	s.processTelegramDeduction(ctx, message, command, botcommands.Text(botcommands.English))
	assertBalance("500")
	entries, err := s.deps.Store.ListLedger(ctx, target.ID, 10)
	if err != nil || len(entries) != 3 {
		t.Fatalf("ledger count = %d, %v; want seed and two distinct deductions", len(entries), err)
	}
}
