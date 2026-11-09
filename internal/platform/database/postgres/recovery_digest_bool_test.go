package postgres

import (
	"crypto/sha256"
	"testing"
)

func TestRecoveryDigestPreservesBooleanValuesAndTypes(t *testing.T) {
	seen := map[string]bool{}
	for _, value := range []any{true, false, nil, int64(1), int64(0), "true", "false", []byte("true"), []byte("false")} {
		h := sha256.New()
		if err := hashValue(h, value); err != nil {
			t.Fatalf("digest %T: %v", value, err)
		}
		digest := string(h.Sum(nil))
		if seen[digest] {
			t.Fatalf("boolean recovery evidence collides with %T", value)
		}
		seen[digest] = true
	}
	for _, check := range []struct {
		value bool
		frame string
	}{{true, "t4:true;"}, {false, "t5:false;"}} {
		h := sha256.New()
		if err := hashValue(h, check.value); err != nil {
			t.Fatal(err)
		}
		want := sha256.Sum256([]byte(check.frame))
		if string(h.Sum(nil)) != string(want[:]) {
			t.Fatal("boolean recovery fingerprint encoding changed")
		}
	}
}
