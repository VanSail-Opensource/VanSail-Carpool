package database

import (
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyProofDigestMatchesCanonicalV1Envelope(t *testing.T) {
	p := model.HourlyCreditProof{HourlyCreditRequest: model.HourlyCreditRequest{ReservationID: "reservation-one", OwnerID: "89401", AccountGeneration: 0, DayUTC: "2026-11-09", NodeID: "hourly-node", AgentID: "11111111111111111111111111111111", BootID: "22222222222222222222222222222222", SessionID: "33333333333333333333333333333333", Kind: "free", ByteCeiling: 1000}, IssueSequence: 1, AdmittedUploadBytes: 200, AdmittedDownloadBytes: 300, UnusedBytes: 500}
	const golden = "ccd05d92286eee2173cddd6174a862a0c9de85b0c67caba4838bd684568749c0"
	digest, err := hourlyCreditProofDigest(p)
	if err != nil || digest != golden {
		t.Fatal("canonical agent proof contract drifted", digest, err)
	}
	p.Digest = digest
	if err = validateHourlyCreditProof(p); err != nil {
		t.Fatal(err)
	}
	p.UnusedBytes++
	if err = validateHourlyCreditProof(p); !errors.Is(err, ErrConflict) {
		t.Fatal("tampered partition accepted", err)
	}
}
