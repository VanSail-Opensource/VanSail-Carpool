package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseFileURIKeepsLiteralReservedCharactersAndIsolation(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"panel #first%.db", "panel #second%.db"} {
		path := filepath.Join(dir, name)
		testDatabaseOpenGate <- struct{}{}
		ctx, cancel := context.WithTimeout(context.Background(), testDatabaseOpenTimeout)
		db, err := Open(ctx, path)
		cancel()
		<-testDatabaseOpenGate
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		// The same identity must be new in each literal file, never a URI
		// fragment-truncated shared database. Check the real file location too.
		createTestUser(t, NewStore(db), 69020)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("database was not created at its literal path: %v", err)
		}
	}
}
