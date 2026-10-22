package database

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type timestampCursor struct {
	Timestamp string `json:"t"`
	ID        string `json:"i"`
	Filter    string `json:"f"`
}

var timestampUserIDPattern = regexp.MustCompile(`^[1-9][0-9]{0,15}$`)

func encodeTimestampCursor(createdAt time.Time, id, filter string) (string, error) {
	payload, err := json.Marshal(timestampCursor{Timestamp: stamp(createdAt), ID: id, Filter: filter})
	if err != nil {
		return "", fmt.Errorf("encode timestamp cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeTimestampCursor(value, filter string) (timestampCursor, error) {
	if len(value) < 16 || len(value) > 512 {
		return timestampCursor{}, ErrInvalidCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return timestampCursor{}, ErrInvalidCursor
	}
	var cursor timestampCursor
	if json.Unmarshal(payload, &cursor) != nil {
		return timestampCursor{}, ErrInvalidCursor
	}
	validID := cursorIDPattern.MatchString(cursor.ID) || timestampUserIDPattern.MatchString(cursor.ID)
	if !validID || cursor.Filter != filter {
		return timestampCursor{}, ErrInvalidCursor
	}
	parsed, err := time.Parse(time.RFC3339Nano, cursor.Timestamp)
	if err != nil || stamp(parsed) != cursor.Timestamp {
		return timestampCursor{}, ErrInvalidCursor
	}
	return cursor, nil
}

func pageFilterFingerprint(values ...string) string {
	normalized := make([]string, len(values))
	for index, value := range values {
		normalized[index] = strings.ToLower(strings.TrimSpace(value))
	}
	digest := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return base64.RawURLEncoding.EncodeToString(digest[:12])
}
