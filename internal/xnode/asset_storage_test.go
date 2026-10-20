package xnode

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeoSpoolStorageFailureIsSafeUnavailable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing", "token-secret")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, missing)
	}
	err := geoRecords(context.Background(), io.LimitReader(strings.NewReader("asset"), 100), func(io.ReaderAt, int64, int64) error { t.Fatal("storage failure reached parser"); return nil })
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrInvalid) || strings.Contains(err.Error(), missing) {
		t.Fatal(err)
	}
	wrapped := geoIPPolicyFailure(err)
	if got := SafeOperationCause(wrapped); got != "asset scratch storage is unavailable" {
		t.Fatal(got)
	}
	transfer := assetTransferFailure("GeoIP asset", &os.PathError{Op: "write", Path: "secret", Err: os.ErrPermission})
	if !errors.Is(transfer, ErrUnavailable) || strings.Contains(transfer.Error(), "secret") {
		t.Fatal(transfer)
	}
}
