package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestUpdateReceiptRequiresMatchingTransactionAndVersions(t *testing.T) {
	expected := UpdateExpectation{"update.install", "operation-id", "v2.0.18", "v2.0.17"}
	valid := UpdateTransaction{"operation-id", "v2.0.18", "v2.0.17", "confirmed", time.Now()}
	if !expected.matches(valid) {
		t.Fatal("matching receipt rejected")
	}
	valid.Status = "pending"
	if expected.confirmed(valid) {
		t.Fatal("pending install was confirmed")
	}
	valid.Status = "confirmed"
	for _, mutate := range []func(*UpdateTransaction){
		func(t *UpdateTransaction) { t.ID = "another-operation" },
		func(t *UpdateTransaction) { t.Version = "v2.0.19" },
		func(t *UpdateTransaction) { t.Previous = "v2.0.16" },
		func(t *UpdateTransaction) { t.Deadline = time.Time{} },
	} {
		changed := valid
		mutate(&changed)
		if expected.matches(changed) {
			t.Fatal("unrelated transaction accepted", changed)
		}
	}
	expected.Command = "update.rollback"
	if expected.runningVersion() != "v2.0.17" || expected.confirmed(valid) {
		t.Fatal("rollback expected the install version")
	}
	valid.Status = "rolled_back"
	if !expected.confirmed(valid) {
		t.Fatal("matching rollback receipt rejected")
	}
}

func TestRecoveredUpdateWithoutOriginalIntentCannotMutate(t *testing.T) {
	service := &Service{} // Any native dispatch would dereference its absent master.
	operation := providerops.Operation{Attempts: 2, ResultJSON: "{}"}
	_, err := service.executeUpdate(context.Background(), "node", operation, CommandRequest{Name: "update.install"}, nil)
	if !errors.Is(err, ErrUpdateEvidence) {
		t.Fatal("legacy interrupted mutation was repeated", err)
	}
}

func TestPreparedUpdateReusesItsOriginalResolvedRelease(t *testing.T) {
	expected := UpdateExpectation{"update.install", "operation-id", "v2.0.18", "v2.0.17"}
	raw, err := json.Marshal(map[string]any{"updateExpectation": expected})
	if err != nil {
		t.Fatal(err)
	}
	operation := providerops.Operation{Attempts: 1, ResultJSON: string(raw)}
	operation.Receipt.ID = expected.TransactionID
	service := &Service{} // Recovery must not perform another release lookup.
	args := map[string]any{}
	_, err = service.prepareUpdate(context.Background(), "node", operation, CommandRequest{Name: "update.install"}, args)
	if err != nil || args["version"] != expected.Version {
		t.Fatal("resolved target changed after restart", args, err)
	}
	_, err = service.prepareUpdate(context.Background(), "node", operation, CommandRequest{Name: "update.install", Version: "v2.0.19"}, args)
	if !errors.Is(err, ErrUpdateEvidence) {
		t.Fatal("request/intent mismatch accepted", err)
	}
}

func TestUpdateResultRetainsIntentAndSanitizesNativePaths(t *testing.T) {
	expected := UpdateExpectation{"update.install", "operation-id", "v2.0.18", "v2.0.17"}
	intent, _ := json.Marshal(map[string]any{"updateExpectation": expected})
	raw := json.RawMessage(`{"id":"another-operation","version":"v2.0.18","previous":"v2.0.17","status":"confirmed","deadline":"2026-11-08T12:00:00Z","node_path":"/private/node","cli_path":"/private/cli"}`)
	result, err := updateOperationResult(providerops.Operation{ResultJSON: string(intent)}, CommandRequest{Name: "update.install"}, "node", raw)
	if err != nil || strings.Contains(result, "/private") || !strings.Contains(result, "another-operation") {
		t.Fatal("receipt projection invalid", result, err)
	}
	retained, ok := UpdateExpectationFromResult(result)
	if !ok || retained != expected {
		t.Fatal("original intent lost on an uncertain outcome")
	}
	legacy, err := updateOperationResult(providerops.Operation{ResultJSON: `{"legacyNote":"retain this evidence"}`}, CommandRequest{Name: "update.install"}, "node", nil)
	if err != nil || !strings.Contains(legacy, "retain this evidence") {
		t.Fatal("legacy uncertainty evidence was discarded", err)
	}
	replayed, err := updateOperationResult(providerops.Operation{ResultJSON: legacy}, CommandRequest{Name: "update.install"}, "node", nil)
	if err != nil || len(replayed) != len(legacy) {
		t.Fatal("replay duplicated legacy evidence", err)
	}
}
