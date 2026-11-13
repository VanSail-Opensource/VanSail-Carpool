package usdt

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/jsonlimit"
)

// MaxCursorBytes bounds both persisted progress and its typed decoding graph.
const MaxCursorBytes = 2 * 1024 * 1024
const MaxCursorHashBytes = 128
const maxSolanaCursorAccounts = 4096

// ValidateCursor refuses oversized progress without discarding earlier history.
func ValidateCursor(cursor Cursor) error {
	if len(cursor.Value) > MaxCursorBytes || len(cursor.Hash) > MaxCursorHashBytes {
		return ErrCapacity
	}
	return nil
}

func decodeCursor(raw string, target any) error {
	if raw == "" {
		return nil
	}
	if err := ValidateCursor(Cursor{Value: raw}); err != nil {
		return err
	}
	limits := jsonlimit.Limits{Depth: 8, ArrayItems: 128, Nodes: 16*maxSolanaCursorAccounts + 32, StringBytes: MaxCursorBytes}
	if _, err := io.Copy(io.Discard, jsonlimit.NewReader(strings.NewReader(raw), limits)); err != nil {
		if errors.Is(err, jsonlimit.ErrLimit) {
			return ErrCapacity
		}
		return ErrProof
	}
	if json.Unmarshal([]byte(raw), target) != nil {
		return ErrProof
	}
	return nil
}

func trackSolanaAccount(accounts map[string]string, account, owner string) error {
	if previous, found := accounts[account]; found {
		if previous != owner {
			return ErrProof
		}
		return nil
	}
	if len(accounts) >= maxSolanaCursorAccounts {
		return ErrCapacity
	}
	accounts[account] = owner
	return nil
}
