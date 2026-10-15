package usdt

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPageLimitsRejectBeforeOversizedSlices(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target any
		count  int
	}{
		{"activities", new(transferPage[aptosActivity]), 101},
		{"accounts", new(accountPage[string]), 129},
		{"logs", new(logPage[evmLog]), 2001},
		{"tron", new(historyPage[tronActivity]), 201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "[" + strings.TrimSuffix(strings.Repeat("null,", tc.count), ",") + "]"
			if json.Unmarshal([]byte(raw), tc.target) == nil {
				t.Fatal("excess page accepted")
			}
		})
	}
}

func TestResponseGraphCannotExpandSmallBodyIntoHugeSlice(t *testing.T) {
	raw := "{\"id\":1,\"result\":[" + strings.TrimSuffix(strings.Repeat("{},", 9000), ",") + "]}"
	var result struct{ Result []evmLog }
	if err := decodeResponse(strings.NewReader(raw), &result); err == nil || len(result.Result) != 0 {
		t.Fatal("graph exceeded allocation bound")
	}
}

func TestSolanaCursorPrunesOnlyUnwatchedOwners(t *testing.T) {
	cursor := solCursor{Baseline: 42, Position: 2, Accounts: map[string]solHistory{
		"current":  {Owner: "active", Last: "saved"},
		"closed":   {Owner: "late-payment", Before: "pending"},
		"obsolete": {Owner: "removed", Last: "old"},
	}}
	pruneSolanaOwners(&cursor, []string{"active", "late-payment"})
	if len(cursor.Accounts) != 2 || cursor.Accounts["closed"].Before != "pending" || cursor.Accounts["current"].Last != "saved" || cursor.Baseline != 42 || cursor.Position != 0 {
		t.Fatalf("cursor=%+v", cursor)
	}
}

func TestExecutorCancellationJoinsStartedCallback(t *testing.T) {
	executor := newExecutor()
	lifecycle, cancelLife := context.WithCancel(context.Background())
	defer cancelLife()
	if err := executor.configure(lifecycle, 1); err != nil {
		t.Fatal(err)
	}
	defer executor.close(context.Background())
	ctx, cancel := context.WithCancel(lifecycle)
	entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		_ = executor.do(ctx, func(context.Context) error { close(entered); <-release; return nil })
		close(returned)
	}()
	<-entered
	cancel()
	select {
	case <-returned:
		t.Fatal("caller returned while callback retained output")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("cancelled callback did not join")
	}
}
