package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGroupMemberTagQueueAndExecutionTimeChecks(t *testing.T) {
	var calls, mutations atomic.Int32
	var joined atomic.Bool
	var restricted atomic.Bool
	joined.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body struct {
			UserID int64  `json:"user_id"`
			Tag    string `json:"tag"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":99,"is_bot":true,"first_name":"bot","username":"tag_test_bot"}}`))
		case strings.HasSuffix(r.URL.Path, "/getChatMember"):
			result := map[string]any{"status": "member", "tag": "old"}
			if !joined.Load() {
				result["status"] = "left"
			}
			if joined.Load() && restricted.Load() {
				result["status"] = "restricted"
				result["is_member"] = true
				result["can_edit_tag"] = false
			}
			if body.UserID == 99 {
				result = map[string]any{"status": "administrator", "can_manage_tags": true}
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result}); err != nil {
				t.Error(err)
			}
		case strings.HasSuffix(r.URL.Path, "/setChatMemberTag"):
			mutations.Add(1)
			if body.Tag != "" {
				t.Errorf("clear payload tag=%q", body.Tag)
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			t.Errorf("unexpected method: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := telegram.NewClient("123:test", telegram.WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "tags", Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	a := telegramAdapter{client: &queuedTelegram{client: client, queue: queue}}
	if err := a.SetGroupMemberTag(context.Background(), "-100", 42, ""); !errors.Is(err, upstreamqueue.ErrNotRunning) || calls.Load() != 0 {
		t.Fatalf("queue bypass: %v calls=%d", err, calls.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := queue.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupMemberTag(ctx, "-100", 42, ""); err != nil || mutations.Load() != 1 {
		t.Fatalf("clear: %v mutations=%d", err, mutations.Load())
	}
	joined.Store(false)
	if err := a.SetGroupMemberTag(ctx, "-100", 42, "new"); !errors.Is(err, accounts.ErrGroupTagUnavailable) || mutations.Load() != 1 {
		t.Fatalf("departed member: %v mutations=%d", err, mutations.Load())
	}
	joined.Store(true)
	restricted.Store(true)
	if err := a.SetGroupMemberTag(ctx, "-100", 42, "new"); !errors.Is(err, accounts.ErrGroupTagUnavailable) || mutations.Load() != 1 {
		t.Fatalf("restricted member: %v mutations=%d", err, mutations.Load())
	}
}
