package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func prepareHourlyBillingCredit(t *testing.T, s *Store, input HourlyActivationInput, snapshot model.HourlyPolicySnapshot) model.HourlyCreditRequest {
	t.Helper()
	ctx := context.Background()
	if err := s.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "hourly-config", Name: "Billing", DocumentCipher: "opaque"}); err != nil {
		t.Fatal(err)
	}
	addHourlyCreditNode(t, s, "hourly-node", "11111111111111111111111111111111")
	day := hourlyBillingNow(t, s).Format(time.DateOnly)
	if _, err := s.EnsureHourlyCreditDay(ctx, input.OwnerID, input.AccountGeneration, day, snapshot.Policy.DailyAllowanceBytes); err != nil {
		t.Fatal(err)
	}
	return model.HourlyCreditRequest{ReservationID: "billing-free-credit", OwnerID: input.OwnerID, AccountGeneration: input.AccountGeneration,
		DayUTC: day, NodeID: "hourly-node", AgentID: "11111111111111111111111111111111", BootID: "22222222222222222222222222222222",
		SessionID: "33333333333333333333333333333333", Kind: "free", ByteCeiling: snapshot.Policy.DailyAllowanceBytes}
}

func freshHourlyFunding(t *testing.T, s *Store, input HourlyActivationInput, policy model.HourlyPolicySnapshot, day, receipt string) HourlyExtraFundingInput {
	t.Helper()
	return HourlyExtraFundingInput{ReceiptID: receipt, OwnerID: input.OwnerID, AccountGeneration: input.AccountGeneration,
		DayUTC: day, ExpectedPolicyVersion: policy.Version, PendingBytes: 1, PendingObservedAt: hourlyBillingNow(t, s)}
}

func optIntoHourlyExtras(t *testing.T, s *Store, input HourlyActivationInput, policy model.HourlyPolicySnapshot) {
	t.Helper()
	if _, err := s.SetHourlyExtraPreference(context.Background(), HourlyExtraPreferenceInput{OwnerID: input.OwnerID,
		AccountGeneration: input.AccountGeneration, Enabled: true, AcceptedPriceTXBMinor: policy.Policy.ExtraGBPriceTXBMinor}, policy, time.Now()); err != nil {
		t.Fatal(err)
	}
}
