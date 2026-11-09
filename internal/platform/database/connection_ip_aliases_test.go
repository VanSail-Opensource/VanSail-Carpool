package database

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func TestRecoveryReadsLegacyIPScopeWithoutRewritingAndRejectsTargetChanges(t *testing.T) {
	s := newTestStore(t)
	owner := createTestUser(t, s, 577230)
	other := createTestUser(t, s, 577231)
	ctx := context.Background()
	key := []byte(strings.Repeat("k", 32))
	vault, err := secret.NewVault(key)
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	sealed, err := vault.Encrypt("connection-ip-block:legacy-member:node:"+digest, "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO member_identity_aliases(old_user_id,user_id) VALUES('legacy-member',$1)`, owner.ID); err != nil {
		t.Fatal(err)
	}
	at := stamp(time.Now().UTC())
	if _, err = s.db.ExecContext(ctx, `INSERT INTO connection_ip_blocks(id,user_id,node_uuid,ip_digest,sealed_ip,status,expiry_job_id,created_at,updated_at,expires_at)
 VALUES('legacy-block',$1,'node',$2,$3,'unblocking','retained-job',$4,$4,$4)`, owner.ID, digest, sealed, at); err != nil {
		t.Fatal(err)
	}
	verify := func() error {
		tx, e := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
		if e != nil {
			return e
		}
		defer tx.Rollback()
		return postgres.VerifyEncryption(ctx, tx, key)
	}
	if err = verify(); err != nil {
		t.Fatal("restored legacy scope was not readable in one read-only transaction", err)
	}
	aliases, err := s.ConnectionIPOwnerAliases(ctx, owner.ID)
	if err != nil || len(aliases) != 1 || aliases[0].OwnerID != owner.ID {
		t.Fatal("retained owner proof unavailable", err)
	}
	for _, test := range []struct{ name, owner, node, digest string }{
		{"foreign_owner", other.ID, "node", digest}, {"changed_node", owner.ID, "another", digest}, {"changed_digest", owner.ID, "node", strings.Repeat("b", 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, e := s.db.ExecContext(ctx, `UPDATE connection_ip_blocks SET user_id=$1,node_uuid=$2,ip_digest=$3 WHERE id='legacy-block'`, test.owner, test.node, test.digest); e != nil {
				t.Fatal(e)
			}
			if e := verify(); e == nil {
				t.Fatal("changed target passed recovery")
			}
		})
	}
	var retained, status string
	if err = s.db.QueryRowContext(ctx, `SELECT sealed_ip,status FROM connection_ip_blocks WHERE id='legacy-block'`).Scan(&retained, &status); err != nil || retained != sealed || status != "unblocking" {
		t.Fatal("verification mutated retained cleanup evidence", err)
	}
}

func TestConnectionIPAliasLookupIsBoundedAndOwnerScoped(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO member_identity_aliases(old_user_id,user_id) SELECT 'legacy-'||n,'member' FROM generate_series(1,$1) n`, secret.MaxConnectionIPOwnerAliases+1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConnectionIPOwnerAliases(ctx, "member"); err == nil {
		t.Fatal("oversized alias evidence accepted")
	}
	if got, err := s.ConnectionIPOwnerAliases(ctx, "unrelated"); err != nil || len(got) != 0 {
		t.Fatal("foreign owner aliases returned", err)
	}
}
