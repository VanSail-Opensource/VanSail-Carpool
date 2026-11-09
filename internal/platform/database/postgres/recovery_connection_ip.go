package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

// ConnectionIPOwnerAliases reads bounded, exact retained migration evidence.
// It grants no account lookup, credential replacement or native subject aliasing.
func ConnectionIPOwnerAliases(ctx context.Context, q queryer, owner string) ([]secret.ConnectionIPOwnerAlias, error) {
	rows, err := q.QueryContext(ctx, `SELECT old_user_id,user_id FROM member_identity_aliases
 WHERE user_id=$1 AND old_user_id<>user_id ORDER BY old_user_id LIMIT $2`, owner, secret.MaxConnectionIPOwnerAliases+1)
	if err != nil {
		return nil, fmt.Errorf("read retained IP owner evidence: %w", err)
	}
	defer rows.Close()
	var aliases []secret.ConnectionIPOwnerAlias
	for rows.Next() {
		var alias secret.ConnectionIPOwnerAlias
		if err = rows.Scan(&alias.OldOwnerID, &alias.OwnerID); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(aliases) > secret.MaxConnectionIPOwnerAliases {
		return nil, secret.ErrConnectionIPTarget
	}
	return aliases, nil
}

func verifyRecoveryConnectionIPBlocks(ctx context.Context, q queryer, vault *secret.Vault) error {
	rows, err := q.QueryContext(ctx, `SELECT b.user_id,b.node_uuid,b.ip_digest,b.sealed_ip,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('OldOwnerID',a.old_user_id,'OwnerID',a.user_id))
 FROM (SELECT old_user_id,user_id FROM member_identity_aliases WHERE user_id=b.user_id AND old_user_id<>user_id
 ORDER BY old_user_id LIMIT $1) a),'[]'::jsonb)::text
 FROM connection_ip_blocks b WHERE b.sealed_ip<>''`, secret.MaxConnectionIPOwnerAliases+1)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var target secret.ConnectionIPTarget
		var encodedAliases string
		if err = rows.Scan(&target.OwnerID, &target.NodeID, &target.Digest, &target.Ciphertext, &encodedAliases); err != nil {
			return err
		}
		plain, openErr := vault.OpenConnectionIPTarget(ctx, target, func(context.Context, string) ([]secret.ConnectionIPOwnerAlias, error) {
			var aliases []secret.ConnectionIPOwnerAlias
			if json.Unmarshal([]byte(encodedAliases), &aliases) != nil {
				return nil, secret.ErrConnectionIPTarget
			}
			return aliases, nil
		})
		clear(plain)
		if openErr != nil {
			return fmt.Errorf("encrypted document verification failed")
		}
	}
	return rows.Err()
}
