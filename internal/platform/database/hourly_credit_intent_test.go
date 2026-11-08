package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyIntentAbsenceRequiresPermanentAdmissionFence(t *testing.T) {
	for _, fence := range []string{"active", "disabled", "removing", "generation", "expired"} {
		t.Run(fence, func(t *testing.T) {
			s, request := hourlyCreditFixture(t)
			ctx := context.Background()
			var err error
			switch fence {
			case "disabled":
				_, err = s.DB().ExecContext(ctx, `UPDATE xnode_nodes SET enabled=0 WHERE id=$1`, request.NodeID)
			case "removing":
				_, err = s.DB().ExecContext(ctx, `UPDATE xnode_nodes SET enabled=0,removing_at=$2 WHERE id=$1`, request.NodeID, stamp(time.Now()))
			case "generation":
				_, err = s.DB().ExecContext(ctx, `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, request.OwnerID)
			case "expired":
				request.DayUTC = "2020-01-01"
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.ResolveHourlyCreditIntent(ctx, request)
			expected := model.ErrHourlyCreditNotFound
			if fence == "active" || fence == "disabled" {
				expected = model.ErrHourlyCreditUnavailable
			}
			if !errors.Is(err, expected) {
				t.Fatal("missing intent resolved without the required fence", err)
			}
			if day := readHourlyDayFixture(t, s, model.HourlyCreditRequest{OwnerID: request.OwnerID, AccountGeneration: 0, DayUTC: time.Now().UTC().Format(time.DateOnly)}); day.FreeReservedBytes != 0 {
				t.Fatal("absence lookup minted/returned credit", day)
			}
		})
	}
}

func TestHourlyIntentReturnsCommittedOriginalAfterLifecycleCloses(t *testing.T) {
	s, request := hourlyCreditFixture(t)
	r := reserveHourlyFixture(t, s, request)
	ctx := context.Background()
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=1 WHERE id=$1`, request.OwnerID); err != nil {
		t.Fatal(err)
	}
	stored, err := s.ResolveHourlyCreditIntent(ctx, request)
	if err != nil || stored.HourlyCreditRequest != r.HourlyCreditRequest || stored.IssueSequence != r.IssueSequence {
		t.Fatal("closure hid an uncertain committed obligation", stored, err)
	}
	request.ByteCeiling--
	if _, err = s.ResolveHourlyCreditIntent(ctx, request); !errors.Is(err, ErrConflict) {
		t.Fatal("altered stable intent accepted", err)
	}
}
