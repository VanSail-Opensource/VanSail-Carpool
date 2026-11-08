package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Preserve original paid scopes and the agent's still-open journal; close only
// account periods as the native barrier would before final operational scrub.
func prepareUsagePurge(t *testing.T, s *Store, owner string, at time.Time) model.User {
	t.Helper()
	ctx := context.Background()
	u, err := s.UserByID(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, s, u, at)
	if _, err = s.DB().ExecContext(ctx, `UPDATE xnode_periods SET status='closed',ended_at=$2 WHERE proxy_user_id IN(`+accountNativePrincipals+`)`, owner, stamp(at)); err != nil {
		t.Fatal(err)
	}
	return u
}

func nextUsageEnrollment(t *testing.T, s *Store, u model.User, at time.Time) xnode.HourlyPrincipal {
	t.Helper()
	ctx := context.Background()
	id := fmt.Sprintf("history-generation-%d", u.AccountGeneration)
	if _, err := s.AdjustBalance(ctx, u.ID, 100, "history-rejoin-seed", "fixture", at); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at) VALUES($1,$2,$3,'history-group',$1,$4,'preparing',$5,$5)`, id, u.ID, u.AccountGeneration, strings.Repeat("b", 64), at); err != nil {
		t.Fatal(err)
	}
	p, err := s.EnsureXNodeHourlyPrincipal(ctx, id, "encrypted-new-generation")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	r, err := payHourlyIntervalTx(ctx, tx, model.HourlyEnrollment{ID: id, OwnerID: u.ID, GroupID: p.GroupID}, model.HourlyOffer{HourlyPriceTXBMinor: 1, Revision: 1}, model.HourlyPolicySnapshot{Version: strings.Repeat("b", 64)}, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE hourly_enrollments SET status='active',activated_at=$2,paid_through=$3 WHERE id=$1`, id, at, r.PaidUntil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = s.TrackXNodePeriodNode(ctx, p.PeriodID, "node"); err != nil {
		t.Fatal(err)
	}
	p, err = s.XNodeHourlyPrincipal(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func settledUsageCredit(t *testing.T, s *Store, owner string, at time.Time) model.HourlyCreditProof {
	t.Helper()
	ctx := context.Background()
	agent := strings.Repeat("c", 32)
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "usage-credit-node", Name: "Credit", ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: xnode.MultiplierScale}); err != nil {
		t.Fatal(err)
	}
	if err := s.BindXNodeAgent(ctx, "usage-credit-node", agent); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureHourlyCreditDay(ctx, owner, 0, at.Format(time.DateOnly), 1000); err != nil {
		t.Fatal(err)
	}
	r, err := s.ReserveHourlyCredit(ctx, model.HourlyCreditRequest{ReservationID: "usage-retained-credit", OwnerID: owner, AccountGeneration: 0, DayUTC: at.Format(time.DateOnly), NodeID: "usage-credit-node", AgentID: agent, BootID: strings.Repeat("d", 32), SessionID: strings.Repeat("e", 32), Kind: "free", ByteCeiling: 10})
	if err != nil {
		t.Fatal(err)
	}
	p := model.HourlyCreditProof{HourlyCreditRequest: r.HourlyCreditRequest, IssueSequence: r.IssueSequence, AdmittedUploadBytes: 3, AdmittedDownloadBytes: 4, UnusedBytes: 3}
	p.Digest, err = hourlyCreditProofDigest(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyHourlyCreditProof(ctx, p.NodeID, p.AgentID, p); err != nil {
		t.Fatal(err)
	}
	return p
}
