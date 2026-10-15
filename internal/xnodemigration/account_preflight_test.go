package xnodemigration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type preflightSource struct {
	*cutoverSource
	accounts []Account
}

func (s preflightSource) NodeAccounts(context.Context, SourceConfig, string) ([]Account, error) {
	return s.accounts, nil
}

type preflightRepository struct {
	*cutoverRepo
	identities []xnode.Identity
	user       model.User
}

func (r preflightRepository) XNodeIdentities(context.Context) ([]xnode.Identity, error) {
	return r.identities, nil
}
func (r preflightRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, nil
}

type preflightPanel struct {
	cutoverPanel
	credentialErr error
	verified      int
	notReady      bool
}

func (p *preflightPanel) Candidate(context.Context, string) (wire.Config, error) {
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`)}
	if !p.notReady {
		config.Users = []wire.User{{ID: 42}}
	}
	return config, nil
}

func (p *preflightPanel) ImportIdentity(context.Context, model.User, int64, string, string, string, ...bool) (xnode.Identity, error) {
	p.verified++
	return xnode.Identity{}, p.credentialErr
}

func TestCutoverChecksEveryServedSourceIdentityBeforeDisablingNode(t *testing.T) {
	for _, scenario := range []string{"matched", "unmapped", "changed-name", "changed-telegram", "changed-credentials", "provisioning-pending"} {
		t.Run(scenario, func(t *testing.T) {
			s, base, source, _, events := migrationCutoverFixture(t)
			username, telegram := "member", int64(101)
			repo := preflightRepository{cutoverRepo: base, identities: []xnode.Identity{{ProxyUserID: 42, LegacyUserID: "42", UserID: "panel"}}, user: model.User{ID: "panel", Username: &username, TelegramID: telegram}}
			account := Account{LegacyID: 42, Username: username, TelegramID: &telegram}
			panel := &preflightPanel{}
			switch scenario {
			case "provisioning-pending":
				panel.notReady = true
			case "unmapped":
				account.LegacyID = 43
			case "changed-name":
				account.Username = "other"
			case "changed-telegram":
				other := int64(102)
				account.TelegramID = &other
			case "changed-credentials":
				panel.credentialErr = xnode.ErrConflict
			}
			s.repo, s.source, s.panel = repo, preflightSource{cutoverSource: source, accounts: []Account{account}}, panel
			_, err := s.Cutover(context.Background(), "node")
			if scenario == "matched" {
				if err != nil || panel.verified != 1 || base.state.Nodes[0].Phase != "native" {
					t.Fatalf("matched handover=%+v,%v", base.state, err)
				}
				return
			}
			if !errors.Is(err, xnode.ErrConflict) || base.state.Nodes[0].Phase != "legacy" {
				t.Fatalf("unsafe source accepted=%+v,%v", base.state, err)
			}
			for _, event := range *events {
				if event == "source:disable" {
					t.Fatalf("serving source was interrupted: %v", *events)
				}
			}
		})
	}
}
