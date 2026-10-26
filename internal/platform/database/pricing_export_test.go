package database

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func TestPricingExportRetainedRegistryHashesPrivacyAndCancellation(t *testing.T) {
	s, parent, _, _ := paygBillingFixture(t)
	ctx := context.Background()
	secret := "private-xray-material-never-export"
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher=?`, secret); err != nil {
		t.Fatal(err)
	}
	scope := PricingExportScope{Format: "jsonl", IncludeUsers: true, IncludeFinancial: true, Pseudonymize: true, Users: []string{parent.UserID}}
	var buffer bytes.Buffer
	if err := s.StreamPricingExport(ctx, &buffer, scope); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*zip.File{}
	for _, file := range archive.File {
		files[file.Name] = file
	}
	manifestFile := files["bapo-export/manifest.json"]
	if manifestFile == nil {
		t.Fatal("no manifest")
	}
	r, err := manifestFile.Open()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"`+parent.UserID+`"`) {
		t.Fatal("manifest exposed the pseudonym reverse mapping")
	}
	var manifest struct{ Files []pricingExportFile }
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != len(pricing.Datasets()) {
		t.Fatalf("retained registry incomplete: %d/%d", len(manifest.Files), len(pricing.Datasets()))
	}
	for _, meta := range manifest.Files {
		file := files[meta.Name]
		if file == nil {
			t.Fatal(meta.Name)
		}
		r, e := file.Open()
		if e != nil {
			t.Fatal(e)
		}
		compressed, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256(compressed)
		if hex.EncodeToString(h[:]) != meta.SHA256 {
			t.Fatal("checksum", meta.Name)
		}
		gz, e := gzip.NewReader(bytes.NewReader(compressed))
		if e != nil {
			t.Fatal(e)
		}
		body, e := io.ReadAll(gz)
		gz.Close()
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(body), secret) || strings.Contains(string(body), `"user_id":"`+parent.UserID+`"`) {
			t.Fatal("private field leaked", meta.Name)
		}
		var rows int64
		decoder := json.NewDecoder(bytes.NewReader(body))
		for {
			var row map[string]any
			e = decoder.Decode(&row)
			if errors.Is(e, io.EOF) {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			rows++
		}
		if rows != meta.Rows {
			t.Fatal("row count", meta.Name, rows, meta.Rows)
		}
	}
	blocked, cancel := context.WithCancel(ctx)
	writer := &pricingBlockedWriter{ctx: blocked, entered: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- s.StreamPricingExport(blocked, writer, scope) }()
	<-writer.entered
	if err = s.StreamPricingExport(ctx, io.Discard, scope); !errors.Is(err, pricing.ExportResourceLimit) {
		t.Fatal("unbounded parallel exports", err)
	}
	cancel()
	if err = <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = s.StreamPricingExport(ctx, io.Discard, scope); err != nil {
		t.Fatal("cancellation leaked snapshot/slot", err)
	}
}

type pricingBlockedWriter struct {
	ctx     context.Context
	entered chan struct{}
	once    sync.Once
}

func (w *pricingBlockedWriter) Write([]byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.ctx.Done()
	return 0, w.ctx.Err()
}
