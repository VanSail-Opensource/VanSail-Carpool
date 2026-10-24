package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestRoutingRetryRemainsPromptWithoutChangingCommandPolicy(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	user := createTestUser(t, s, 91819)
	for _, kind := range []string{"xnode_routing", "xnode_diy", "xnode_command"} {
		t.Run(kind, func(t *testing.T) {
			op, _, err := s.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: user.ID, OwnerUserID: user.ID, Kind: kind,
				IdempotencyKey: kind, RequestFingerprint: "routing-retry-policy-" + kind, Items: []providerops.ItemInput{{Key: "scope", TargetType: "user", TargetID: user.ID}}}, now)
			if err != nil {
				t.Fatal(err)
			}
			// An eighth retry previously delayed interactive changes for 128 min.
			if _, err = s.db.ExecContext(ctx, `UPDATE outbox_jobs SET attempts=7 WHERE kind='provider_operation' AND json_extract(payload,'$.operationId')=?`, op.Receipt.ID); err != nil {
				t.Fatal(err)
			}
			job, err := s.ClaimOutboxJob(ctx, now.Add(time.Second))
			if err != nil || job == nil || job.Attempts != 8 {
				t.Fatal("claim", job, err)
			}
			var payload struct {
				OperationID string `json:"operationId"`
			}
			if err = json.Unmarshal([]byte(job.Payload), &payload); err != nil || payload.OperationID != op.Receipt.ID {
				t.Fatal("wrong operation claimed", err)
			}
			if err = s.CompleteOutboxClaim(ctx, *job, errors.New("node not ready"), now); err != nil {
				t.Fatal(err)
			}
			var available string
			if err = s.db.QueryRowContext(ctx, `SELECT available_at FROM outbox_jobs WHERE id=?`, job.ID).Scan(&available); err != nil {
				t.Fatal(err)
			}
			next, err := parseStamp(available)
			want := 30 * time.Second
			if kind == "xnode_command" {
				want = 128 * time.Minute
			}
			if err != nil || next.Sub(now) != want {
				t.Fatalf("retry delay %v, want %v: %v", next.Sub(now), want, err)
			}
			if _, err = s.db.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE id=?`, job.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}
