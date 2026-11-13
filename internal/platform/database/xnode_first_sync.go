package database

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// ConfirmXNodeFirstSync accepts only captured authenticated applied membership.
// Immutable period, owner generation and credential hashes prevent a delayed
// acknowledgment from proving readiness for replacement native material.
func (s *Store) retryAttempt_ConfirmXNodeFirstSync(ctx context.Context, node string, revision uint64, subjects []xnode.FirstSyncSubject, at time.Time) error {
	if revision == 0 || at.IsZero() || len(subjects) > 256 {
		return ErrConflict
	}
	if len(subjects) == 0 {
		return nil
	}
	principals, ownerPrincipals, generations := make([]int64, len(subjects)), make([]int64, len(subjects)), make([]int64, len(subjects))
	periods, owners := make([]string, len(subjects)), make([]string, len(subjects))
	credentials, ownerCredentials := make([]string, len(subjects)), make([]string, len(subjects))
	seen := make(map[int64]bool, len(subjects))
	for index, subject := range subjects {
		if subject.ProxyUserID <= 0 || subject.OwnerProxyUserID <= 0 || subject.OwnerID == "" ||
			subject.AccountGeneration < 0 || subject.PeriodID == "" || seen[subject.ProxyUserID] ||
			subject.CredentialHash == [32]byte{} || subject.OwnerCredentialHash == [32]byte{} {
			return ErrConflict
		}
		seen[subject.ProxyUserID] = true
		principals[index], periods[index] = subject.ProxyUserID, subject.PeriodID
		owners[index], ownerPrincipals[index], generations[index] = subject.OwnerID, subject.OwnerProxyUserID, subject.AccountGeneration
		credentials[index], ownerCredentials[index] = hex.EncodeToString(subject.CredentialHash[:]), hex.EncodeToString(subject.OwnerCredentialHash[:])
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Purge/rejoin locks these rows exclusively before changing native material.
	// Lock in stable order. Serializable conflicts retry the whole operation
	// so every authority check is repeated against the replacement snapshot.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE id=ANY($1::text[]) ORDER BY id FOR SHARE`, owners)
	if err != nil {
		return err
	}
	for rows.Next() {
		var owner string
		if err = rows.Scan(&owner); err != nil {
			rows.Close()
			return err
		}
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	// Serialize source-material changes with this receipt as well. Owner locks
	// fence lifecycle transitions; identity locks fence ordinary credential
	// rotation, including a child principal whose owner row is unchanged.
	rows, err = tx.QueryContext(ctx, `SELECT proxy_user_id FROM xnode_identities
 WHERE proxy_user_id=ANY($1::bigint[]) OR proxy_user_id=ANY($2::bigint[])
 ORDER BY proxy_user_id FOR UPDATE`, principals, ownerPrincipals)
	if err != nil {
		return err
	}
	for rows.Next() {
		var principal int64
		if err = rows.Scan(&principal); err != nil {
			rows.Close()
			return err
		}
	}
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `WITH proof AS (
 SELECT * FROM unnest($4::bigint[],$5::text[],$6::text[],$7::bigint[],$8::bigint[],$9::text[],$10::text[])
 AS captured(proxy_user_id,period_id,owner_id,owner_proxy_user_id,account_generation,credential_hash,owner_credential_hash)
 ), current_subjects AS (
 SELECT c.* FROM proof c
 JOIN users u ON u.id=c.owner_id AND u.account_generation=c.account_generation AND u.account_status='active'
 JOIN xnode_identities i ON i.proxy_user_id=c.proxy_user_id AND i.current_period_id=c.period_id
 JOIN xnode_periods p ON p.id=c.period_id AND p.proxy_user_id=i.proxy_user_id AND p.status='open'
 WHERE i.credentials_cipher<>'' AND i.access_disabled=0
 AND encode(sha256(convert_to(i.credentials_cipher,'UTF8')),'hex')=c.credential_hash
 AND (i.user_id=c.owner_id
 OR EXISTS(SELECT 1 FROM xnode_payg_accounts a WHERE a.proxy_user_id=i.proxy_user_id AND a.user_id=c.owner_id AND a.period_id=c.period_id)
 OR EXISTS(SELECT 1 FROM hourly_enrollments e WHERE e.proxy_user_id=i.proxy_user_id AND e.owner_id=c.owner_id AND e.period_id=c.period_id AND e.account_generation=c.account_generation))
 )
 UPDATE xnode_identities owner SET first_synced_at=$1
 WHERE owner.user_id IS NOT NULL AND owner.first_synced_at IS NULL
 AND owner.credentials_cipher<>'' AND owner.access_disabled=0
 AND EXISTS(SELECT 1 FROM xnode_nodes WHERE id=$2 AND applied_revision>=$3)
 AND EXISTS(SELECT 1 FROM current_subjects c WHERE c.owner_id=owner.user_id AND c.owner_proxy_user_id=owner.proxy_user_id
 AND encode(sha256(convert_to(owner.credentials_cipher,'UTF8')),'hex')=c.owner_credential_hash)`,
		stamp(at), node, revision, principals, periods, owners, ownerPrincipals, generations, credentials, ownerCredentials)
	if err != nil {
		return err
	}
	return tx.Commit()
}
