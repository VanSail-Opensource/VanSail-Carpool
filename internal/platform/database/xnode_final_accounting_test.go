package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestFinalAccountingEvidenceSurvivesMigrationStateUpdates(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	want := xnode.MigrationState{Phase: "active", SourceCipher: "encrypted-source", SnapshotCipher: "encrypted-import", SnapshotDigest: "import-digest", SourceBackupReference: "source-backup", BackupName: "panel.db", FinalSnapshotCipher: "encrypted-final", FinalDigest: "final-digest", FinalBackupReference: "final-source-backup"}
	if err := store.SaveXNodeMigration(ctx, want); err != nil {
		t.Fatal(err)
	}
	state, err := store.XNodeMigration(ctx)
	if err != nil || state.FinalizedAt != nil || state.FinalSnapshotCipher != want.FinalSnapshotCipher || state.FinalDigest != want.FinalDigest || state.FinalBackupReference != want.FinalBackupReference {
		t.Fatalf("staged evidence=%+v,%v", state, err)
	}
	state.Phase, state.FinalizedAt = "complete", &at
	if err = store.SaveXNodeMigration(ctx, state); err != nil {
		t.Fatal(err)
	}
	state, err = store.XNodeMigration(ctx)
	if err != nil || state.Phase != "complete" || state.FinalizedAt == nil || !state.FinalizedAt.Equal(at) || state.SnapshotCipher != want.SnapshotCipher || state.SourceCipher != want.SourceCipher {
		t.Fatalf("completed evidence=%+v,%v", state, err)
	}
}
