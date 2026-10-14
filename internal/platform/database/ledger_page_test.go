package database

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestLedgerCursorRoundTrip(t *testing.T) {
	entry := model.LedgerEntry{
		ID:        "8c62dd9f-59ae-4fe8-a8d5-a589ee2a4bd0",
		CreatedAt: time.Date(2026, 8, 10, 9, 30, 0, 123, time.UTC),
	}
	encoded, err := encodeLedgerCursor(entry)
	if err != nil {
		t.Fatalf("encodeLedgerCursor(): %v", err)
	}
	decoded, err := decodeLedgerCursor(encoded)
	if err != nil {
		t.Fatalf("decodeLedgerCursor(): %v", err)
	}
	if decoded.ID != entry.ID || decoded.CreatedAt != stamp(entry.CreatedAt) {
		t.Fatalf("decoded cursor = %#v", decoded)
	}
}

func TestLedgerCursorRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "not-base64", base64Cursor(`{"t":"2026-08-10T00:00:00Z","i":"../bad"}`)} {
		if _, err := decodeLedgerCursor(value); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("decodeLedgerCursor(%q) error = %v, want ErrInvalidCursor", value, err)
		}
	}
}

func base64Cursor(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}
