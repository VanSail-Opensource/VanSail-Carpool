package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestStatisticsPartitionPreservesExactJSONNumbers(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	ctx := context.Background()
	generated := time.Date(2026, 11, 13, 15, 0, 0, 123456789, time.UTC)
	for _, raw := range []string{
		`{"groupMessagesTotal":9007199254740993}`,
		`{"nested":{"signed":-9223372036854775808,"unsigned":18446744073709551615},"values":[9007199254740993,0.1234567890123456789]}`,
	} {
		if err := store.SaveStatisticsPartition(ctx, "database", []byte(raw), generated); err != nil {
			t.Fatal(err)
		}
		payload, at, err := store.LoadStatisticsPartition(ctx, "database")
		if err != nil || !at.Equal(generated) {
			t.Fatalf("load exact partition = %s, %v, %v", payload, at, err)
		}
		var expected, actual map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &expected); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(payload, &actual); err != nil {
			t.Fatal(err)
		}
		for key, value := range expected {
			if string(actual[key]) != string(value) {
				t.Fatalf("partition rounded %s: got %s, want %s", key, actual[key], value)
			}
		}
	}
}

func TestStatisticsPartitionRejectsNonObjectOrTrailingJSON(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	for _, raw := range []string{"null", "[]", "123", `{} {}`, `{"amount":NaN}`} {
		if err := store.SaveStatisticsPartition(context.Background(), "database", []byte(raw), time.Now()); err == nil {
			t.Fatalf("accepted invalid snapshot %q", raw)
		}
	}
}
