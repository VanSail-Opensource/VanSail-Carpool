package emby

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

// SuffixedUsername appends a stable eight-character hash to a colliding base name.
func SuffixedUsername(baseUsername, userID string) string {
	hash := sha256.Sum256([]byte(userID))
	return strings.TrimSpace(baseUsername) + "-" + hex.EncodeToString(hash[:4])
}

func normalizePreferences(preferences Preferences) Preferences {
	seen := make(map[string]struct{}, len(preferences.DisabledLibraryIDs))
	libraries := make([]string, 0, len(preferences.DisabledLibraryIDs))
	for _, libraryID := range preferences.DisabledLibraryIDs {
		libraryID = strings.TrimSpace(libraryID)
		if libraryID == "" {
			continue
		}
		if _, exists := seen[libraryID]; exists {
			continue
		}
		seen[libraryID] = struct{}{}
		libraries = append(libraries, libraryID)
	}
	sort.Strings(libraries)
	preferences.DisabledLibraryIDs = libraries
	return preferences
}

func passwordContext(userID string) string { return "emby.provisioning.password:" + userID }

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func safeFailure(err error) string {
	switch {
	case errors.Is(err, ErrAccountExists):
		return "The requested Emby username is unavailable."
	case errors.Is(err, ErrRemoteAccountMissing):
		return "The linked Emby account could not be found."
	case errors.Is(err, ErrInvalidSetup):
		return "The Emby setup fields are invalid."
	default:
		return "Emby provisioning could not be completed."
	}
}

// redactedRemoteError keeps the original cause available for errors.Is/As and
// terminal classification while ensuring a provider or transport cannot echo
// password material into logs, outbox diagnostics, or durable account errors.
type redactedRemoteError struct {
	cause   error
	message string
}

func (e redactedRemoteError) Error() string { return e.message }
func (e redactedRemoteError) Unwrap() error { return e.cause }

func redactRemoteError(cause error, message string) error {
	return redactedRemoteError{cause: cause, message: message}
}
