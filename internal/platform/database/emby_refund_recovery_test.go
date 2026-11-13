package database

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	remote "github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/emby"
)

func TestEmbyPolicyRejectionRetainsDebitForEnabledRemoteAccount(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 98_911)
	now := time.Now().UTC()
	if _, err := store.AdjustBalance(ctx, owner.ID, 1000, "seed", "test", now); err != nil {
		t.Fatal(err)
	}
	account, _, err := store.QueueEmbySetup(ctx, domain.QueueSetupInput{
		ID: "policy-review", UserID: owner.ID, BaseUsername: owner.AccountName(), SetupPriceTXBMinor: 275,
		PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:" + owner.ID,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	state := &embyRefundHTTPState{rejectPolicy: true}
	server := httptest.NewServer(state)
	defer server.Close()
	client, err := remote.NewClient(server.URL, "test-administrator-key", remote.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	service := domain.NewService(store, client, nil, embyRefundSecrets{})
	if err := service.Provision(ctx, account.ID); !errors.Is(err, domain.ErrPendingReview) {
		t.Fatalf("unrestricted remote account received a refund: %v", err)
	}
	record, err := store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil || record.Status != domain.StatusPendingReview || record.RefundedAt != nil ||
		record.RemoteUserID != "0123456789abcdef0123456789abcdef" || record.PasswordCiphertext != "sealed" {
		t.Fatalf("post-create failure lost paid recovery: %+v, %v", record.Account, err)
	}
	state.mu.Lock()
	retainedCredentials := state.password == "member-password" && state.user["Id"] == "0123456789abcdef0123456789abcdef" &&
		string(state.policy["IsDisabled"]) == "false" && string(state.policy["EnableMediaPlayback"]) == "true"
	state.rejectPolicy = false
	state.mu.Unlock()
	if !retainedCredentials {
		t.Fatal("fixture must retain an enabled remote account with the member's accepted password")
	}
	assertBalance(t, store, owner.ID, "725")
	if _, err := store.RetryEmbyProvisioning(ctx, account.ID, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := service.Provision(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	record, err = store.EmbyProvisioningByID(ctx, account.ID)
	if err != nil || record.Status != domain.StatusActive || record.PasswordCiphertext != "" || record.RefundedAt != nil {
		t.Fatalf("reviewed retry did not recover the paid account: %+v, %v", record.Account, err)
	}
	assertBalance(t, store, owner.ID, "725")
	var refunds int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND kind='emby_setup_refund'`, owner.ID).Scan(&refunds); err != nil || refunds != 0 {
		t.Fatalf("setup created refund evidence: count=%d error=%v", refunds, err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.creates != 1 {
		t.Fatalf("reviewed retry created %d remote accounts", state.creates)
	}
}

type embyRefundSecrets struct{}

func (embyRefundSecrets) Seal(string, []byte) (string, error) { return "sealed", nil }
func (embyRefundSecrets) Open(string, string) ([]byte, error) {
	return []byte("member-password"), nil
}

type embyRefundHTTPState struct {
	mu           sync.Mutex
	user         map[string]any
	policy       domain.Policy
	password     string
	creates      int
	rejectPolicy bool
}

func (s *embyRefundHTTPState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.Method + " " + r.URL.Path {
	case "GET /Users":
		users := []map[string]any{}
		if s.user != nil {
			users = append(users, s.user)
		}
		_ = json.NewEncoder(w).Encode(users)
	case "POST /Users/New":
		var input struct{ Name string }
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			http.Error(w, "invalid input", http.StatusBadRequest)
			return
		}
		s.creates++
		s.policy = domain.Policy{"IsDisabled": json.RawMessage("false"), "EnableMediaPlayback": json.RawMessage("true")}
		s.user = map[string]any{"Id": "0123456789abcdef0123456789abcdef", "Name": input.Name, "Policy": s.policy}
		_ = json.NewEncoder(w).Encode(s.user)
	case "GET /Users/0123456789abcdef0123456789abcdef":
		_ = json.NewEncoder(w).Encode(s.user)
	case "POST /Users/0123456789abcdef0123456789abcdef/Password":
		var input struct {
			NewPassword string `json:"NewPw"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			http.Error(w, "invalid input", http.StatusBadRequest)
			return
		}
		s.password = input.NewPassword
		w.WriteHeader(http.StatusNoContent)
	case "POST /Users/0123456789abcdef0123456789abcdef/Policy":
		if s.rejectPolicy {
			http.Error(w, "policy update forbidden", http.StatusForbidden)
			return
		}
		if json.NewDecoder(r.Body).Decode(&s.policy) != nil {
			http.Error(w, "invalid input", http.StatusBadRequest)
			return
		}
		s.user["Policy"] = s.policy
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}
