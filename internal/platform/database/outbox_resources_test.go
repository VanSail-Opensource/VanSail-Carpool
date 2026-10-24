package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestOutboxSkipsOccupiedAccountAcrossKinds(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, row := range []struct{ id, kind, payload string }{
		{"z-first", "xnode_sync_user", `{"userId":"account-a"}`},
		{"a-second", "xnode_apply_entitlement", `{"userId":"account-a"}`},
		{"m-third", "xnode_sync_user", `{"userId":"account-b"}`},
	} {
		_, err := s.db.ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,attempts,available_at,created_at,updated_at) VALUES(?,?,?,'pending',0,?,?,?)`, row.id, row.kind, row.payload, stamp(now), stamp(now), stamp(now))
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || first == nil || first.ID != "z-first" {
		t.Fatalf("first=%+v %v", first, err)
	}
	second, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || second == nil || second.ID != "m-third" {
		t.Fatalf("independent=%+v %v", second, err)
	}
	blocked, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || blocked != nil {
		t.Fatalf("account overlapped=%+v %v", blocked, err)
	}
	if err := s.CompleteOutboxClaim(ctx, *first, nil, now); err != nil {
		t.Fatal(err)
	}
	next, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || next == nil || next.ID != "a-second" {
		t.Fatalf("successor=%+v %v", next, err)
	}
}

func TestStandaloneNativeTargetDoesNotBlockOutboxAdmission(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	actor := createTestUser(t, s, 91723)
	identity, err := s.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: "standalone", CredentialsCipher: "encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"first", "second"} {
		_, _, err := s.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor.ID, Kind: xnode.RoutingOperationKind,
			IdempotencyKey: key, RequestFingerprint: "standalone-admission-" + key, Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(identity.ProxyUserID)}}}, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || first == nil {
		t.Fatal("standalone admission failed", first, err)
	}
	next, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || next != nil {
		t.Fatal("standalone target overlapped", next, err)
	}
	if err := s.CompleteOutboxClaim(ctx, *first, nil, now); err != nil {
		t.Fatal(err)
	}
	next, err = s.ClaimOutboxJob(ctx, now)
	if err != nil || next == nil {
		t.Fatal("standalone successor blocked", next, err)
	}
}
