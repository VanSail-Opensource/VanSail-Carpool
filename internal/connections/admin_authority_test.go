package connections

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func scanTestAdministrator() model.User {
	return model.User{ID: "administrator", Role: "admin", AccountStatus: "active"}
}

type scanAuthorityRepository struct {
	targetScopedScanRepository
	actor   model.User
	input   providerops.ConnectionScanInput
	creates int
}

func (r *scanAuthorityRepository) UserByID(context.Context, string) (model.User, error) {
	return r.actor, nil
}
func (r *scanAuthorityRepository) CreateConnectionScan(_ context.Context, in providerops.ConnectionScanInput, _ time.Time) (providerops.ConnectionScan, bool, error) {
	r.input, r.creates = in, r.creates+1
	return providerops.ConnectionScan{ID: "accepted"}, false, nil
}

type scanAuthorityRemote struct {
	repository *scanAuthorityRepository
	calls      int
}

func (*scanAuthorityRemote) RequestConnectionScan(context.Context, string) (string, error) {
	return "", errors.New("unexpected start")
}
func (r *scanAuthorityRemote) PollConnectionScan(context.Context, string) (ProviderScan, error) {
	r.calls++
	r.repository.actor.Role = "user"
	return ProviderScan{Completed: true, Nodes: []ProviderNode{{UUID: "private-node", IPs: []Observation{{Address: "203.0.113.1"}}}}}, nil
}

func TestAdministratorScanRequiresCurrentOriginalAuthority(t *testing.T) {
	for _, state := range []string{"demoted", "blocked", "purged", "revived"} {
		t.Run(state, func(t *testing.T) {
			actor := scanTestAdministrator()
			repository := &scanAuthorityRepository{actor: actor}
			switch state {
			case "demoted":
				repository.actor.Role = "user"
			case "blocked":
				repository.actor.BotBlocked = true
			case "purged":
				repository.actor.AccountStatus = "purged"
			case "revived":
				repository.actor.AccountGeneration++
			}
			service := NewService(repository, nil, nil)
			if _, err := service.RequestAdmin(context.Background(), actor, model.User{ID: "target"}, "new"); !errors.Is(err, ErrScanForbidden) || repository.creates != 0 {
				t.Fatal("revoked actor admitted scan", err, repository.creates)
			}
			if _, err := service.PollAdmin(context.Background(), actor, "target", "scan"); !errors.Is(err, ErrScanForbidden) || repository.lastOwnerID != "" {
				t.Fatal("revoked actor read scan", err, repository.lastOwnerID)
			}
		})
	}
	actor := scanTestAdministrator()
	actor.AccountGeneration = 7
	repository := &scanAuthorityRepository{actor: actor}
	if _, err := NewService(repository, nil, nil).RequestAdmin(context.Background(), actor, model.User{ID: "target"}, "new"); err != nil || repository.input.ActorUserID != actor.ID || repository.input.ActorAccountGeneration != 7 || repository.input.UserID != "target" {
		t.Fatal("actor evidence lost before SQL admission", repository.input, err)
	}
}

func TestAdministratorDemotionDuringScanPollDiscardsIPProjection(t *testing.T) {
	actor := scanTestAdministrator()
	repository := &scanAuthorityRepository{actor: actor, targetScopedScanRepository: targetScopedScanRepository{scan: providerops.ConnectionScan{ID: "scan", UserID: "target", Status: providerops.StatusSucceeded, ProviderJobID: "native-job", RequestFingerprint: nativeScanFingerprint(), ExpiresAt: time.Now().Add(time.Minute)}}}
	remote := &scanAuthorityRemote{repository: repository}
	signer, err := NewSigner([]byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := NewService(repository, remote, signer).PollAdmin(context.Background(), actor, "target", "scan")
	if !errors.Is(err, ErrScanForbidden) || !reflect.DeepEqual(projection, Scan{}) || remote.calls != 1 {
		t.Fatal("concurrently demoted actor received IP observations", projection, err, remote.calls)
	}
}
