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
		{"nanosecond-start", whole, whole.Add(time.Nanosecond), later, whole.Add(time.Nanosecond)},
		{"nanosecond-end", whole, whole.Add(-time.Hour), whole.Add(time.Nanosecond), whole.Add(time.Nanosecond)},
		{"nanosecond-before-end", whole.Add(599 * time.Nanosecond), whole.Add(-time.Hour), whole.Add(600 * time.Nanosecond), whole.Add(600 * time.Nanosecond)},
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
	// A preserved noncanonical fractional width represents the same instant.
	// It must not hide the later boundary or be returned repeatedly as future.
	if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id='compile-boundary'`, "2030-01-02T03:04:05.500000000Z", stamp(later)); err != nil {
		t.Fatal(err)
	}
	_, boundary, err := s.XNodeCompileVersion(ctx, half)
	if err != nil || !boundary.Equal(later) {
		t.Fatal("equivalent precision hid the next actual boundary", boundary, err)
	}
}
