package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ApplyHourlyCreditProof durably settles one original authenticated obligation.
// It remains available after account closure and day expiry. A recovery proof
// never returns unused capacity; late progress may confirm its unknown bytes.
func (s *Store) ApplyHourlyCreditProof(ctx context.Context, node, agent string, proof model.HourlyCreditProof) (model.HourlyCreditReservation, error) {
	if validateHourlyCreditProof(proof) != nil || node != proof.NodeID || agent != proof.AgentID {
		return model.HourlyCreditReservation{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyCreditReservation, error) {
		r, err := hourlyCreditReservationTx(ctx, tx, proof.ReservationID)
		if err != nil {
			return r, err
		}
		if r.HourlyCreditRequest != proof.HourlyCreditRequest || r.IssueSequence != proof.IssueSequence {
			return r, ErrConflict
		}
		if r.FinalProofDigest != "" {
			if r.FinalProofDigest != proof.Digest || r.Recovery != proof.Recovery || !r.Recovery && (r.AdmittedUploadBytes != proof.AdmittedUploadBytes || r.AdmittedDownloadBytes != proof.AdmittedDownloadBytes || r.UnusedBytes != proof.UnusedBytes) {
				return r, ErrConflict
			}
			return r, nil
		}
		return applyHourlyCreditProofTx(ctx, tx, r, proof)
	})
}

func applyHourlyCreditProofTx(ctx context.Context, tx *sql.Tx, r model.HourlyCreditReservation, p model.HourlyCreditProof) (model.HourlyCreditReservation, error) {
	day, err := hourlyCreditDayTx(ctx, tx, r.OwnerID, r.AccountGeneration, r.DayUTC)
	if err != nil {
		return r, err
	}
	var admitted, release int64
	originalReserved := r.ReservedBytes()
	if p.Recovery {
		r.UnknownBytes = r.ByteCeiling - r.AdmittedUploadBytes - r.AdmittedDownloadBytes
	} else {
		if p.AdmittedUploadBytes < r.AdmittedUploadBytes || p.AdmittedDownloadBytes < r.AdmittedDownloadBytes {
			return r, ErrConflict
		}
		admitted = p.AdmittedUploadBytes + p.AdmittedDownloadBytes - r.AdmittedUploadBytes - r.AdmittedDownloadBytes
		release = originalReserved
		r.AdmittedUploadBytes, r.AdmittedDownloadBytes, r.UnusedBytes = p.AdmittedUploadBytes, p.AdmittedDownloadBytes, p.UnusedBytes
	}
	reserved := day.FreeReservedBytes
	if r.Kind == "extra" {
		reserved = day.ExtraReservedBytes
	}
	if originalReserved > reserved {
		return r, ErrConflict
	}
	if err = adjustHourlyCreditDayTx(ctx, tx, r.HourlyCreditRequest, admitted, -release); err != nil {
		return r, err
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `UPDATE hourly_credit_reservations SET admitted_upload_bytes=$1,admitted_download_bytes=$2,unused_bytes=$3,unknown_bytes=$4,final_proof_digest=$5,final_recovery=$6,finalized_at=$7 WHERE reservation_id=$8 AND final_proof_digest=''`, r.AdmittedUploadBytes, r.AdmittedDownloadBytes, r.UnusedBytes, r.UnknownBytes, p.Digest, p.Recovery, stamp(now), r.ReservationID)
	if err != nil {
		return r, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return r, err
	}
	if count != 1 {
		return r, ErrConflict
	}
	r.FinalProofDigest, r.Recovery, r.FinalizedAt = p.Digest, p.Recovery, &now
	return r, nil
}
