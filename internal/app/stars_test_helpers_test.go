package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func starsHistoryTelegramForTest(t *testing.T, history func(offset, limit int) ([]telegram.StarTransaction, error)) *queuedTelegram {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var page struct{ Offset, Limit int }
		if r.URL.Path != "/bot123:stars-test/getStarTransactions" || json.NewDecoder(r.Body).Decode(&page) != nil {
			t.Errorf("unexpected Stars request: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		transactions, err := history(page.Offset, page.Limit)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":503,"description":"unavailable"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"transactions": transactions}})
	}))
	t.Cleanup(server.Close)
	client, err := telegram.NewClient("123:stars-test", telegram.WithBaseURL(server.URL), telegram.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "stars-test", Capacity: 8})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := queue.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return &queuedTelegram{client: client, queue: queue}
}
