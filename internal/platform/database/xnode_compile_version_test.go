package database

import (
	"context"
	"testing"
	"time"
)

func TestCompileBoundaryComparesInstantsAcrossFractionalSeconds(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, s, 80409)
	combo := saveTestCombo(t, s, "Boundary", 0, 30)
	whole := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	half := whole.Add(500 * time.Millisecond)
	later := whole.Add(time.Hour)
	for _, tc := range []struct {
		name                   string
		now, from, until, want time.Time
	}{
		{"fractional-start", whole, half, later, half},
		{"fractional-end", whole, whole.Add(-time.Hour), half, half},
		{"past-whole-second", half, whole, later, later},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.db.ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,status,created_at,updated_at)
VALUES('compile-boundary',$1,$2,0,$3,$4,'active',$5,$5)
ON CONFLICT(id) DO UPDATE SET valid_from=excluded.valid_from,valid_until=excluded.valid_until`,
				user.ID, combo.ID, stamp(tc.from), stamp(tc.until), stamp(whole))
			if err != nil {
				t.Fatal(err)
			}
			_, boundary, err := s.XNodeCompileVersion(ctx, tc.now)
			if err != nil || !boundary.Equal(tc.want) {
				t.Fatalf("next boundary=%s, want=%s, error=%v", boundary, tc.want, err)
			}
		})
	}
}
