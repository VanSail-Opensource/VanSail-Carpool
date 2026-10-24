package datamigrate

import (
	"context"
	"fmt"
	"math/big"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func checkBalances(ctx context.Context, q queryer, report *Report) error {
	rows, err := q.QueryContext(ctx, `SELECT b.user_id,b.txb_minor,l.delta_txb_minor FROM balances b LEFT JOIN ledger_entries l ON l.user_id=b.user_id ORDER BY b.user_id,l.created_at,l.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var previous string
	var balance int64
	sum := new(big.Int)
	check := func() {
		if previous != "" && sum.Cmp(big.NewInt(balance)) != 0 {
			report.FinancialMismatches++
		}
	}
	for rows.Next() {
		var user string
		var current int64
		var delta *int64
		if err = rows.Scan(&user, &current, &delta); err != nil {
			return err
		}
		if previous != user {
			check()
			previous = user
			balance = current
			sum.SetInt64(0)
		}
		if delta != nil {
			sum.Add(sum, big.NewInt(*delta))
		}
	}
	check()
	return rows.Err()
}

// VerifyEncryption checks every known encrypted document under its unchanged
// associated-data context. Only success/failure is returned, never plaintext.
func VerifyEncryption(ctx context.Context, q queryer, key []byte) error {
	vault, err := secret.NewVault(key)
	if err != nil {
		return fmt.Errorf("a valid CONFIG_MASTER_KEY is required for verification")
	}
	queries := []string{
		`SELECT key,value FROM settings WHERE encrypted=1 AND value<>''`,
		`SELECT 'payment-profile:'||id,credential_ciphertext FROM payment_profiles WHERE credential_ciphertext<>''`,
		`SELECT 'xnode.config.'||id,document_cipher FROM xnode_configs WHERE document_cipher<>''`,
		`SELECT 'xnode.snippet.'||id,document_cipher FROM xnode_snippets WHERE document_cipher<>''`,
		`SELECT 'xnode.node.'||id,settings_cipher FROM xnode_nodes WHERE settings_cipher<>''`,
		`SELECT 'xnode.endpoint.'||id,overrides_cipher FROM xnode_endpoints WHERE overrides_cipher<>''`,
		`SELECT 'xnode.diy.'||user_id,cipher FROM xnode_member_diy WHERE cipher<>''`,
		`SELECT 'xnode.outbounds.'||COALESCE(user_id,'native.'||proxy_user_id)||'.'||node_id,outbounds_cipher FROM xnode_customizations WHERE outbounds_cipher<>''`,
		`SELECT 'xnode.routing.'||COALESCE(user_id,'native.'||proxy_user_id)||'.'||node_id,routing_cipher FROM xnode_customizations WHERE routing_cipher<>''`,
		`SELECT 'xnode.routes.'||proxy_user_id,cipher FROM xnode_routing_plans WHERE cipher<>''`,
		`SELECT 'xnode.routes.'||proxy_user_id,previous_cipher FROM xnode_routing_plans WHERE previous_cipher<>''`,
		`SELECT 'xnode.routes.legacy.'||proxy_user_id,legacy_cipher FROM xnode_routing_plans WHERE legacy_cipher<>''`,
		`SELECT 'xnode.identity.'||CASE WHEN user_id IS NULL THEN 'service.'||service_name ELSE 'user.'||user_id END,credentials_cipher FROM xnode_identities WHERE credentials_cipher<>''`,
		`SELECT 'xnode.subscription.'||CASE WHEN user_id IS NULL THEN 'service.'||service_name ELSE 'user.'||user_id END,subscription_cipher FROM xnode_identities WHERE subscription_cipher<>''`,
		`SELECT password_context,password_ciphertext FROM emby_accounts WHERE password_ciphertext<>''`,
	}
	for _, query := range queries {
		rows, e := q.QueryContext(ctx, query)
		if e != nil {
			return e
		}
		for rows.Next() {
			var context, cipher string
			if e = rows.Scan(&context, &cipher); e != nil {
				rows.Close()
				return e
			}
			if _, e = vault.Decrypt(context, cipher); e != nil {
				rows.Close()
				return fmt.Errorf("encrypted document verification failed")
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
	}
	return nil
}
