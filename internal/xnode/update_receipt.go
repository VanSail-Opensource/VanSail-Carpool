package xnode

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrUpdateEvidence = errors.New("X-Node update transaction evidence does not match")

// UpdateExpectation is durable command intent, persisted before native mutation.
// Rollback refers to the original install transaction and restores Previous.
type UpdateExpectation struct {
	Command       string `json:"command"`
	TransactionID string `json:"transactionId"`
	Version       string `json:"version"`
	Previous      string `json:"previous"`
}

func (e UpdateExpectation) Valid() bool {
	if e.Command != "update.install" && e.Command != "update.rollback" {
		return false
	}
	for _, value := range []string{e.TransactionID, e.Version, e.Previous} {
		if strings.TrimSpace(value) == "" || len(value) > 128 {
			return false
		}
	}
	return true
}

// UpdateTransaction is the documented, sanitized update.status projection.
type UpdateTransaction struct {
	ID       string    `json:"id"`
	Version  string    `json:"version"`
	Previous string    `json:"previous"`
	Status   string    `json:"status"`
	Deadline time.Time `json:"deadline"`
}

func (t UpdateTransaction) bounded() bool {
	return len(t.ID) <= 128 && len(t.Version) <= 128 && len(t.Previous) <= 128 && len(t.Status) <= 32
}

func UpdateExpectationFromResult(raw string) (UpdateExpectation, bool) {
	var value struct {
		Update UpdateExpectation `json:"updateExpectation"`
	}
	if json.Unmarshal([]byte(raw), &value) != nil {
		return UpdateExpectation{}, false
	}
	return value.Update, value.Update.Valid()
}

func (e UpdateExpectation) matches(transaction UpdateTransaction) bool {
	return e.Valid() && transaction.ID == e.TransactionID && transaction.Version == e.Version &&
		transaction.Previous == e.Previous && !transaction.Deadline.IsZero()
}

func (e UpdateExpectation) runningVersion() string {
	if e.Command == "update.rollback" {
		return e.Previous
	}
	return e.Version
}

func (e UpdateExpectation) confirmed(transaction UpdateTransaction) bool {
	status := "confirmed"
	if e.Command == "update.rollback" {
		status = "rolled_back"
	}
	return e.matches(transaction) && transaction.Status == status
}
