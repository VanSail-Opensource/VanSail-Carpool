package database

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// This canonical v1 envelope is independent of the agent module pin. NodeID is
// authenticated transport scope; the original install supplies the hashed scope.
func hourlyCreditProofDigest(p model.HourlyCreditProof) (string, error) {
	type owner struct {
		ID         string `json:"id"`
		Generation int64  `json:"generation,string"`
	}
	type install struct {
		ReservationID string `json:"reservation_id"`
		IssueSequence int64  `json:"issue_sequence,string"`
		Owner         owner  `json:"owner"`
		DayUTC        string `json:"day_utc"`
		AgentID       string `json:"agent_id"`
		BootID        string `json:"boot_id"`
		SessionID     string `json:"session_id"`
		ByteCeiling   int64  `json:"byte_ceiling,string"`
		Kind          string `json:"kind"`
	}
	envelope := struct {
		Install               install `json:"install"`
		AdmittedUploadBytes   int64   `json:"admitted_upload_bytes,string"`
		AdmittedDownloadBytes int64   `json:"admitted_download_bytes,string"`
		UnusedBytes           int64   `json:"unused_bytes,string"`
		UnknownBytes          int64   `json:"unknown_bytes,string"`
		Recovery              bool    `json:"recovery"`
		Digest                string  `json:"digest"`
	}{Install: install{p.ReservationID, p.IssueSequence, owner{p.OwnerID, p.AccountGeneration}, p.DayUTC, p.AgentID, p.BootID, p.SessionID, p.ByteCeiling, p.Kind}, AdmittedUploadBytes: p.AdmittedUploadBytes, AdmittedDownloadBytes: p.AdmittedDownloadBytes, UnusedBytes: p.UnusedBytes, UnknownBytes: p.UnknownBytes, Recovery: p.Recovery}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func validateHourlyCreditProof(p model.HourlyCreditProof) error {
	if validateHourlyCreditRequest(p.HourlyCreditRequest) != nil || p.IssueSequence <= 0 || !validHourlyCreditHex(p.Digest, 64) {
		return ErrConflict
	}
	remaining := p.ByteCeiling
	for _, value := range []int64{p.AdmittedUploadBytes, p.AdmittedDownloadBytes, p.UnusedBytes, p.UnknownBytes} {
		if value < 0 || value > remaining {
			return ErrConflict
		}
		remaining -= value
	}
	if remaining != 0 || p.Recovery && (p.AdmittedUploadBytes != 0 || p.AdmittedDownloadBytes != 0 || p.UnusedBytes != 0 || p.UnknownBytes != p.ByteCeiling) || !p.Recovery && p.UnknownBytes != 0 {
		return ErrConflict
	}
	digest, err := hourlyCreditProofDigest(p)
	if err != nil {
		return err
	}
	if digest != p.Digest {
		return ErrConflict
	}
	return nil
}
