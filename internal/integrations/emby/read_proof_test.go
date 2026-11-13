package emby

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

func TestRequiredEmbyReadPayloadCannotBeAbsent(t *testing.T) {
	for _, payload := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "empty", status: http.StatusOK},
		{name: "null", status: http.StatusOK, body: "null"},
		{name: "no content", status: http.StatusNoContent},
		{name: "valid empty array", status: http.StatusOK, body: "[]"},
	} {
		t.Run(payload.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(payload.status)
				_, _ = w.Write([]byte(payload.body))
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "test-token")
			if err != nil {
				t.Fatal(err)
			}
			calls := []func() error{
				func() error {
					_, _, err := client.FindUserByName(context.Background(), "ada")
					return err
				},
				func() error {
					_, err := client.ListSelectableFolders(context.Background())
					return err
				},
				func() error {
					_, err := client.ListParentalRatings(context.Background())
					return err
				},
			}
			for _, call := range calls {
				err := call()
				if payload.body == "[]" {
					if err != nil {
						t.Fatal("valid empty collection was rejected", err)
					}
				} else if err == nil || client.IsNotFound(err) || client.IsTerminal(err) {
					t.Fatalf("missing data proved absence or terminal failure: %v", err)
				}
			}
			const id = "11111111-1111-4111-8111-111111111111"
			if err := client.DeleteUser(context.Background(), id); err != nil {
				t.Fatal("successful bodyless mutation was rejected", err)
			}
		})
	}
}

type missingInventoryRepository struct {
	domain.Repository
	record  domain.ProvisioningRecord
	refunds int
	reviews int
}

func (r *missingInventoryRepository) BeginEmbyProvisioning(context.Context, string, time.Time) (domain.ProvisioningRecord, error) {
	return r.record, nil
}

func (r *missingInventoryRepository) EmbyProvisioningByID(context.Context, string) (domain.ProvisioningRecord, error) {
	return r.record, nil
}

func (r *missingInventoryRepository) MarkEmbyPendingReview(context.Context, string, error, time.Time) error {
	r.reviews++
	return nil
}

func (r *missingInventoryRepository) FailAndRefundEmbySetup(context.Context, string, string, time.Time) (domain.Account, error) {
	r.refunds++
	return r.record.Account, nil
}

type readProofSecrets struct{}

func (readProofSecrets) Seal(string, []byte) (string, error) { return "sealed", nil }
func (readProofSecrets) Open(string, string) ([]byte, error) { return []byte("password"), nil }

func TestMissingInventoryCannotRefundAnUnresolvedRestoredAccount(t *testing.T) {
	t.Parallel()
	const id = "11111111-1111-4111-8111-111111111111"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users/"+id {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("null"))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	repository := &missingInventoryRepository{record: domain.ProvisioningRecord{
		Account: domain.Account{
			ID: "account", UserID: "owner", RemoteUserID: id,
			CandidateUsername: "ada", Status: domain.StatusProvisioning,
		},
		PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:owner", CreateAttempted: true,
	}}
	service := domain.NewService(repository, client, nil, readProofSecrets{})
	err = service.Provision(context.Background(), "account")
	if !errors.Is(err, domain.ErrPendingReview) || repository.refunds != 0 || repository.reviews != 1 {
		t.Fatalf("missing inventory was treated as lost account proof: refunds=%d reviews=%d error=%v", repository.refunds, repository.reviews, err)
	}
}
