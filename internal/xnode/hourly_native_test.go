package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestHourlyNativePreparationRenewalAndExportKeepExactEnrollmentScope(t *testing.T) {
	f := hourlyNativeFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := f.store.DB().ExecContext(ctx, `DELETE FROM xnode_identities WHERE user_id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.HourlySquadReadiness(ctx, f.group.ID, f.user.ID); err != nil {
		t.Fatal("first-time hourly owner required a legacy routing identity", err)
	}
	if _, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Hourly", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p := seedNativeHourlyPreparation(t, f)
	replayed, err := f.service.PrepareHourlyPrincipal(ctx, p.EnrollmentID)
	if err != nil || replayed.ProxyUserID != p.ProxyUserID || replayed.PeriodID != p.PeriodID {
		t.Fatal("preparation replaced its stable native tuple", replayed, err)
	}
	identity, err := f.store.XNodeIdentity(ctx, p.ProxyUserID)
	if err != nil || identity.MemberOwnerID() != f.user.ID || len(identity.SubscriptionHash) != 0 || identity.SubscriptionCipher != "" {
		t.Fatal("secondary hourly principal lost owner/privacy scope", identity, err)
	}
	agent := connectHourlyNativeAgent(t, f)
	waitHourlyNativeReadiness(t, ctx, f)
	if ready, err := f.service.PreflightHourlyEnrollment(ctx, p.EnrollmentID); err != nil || !ready.Ready {
		t.Fatal("dormant native credential preflight failed", ready, err)
	}
	agent.mu.Lock()
	unpaid := len(agent.config.Users)
	agent.mu.Unlock()
	if unpaid != 0 {
		t.Fatal("preflight dispatched an unpaid hourly user")
	}
	activated := time.Now().UTC().Truncate(time.Microsecond)
	paid := activated.Truncate(time.Hour).Add(time.Hour)
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE hourly_enrollments SET status='active',activated_at=$1,paid_through=$2 WHERE id=$3`, activated, paid, p.EnrollmentID); err != nil {
		t.Fatal(err)
	}
	var uuid string
	for iteration := range 2 {
		if iteration > 0 {
			paid = paid.Add(time.Hour)
			if _, err := f.store.DB().ExecContext(ctx, `UPDATE hourly_enrollments SET paid_through=$1 WHERE id=$2`, paid, p.EnrollmentID); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.service.SyncHourlyEnrollment(ctx, p.EnrollmentID); err != nil {
			t.Fatal("paid enrollment was not applied exactly", err)
		}
		compiled, err := f.service.Compile(ctx, f.node.ID)
		if err != nil || len(compiled.Users) != 1 {
			t.Fatal(compiled.Users, err)
		}
		user := compiled.Users[0]
		if user.Hourly == nil || user.Hourly.EnrollmentID != p.EnrollmentID || user.Hourly.Owner.ID != f.user.ID || user.AccountingPeriod != p.PeriodID || user.ValidUntil == nil || !user.ValidUntil.Equal(paid) || iteration > 0 && user.UUID != uuid {
			t.Fatal("renewal reset scope or padded paid access", user)
		}
		uuid = user.UUID
	}
	proxies, err := f.service.MemberProxies(ctx, f.user.ID)
	if err != nil || len(proxies) != 1 || proxies[0].BillingMode != "hourly" || proxies[0].PrincipalID != p.ProxyUserID {
		t.Fatal("hourly endpoint disappeared or shared billing identity", proxies, err)
	}
	exported, err := f.service.ExportProxiesForUser(ctx, f.user.ID, proxies)
	if err != nil || len(exported) != 1 {
		t.Fatal("zero wallet hid already prepaid hourly subscription", exported, err)
	}
	metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
	if err != nil || !metadata.Ready || metadata.LimitBytes != "0" || metadata.ExpiresAt == nil || !metadata.ExpiresAt.Equal(paid) {
		t.Fatal("hourly subscription showed a PAYG quota/expiry", metadata, err)
	}
	if _, err = f.service.GrantOwnedConnection(ctx, f.user.ID, "1.1.1.1"); err != nil {
		t.Fatal("paid hourly benefit required a positive traffic wallet for IP grant", err)
	}
	connections, err := f.service.OwnedConnections(ctx, f.user.ID, "", 1, 25)
	if err != nil || !connections.CanGrant {
		t.Fatal("paid hourly connection controls remained locked", connections, err)
	}
	metrics, err := f.master.UserNodeMetrics(ctx, f.user.ID)
	if err != nil || len(metrics) != 1 || metrics[0].NodeID != f.node.ID {
		t.Fatal("hourly node disappeared from member metrics", metrics, err)
	}
	if _, err = f.service.SaveOwnedOutbounds(ctx, f.user.ID, f.node.ID, 0, json.RawMessage(`[]`)); err != nil {
		t.Fatal("hourly-only owner could not save existing customization", err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE hourly_enrollments SET status='disabled',disabled_at=clock_timestamp() WHERE id=$1`, p.EnrollmentID); err != nil {
		t.Fatal(err)
	}
	if err = f.service.SyncHourlyEnrollment(ctx, p.EnrollmentID); err != nil {
		t.Fatal("disable did not confirm native removal", err)
	}
	agent.mu.Lock()
	remaining := len(agent.config.Users)
	agent.mu.Unlock()
	if remaining != 0 {
		t.Fatal("disabled squad retained native authorization")
	}
	if _, err = f.service.GrantOwnedConnection(ctx, f.user.ID, "8.8.8.8"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("disabled hourly benefit retained manual grant eligibility", err)
	}
}
