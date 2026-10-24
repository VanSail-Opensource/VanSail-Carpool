package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"testing"
	"time"
)

func TestNodeOperationReplayFencesTheInterruptedAttempt(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	actor := createTestUser(t, store, 80491)
	at := time.Now().UTC()
	op, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: actor.ID, Kind: "xnode_command", IdempotencyKey: "handover", RequestFingerprint: "handover-request-v1", SealedTarget: "encrypted", Items: []providerops.ItemInput{{Key: "node", TargetType: "xnode", TargetID: "node"}}}, at)
	if err != nil {
		t.Fatal(err)
	}
	job, err := store.ClaimOutboxJob(ctx, at.Add(time.Second))
	if err != nil || job == nil {
		t.Fatalf("claim=%v,%v", job, err)
	}
	first, err := store.BeginProviderOperationAttempt(ctx, op.Receipt.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RecoverOutbox(ctx, at.Add(2*time.Second), at.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.ClaimOutboxJob(ctx, at.Add(4*time.Second))
	if err != nil || recovered == nil || recovered.Attempts != job.Attempts+1 {
		t.Fatalf("recovered=%+v,%v", recovered, err)
	}
	second, err := store.BeginProviderOperationAttempt(ctx, op.Receipt.ID, at.Add(4*time.Second))
	if err != nil || second.Attempts != first.Attempts+1 {
		t.Fatalf("replay=%+v,%v", second, err)
	}
	if _, err = store.CompleteXNodeProviderOperation(ctx, op.Receipt.ID, first.Attempts, providerops.Completion{Status: providerops.StatusSucceeded}, at.Add(5*time.Second)); err == nil {
		t.Fatal("interrupted callback overwrote the recovered operation")
	}
	if err = store.CompleteOutboxClaim(ctx, *job, nil, at.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	jobs, err := store.ListOutboxJobs(ctx, 10)
	if err != nil || len(jobs) != 1 || jobs[0].Status != "processing" {
		t.Fatal("old attempt removed the recovered lease")
	}
	if owned, err := store.RenewOutboxClaim(ctx, *job, at); err != nil || owned {
		t.Fatal("old attempt renewed a recovered lease")
	}
	if _, err = store.CompleteXNodeProviderOperation(ctx, op.Receipt.ID, second.Attempts, providerops.Completion{Status: providerops.StatusSucceeded}, at.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = store.CompleteOutboxClaim(ctx, *recovered, nil, at.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
}
