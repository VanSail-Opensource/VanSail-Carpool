package xnodemigration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"path/filepath"
)

func (s *Service) Import(ctx context.Context, digest string) (xnode.MigrationState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return state, err
	}
	if state.SnapshotDigest != digest || (state.Phase != "preview" && state.Phase != "importing" && state.Phase != "active") {
		return state, xnode.ErrConflict
	}
	if state.BackupName == "" {
		backup, err := s.backups.Run(ctx)
		if err != nil {
			return state, err
		}
		if backup.Status != "complete" {
			return state, xnode.ErrUnavailable
		}
		state.BackupName = filepath.Base(backup.Path)
	}
	state.Phase = "importing"
	if err = s.repo.SaveXNodeMigration(ctx, state); err != nil {
		return state, err
	}
	raw, err := s.vault.Decrypt("xnode.migration.snapshot", state.SnapshotCipher)
	if err != nil {
		return state, err
	}
	var snapshot Snapshot
	if json.Unmarshal([]byte(raw), &snapshot) != nil {
		return state, xnode.ErrInvalid
	}
	profileDocs := map[string]json.RawMessage{}
	for _, profile := range snapshot.Profiles {
		profileDocs[profile.UUID] = profile.Config
	}
	bindings := map[string]map[string]string{}
	for _, node := range snapshot.Nodes {
		document, ok := profileDocs[node.ProfileID]
		if !ok {
			return state, missingProfile(node.ProfileID)
		}
		native, tags, err := NativeDocument(node.ProfileID, document, node)
		if err != nil {
			return state, err
		}
		bindings[node.ID] = tags
		id := configID(node)
		if _, err = s.repo.XNodeConfiguration(ctx, id); isMissing(err) {
			cipher, err := s.vault.Encrypt("xnode.config."+id, string(native))
			if err != nil {
				return state, err
			}
			if err = s.repo.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: id, Name: fmt.Sprintf("%s (%s)", node.Name, node.ID), DocumentCipher: cipher, SnippetIDs: []string{}}); err != nil {
				return state, err
			}
		} else if err != nil {
			return state, err
		}
		settings, err := json.Marshal(xnode.NodeSettings{})
		if err != nil {
			return state, err
		}
		cipher, err := s.vault.Encrypt("xnode.node."+node.ID, string(settings))
		if err != nil {
			return state, err
		}
		item := xnode.Node{ID: node.ID, Name: node.Name, CountryCode: node.CountryCode, ProviderName: node.ProviderName, MultiplierFP: node.MultiplierFP, Enabled: node.Enabled, ConfigID: id, SettingsCipher: cipher}
		checkpoint := xnode.MigrationNode{NodeID: node.ID, LegacyNodeID: node.ID, Phase: "legacy", SourceDigest: digest}
		if err = s.repo.ImportXNodeMigrationNode(ctx, item, checkpoint); err != nil {
			return state, err
		}
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return state, err
	}
	existing := map[string]bool{}
	for _, group := range groups {
		existing[group.ID] = true
	}
	for _, source := range snapshot.Groups {
		if existing[source.ID] {
			continue
		}
		group := xnode.Group{ID: source.ID, Name: source.Name, Enabled: true, Bindings: []xnode.GroupBinding{}}
		for _, inbound := range source.Inbounds {
			for node, tags := range bindings {
				if tag := tags[inbound.UUID]; tag != "" {
					group.Bindings = append(group.Bindings, xnode.GroupBinding{GroupID: source.ID, NodeID: node, InboundTag: tag})
				}
			}
		}
		if err = s.repo.SaveXNodeGroup(ctx, group); err != nil {
			return state, err
		}
	}
	for _, account := range snapshot.Accounts {
		if account.ServiceName != "" {
			if err = s.importMonitoring(ctx, account, snapshot); err != nil {
				return state, err
			}
			continue
		}
		if account.PanelUserID == "" {
			continue
		}
		user, err := s.repo.UserByID(ctx, account.PanelUserID)
		if err != nil {
			return state, err
		}
		identity, err := s.panel.ImportIdentity(ctx, user, account.LegacyID, account.UUID, account.Trojan, account.Chacha, account.Disabled)
		if err != nil {
			return state, err
		}
		if err = s.panel.SyncLegacyUsage(ctx, identity, xnode.LegacyUsage{UsedBytes: account.UsedBytes, LifetimeBytes: account.LifetimeBytes, LastResetAt: account.LastResetAt, SyncedAt: snapshot.TakenAt}); err != nil {
			return state, err
		}
		if err = s.importHistory(ctx, identity, account, snapshot); err != nil {
			return state, err
		}
	}
	if err = s.importEndpoints(ctx, snapshot, bindings); err != nil {
		return state, err
	}
	state.Phase = "active"
	if err = s.repo.SaveXNodeMigration(ctx, state); err != nil {
		return state, err
	}
	return s.repo.XNodeMigration(ctx)
}
func isMissing(err error) bool { return err == xnode.ErrNotFound }
