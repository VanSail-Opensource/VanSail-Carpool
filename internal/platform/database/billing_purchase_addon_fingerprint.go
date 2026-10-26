package database

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

func normalizeAndFingerprintPurchaseAddons(input *PurchaseAddonInput) (string, error) {
	if input == nil {
		return "", errors.New("purchase add-on input is required")
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.PurchaseID = strings.TrimSpace(input.PurchaseID)
	input.CouponGrantID = strings.TrimSpace(input.CouponGrantID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.UserID == "" || input.PurchaseID == "" || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
		return "", errors.New("purchase, user, and idempotency key are required")
	}
	seen := make(map[string]struct{}, len(input.AddonSquadIDs))
	for index := range input.AddonSquadIDs {
		input.AddonSquadIDs[index] = strings.TrimSpace(input.AddonSquadIDs[index])
		if input.AddonSquadIDs[index] == "" {
			return "", errors.New("add-on squad is required")
		}
		if _, exists := seen[input.AddonSquadIDs[index]]; exists {
			return "", errors.New("duplicate add-on squad")
		}
		seen[input.AddonSquadIDs[index]] = struct{}{}
	}
	input.AddonSquadIDs = uniqueSorted(input.AddonSquadIDs)
	if len(input.AddonSquadIDs) == 0 || len(input.AddonSquadIDs) > 100 {
		return "", errors.New("one to one hundred add-on squads are required")
	}
	activationIDs := make([]string, 0, len(input.SquadActivationCodes))
	for squadID := range input.SquadActivationCodes {
		activationIDs = append(activationIDs, strings.TrimSpace(squadID))
	}
	payload, err := json.Marshal(struct {
		CouponGrantID string   `json:"couponGrantId,omitempty"`
		Version       int      `json:"version"`
		PurchaseID    string   `json:"purchaseId"`
		AddonSquadIDs []string `json:"addonSquadIds"`
		ActivationIDs []string `json:"activationIds"`
	}{CouponGrantID: input.CouponGrantID, Version: 1, PurchaseID: input.PurchaseID, AddonSquadIDs: input.AddonSquadIDs, ActivationIDs: uniqueSorted(activationIDs)})
	if err != nil {
		return "", fmt.Errorf("encode purchase add-on fingerprint: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func proratedAddonPrice(fullPrice int64, remaining, original time.Duration) (int64, error) {
	if fullPrice < 0 || original <= 0 {
		return 0, errors.New("invalid squad proration basis")
	}
	if fullPrice == 0 || remaining <= 0 {
		return 0, nil
	}
	if remaining >= original {
		return fullPrice, nil
	}
	numerator := new(big.Int).Mul(big.NewInt(fullPrice), big.NewInt(remaining.Nanoseconds()))
	denominator := big.NewInt(original.Nanoseconds())
	numerator.Add(numerator, new(big.Int).Sub(denominator, big.NewInt(1)))
	numerator.Quo(numerator, denominator)
	if !numerator.IsInt64() {
		return 0, errors.New("prorated squad price exceeds integer range")
	}
	price := numerator.Int64()
	if price > fullPrice {
		return fullPrice, nil
	}
	return price, nil
}
