package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// PrepareXNodePAYGAccount publishes no orphan credentials: every new principal,
// empty period and account/participant reference commits under one owner fence.
func (s *Store) PrepareXNodePAYGAccount(ctx context.Context, input xnode.PAYGPreparation) (xnode.PAYGAccount, error) {
	if input.OwnerID == "" || input.GroupID == "" || input.NodeID == "" || input.AccountGeneration < 0 || input.UnitPriceMinor < 0 || input.CredentialsCipher == "" || input.PeriodID == "" || input.ServiceName != xnode.PAYGAccountServiceName(input.OwnerID, input.GroupID, input.NodeID) {
		return xnode.PAYGAccount{}, xnode.ErrInvalid
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (xnode.PAYGAccount, error) { return prepareXNodePAYGAccountTx(ctx, tx, input) })
}

func prepareXNodePAYGAccountTx(ctx context.Context, tx *sql.Tx, input xnode.PAYGPreparation) (xnode.PAYGAccount, error) {
	// Serialize competing first preparations for this owner, before checking the
	// unique account/principal keys. Existing accounts use the read-only fast path.
	if err := lockNativeMemberIdentityTx(ctx, tx, input.OwnerID, input.AccountGeneration, true, true); err != nil {
		return xnode.PAYGAccount{}, err
	}
	var admitting bool
	if err := tx.QueryRowContext(ctx, `SELECT removing_at IS NULL AND retired_at IS NULL FROM xnode_nodes WHERE id=$1 FOR KEY SHARE`, input.NodeID).Scan(&admitting); err != nil {
		return xnode.PAYGAccount{}, err
	}
	if !admitting {
		return xnode.PAYGAccount{}, xnode.ErrConflict
	}
	account, err := scanPAYG(tx.QueryRowContext(ctx, paygSelect+` WHERE a.user_id=$1 AND a.group_id=$2 AND a.node_id=$3 FOR UPDATE OF a`, input.OwnerID, input.GroupID, input.NodeID))
	if err == nil {
		account.AccountGeneration = input.AccountGeneration
		return account, nil
	}
	if !errors.Is(err, xnode.ErrNotFound) {
		return xnode.PAYGAccount{}, err
	}
	identity, err := scanXNodeIdentity(tx.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.service_name=$1 FOR UPDATE OF p`, input.ServiceName))
	if err == nil {
		if err = validateUnpublishedPAYGIdentityTx(ctx, tx, identity); err != nil {
			return xnode.PAYGAccount{}, err
		}
	} else if errors.Is(err, xnode.ErrNotFound) {
		identity = xnode.Identity{ServiceName: input.ServiceName}
		if err = tx.QueryRowContext(ctx, `SELECT nextval('xnode_service_proxy_id_seq')`).Scan(&identity.ProxyUserID); err != nil {
			return xnode.PAYGAccount{}, err
		}
		if identity.ProxyUserID <= 0 || identity.ProxyUserID >= 1<<53 {
			return xnode.PAYGAccount{}, xnode.ErrConflict
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_identities(proxy_user_id,service_name,credentials_cipher,created_at) VALUES($1,$2,'',$3)`, identity.ProxyUserID, input.ServiceName, stamp(time.Now().UTC()))
		if err != nil {
			return xnode.PAYGAccount{}, err
		}
	} else {
		return xnode.PAYGAccount{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,status) VALUES($1,$2,$3,0,'PAYG','open')`, input.PeriodID, identity.ProxyUserID, stamp(time.Now().UTC())); err != nil {
		return xnode.PAYGAccount{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher=$2,current_period_id=$3,access_disabled=0 WHERE proxy_user_id=$1`, identity.ProxyUserID, input.CredentialsCipher, input.PeriodID); err != nil {
		return xnode.PAYGAccount{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_payg_accounts(proxy_user_id,user_id,group_id,node_id,period_id,unit_price_minor) VALUES($1,$2,$3,$4,$5,$6)`, identity.ProxyUserID, input.OwnerID, input.GroupID, input.NodeID, input.PeriodID, input.UnitPriceMinor); err != nil {
		return xnode.PAYGAccount{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_period_nodes(period_id,node_id) VALUES($1,$2)`, input.PeriodID, input.NodeID); err != nil {
		return xnode.PAYGAccount{}, err
	}
	account, err = scanPAYG(tx.QueryRowContext(ctx, paygSelect+` WHERE a.proxy_user_id=$1`, identity.ProxyUserID))
	account.AccountGeneration = input.AccountGeneration
	return account, err
}
