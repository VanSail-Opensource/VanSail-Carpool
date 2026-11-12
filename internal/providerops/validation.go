package providerops

import (
	"encoding/json"
	"errors"
	"strings"
)

// NormalizeCreate validates and canonicalizes an operation command.
func NormalizeCreate(input CreateInput) (CreateInput, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.OwnerUserID = strings.TrimSpace(input.OwnerUserID)
	input.Kind = strings.TrimSpace(input.Kind)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.RequestFingerprint = strings.TrimSpace(input.RequestFingerprint)
	input.SealedTarget = strings.TrimSpace(input.SealedTarget)
	if input.ActorUserID == "" || input.Kind == "" || len(input.Kind) > 80 ||
		input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 ||
		len(input.RequestFingerprint) < 16 || len(input.RequestFingerprint) > 128 ||
		len(input.SealedTarget) > 16*1024 {
		return CreateInput{}, errors.New("invalid provider operation command")
	}
	seen := make(map[string]struct{}, len(input.Items))
	for index := range input.Items {
		item := &input.Items[index]
		item.Key = strings.TrimSpace(item.Key)
		item.TargetType = strings.TrimSpace(item.TargetType)
		item.TargetID = strings.TrimSpace(item.TargetID)
		if item.Key == "" || len(item.Key) > 160 || item.TargetType == "" ||
			len(item.TargetType) > 80 || item.TargetID == "" {
			return CreateInput{}, errors.New("invalid provider operation item")
		}
		if _, duplicate := seen[item.Key]; duplicate {
			return CreateInput{}, errors.New("duplicate provider operation item")
		}
		seen[item.Key] = struct{}{}
	}
	return input, nil
}

// Terminal reports whether no automatic provider retry is allowed.
func Terminal(status Status) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusCompensated, StatusPendingReview, StatusPartial:
		return true
	default:
		return false
	}
}

// ResultObject accepts only a bounded JSON object for sanitized metadata.
func ResultObject(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "{}", nil
	}
	if len(value) > 16*1024 || !json.Valid([]byte(value)) {
		return "", errors.New("operation result must be a JSON object")
	}
	var result map[string]any
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if decoder.Decode(&result) != nil || result == nil {
		return "", errors.New("operation result must be a JSON object")
	}
	canonical, err := json.Marshal(result)
	return string(canonical), err
}
