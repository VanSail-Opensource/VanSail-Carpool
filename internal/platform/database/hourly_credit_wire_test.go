package database

import (
	"math"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyProofDigestMatchesPinnedAgentWithExactLargeIntegers(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		install := wire.HourlyCreditInstall{ReservationID: "cross-module-obligation", IssueSequence: 9007199254740993, Owner: wire.HourlyOwner{ID: "89401", Generation: 9007199254740993}, DayUTC: "2026-11-09", AgentID: "11111111111111111111111111111111", BootID: "22222222222222222222222222222222", SessionID: "33333333333333333333333333333333", ByteCeiling: math.MaxInt64, Kind: "extra"}
		proof := wire.HourlyCreditProof{Install: install, Recovery: recovery}
		if recovery {
			proof.UnknownBytes = install.ByteCeiling
		} else {
			proof.AdmittedUploadBytes, proof.AdmittedDownloadBytes = 9007199254740993, 7
			proof.UnusedBytes = install.ByteCeiling - proof.AdmittedUploadBytes - proof.AdmittedDownloadBytes
		}
		proof.Digest = wire.HourlyProofDigest(proof)
		if err := wire.ValidateHourlyProof(proof); err != nil {
			t.Fatal(err)
		}
		stored := model.HourlyCreditProof{HourlyCreditRequest: model.HourlyCreditRequest{ReservationID: install.ReservationID, OwnerID: install.Owner.ID, AccountGeneration: int64(install.Owner.Generation), DayUTC: install.DayUTC, NodeID: "authenticated-node", AgentID: install.AgentID, BootID: install.BootID, SessionID: install.SessionID, Kind: install.Kind, ByteCeiling: int64(install.ByteCeiling)}, IssueSequence: int64(install.IssueSequence), AdmittedUploadBytes: int64(proof.AdmittedUploadBytes), AdmittedDownloadBytes: int64(proof.AdmittedDownloadBytes), UnusedBytes: int64(proof.UnusedBytes), UnknownBytes: int64(proof.UnknownBytes), Recovery: recovery, Digest: proof.Digest}
		digest, err := hourlyCreditProofDigest(stored)
		if err != nil || digest != proof.Digest || validateHourlyCreditProof(stored) != nil {
			t.Fatal("panel and signed-agent proof encodings differ", recovery, err)
		}
		stored.NodeID = "another-authenticated-node"
		if other, err := hourlyCreditProofDigest(stored); err != nil || other != digest {
			t.Fatal("transport node scope entered the agent digest", err)
		}
	}
}
