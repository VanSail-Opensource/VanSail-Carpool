package backup

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestUploadStreamsCapHashAndSQLiteValidation(t *testing.T) {
	service, store, ctx, migrations := newUploadTestService(t)
	user, _, err := store.UpsertTelegramUser(ctx, telegramUploadProfile(44001), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReceiveUpload(ctx, user.ID, "too-large", "x.db", strings.NewReader("12345"), 4); !errors.Is(err, ErrUploadTooLarge) {
		t.Fatalf("oversized upload error = %v, want ErrUploadTooLarge", err)
	}

	source, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	file, err := os.Open(source.Path)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := service.ReceiveUpload(ctx, user.ID, "valid-upload", "source.db", file, source.SizeBytes+1)
	_ = file.Close()
	if err != nil {
		t.Fatalf("ReceiveUpload(valid): %v", err)
	}
	run, err := service.FinalizeUpload(ctx, candidate.ID, candidate.ActualSHA256, migrations)
	if err != nil || run.Status != "complete" {
		t.Fatalf("FinalizeUpload(valid) = %+v, %v", run, err)
	}
	var sourceName, sha string
	if err := store.DB().QueryRowContext(ctx, `SELECT source,sha256 FROM backup_runs WHERE id=?`, run.ID).Scan(&sourceName, &sha); err != nil {
		t.Fatal(err)
	}
	if sourceName != "upload" || sha != candidate.ActualSHA256 {
		t.Fatalf("uploaded metadata = source %q hash %q", sourceName, sha)
	}

	badHashSource, err := os.Open(source.Path)
	if err != nil {
		t.Fatal(err)
	}
	badHash, err := service.ReceiveUpload(ctx, user.ID, "bad-hash", "source.db", badHashSource, source.SizeBytes+1)
	_ = badHashSource.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.FinalizeUpload(ctx, badHash.ID, strings.Repeat("0", 64), migrations); !errors.Is(err, ErrUploadHashMismatch) {
		t.Fatalf("hash mismatch error = %v, want ErrUploadHashMismatch", err)
	}

	invalid, err := service.ReceiveUpload(ctx, user.ID, "invalid-schema", "bad.db", strings.NewReader("not sqlite"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.FinalizeUpload(ctx, invalid.ID, "", migrations); err == nil {
		t.Fatal("invalid SQLite upload unexpectedly finalized")
	}
}

func telegramUploadProfile(id int64) model.TelegramProfile {
	return model.TelegramProfile{ID: id, FirstName: "Upload", Username: "upload" + strconv.FormatInt(id, 10)}
}
