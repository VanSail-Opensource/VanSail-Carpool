package postgres

import (
	"crypto/sha256"
	"testing"
	"time"
)

func TestRecoveryDigestCanonicalNativeTimes(t *testing.T) {
	instant := time.Date(2026, 11, 12, 10, 11, 12, 123456789, time.UTC)
	digest := func(value any) string {
		t.Helper()
		h := sha256.New()
		if err := hashValue(h, value); err != nil {
			t.Fatal(err)
		}
		return string(h.Sum(nil))
	}
	want := digest(instant)
	if digest(instant.In(time.FixedZone("east", 9*60*60))) != want {
		t.Fatal("equivalent timestamp changed recovery evidence with its location")
	}
	for _, different := range []any{instant.Add(time.Nanosecond), instant.Format(time.RFC3339Nano), nil} {
		if digest(different) == want {
			t.Fatalf("native timestamp collided with distinct evidence %T", different)
		}
	}
}
