package xnodemigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"strings"
)

func (s *Service) Preview(ctx context.Context, input PreviewInput) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(input.SourceBackupReference) == "" || len(input.SourceBackupReference) > 512 {
		return Preview{}, xnode.ErrInvalid
	}
	if _, err := json.Marshal(input.Source); err != nil {
		return Preview{}, err
	}
	users, err := s.repo.MigrationPanelAccounts(ctx)
	if err != nil {
		return Preview{}, err
	}
	snapshot, err := s.source.Capture(ctx, input.Source, users, input.Monitoring)
	if err != nil {
		return Preview{}, err
	}
	raw, err := json.Marshal(snapshot)
	if err != nil || len(raw) > 32<<20 {
		return Preview{}, xnode.ErrInvalid
	}
	digest := sha256.Sum256(raw)
	result := inspectSnapshot(snapshot)
	result.Digest = hex.EncodeToString(digest[:])
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return result, err
	}
	if state.Phase != "none" && state.Phase != "preview" {
		return result, xnode.ErrConflict
	}
	sourceJSON, err := json.Marshal(input.Source)
	if err != nil {
		return result, err
	}
	state.SourceCipher, err = s.vault.Encrypt("xnode.migration.source", string(sourceJSON))
	if err != nil {
		return result, err
	}
	state.SnapshotCipher, err = s.vault.Encrypt("xnode.migration.snapshot", string(raw))
	if err != nil {
		return result, err
	}
	state.Phase, state.SnapshotDigest, state.SourceBackupReference = "preview", result.Digest, input.SourceBackupReference
	return result, s.repo.SaveXNodeMigration(ctx, state)
}
func inspectSnapshot(snapshot Snapshot) Preview {
	result := Preview{Nodes: len(snapshot.Nodes), Groups: len(snapshot.Groups), Accounts: len(snapshot.Accounts), Endpoints: len(snapshot.Hosts), Issues: []Issue{}}
	for _, account := range snapshot.Accounts {
		if account.PanelUserID == "" && account.ServiceName == "" {
			result.Issues = append(result.Issues, Issue{ResourceID: account.Username, Code: "UNMATCHED_IDENTITY", Blocking: false})
		}
	}
	for _, profile := range snapshot.Profiles {
		var native map[string]any
		if json.Unmarshal(profile.Config, &native) != nil {
			result.Issues = append(result.Issues, Issue{ResourceID: profile.UUID, Code: "INVALID_CONFIGURATION", Blocking: true})
			continue
		}
		inbounds, _ := native["inbounds"].([]any)
		for _, raw := range inbounds {
			inbound, _ := raw.(map[string]any)
			protocol, _ := inbound["protocol"].(string)
			if !sourceManagedProtocol(protocol) && protocol != "dokodemo-door" && protocol != "tunnel" {
				result.Issues = append(result.Issues, Issue{ResourceID: profile.UUID, Code: "UNSUPPORTED_UTILITY_INBOUND", Blocking: true})
			}
			if inbound["protocol"] == "shadowsocks" {
				settings, _ := inbound["settings"].(map[string]any)
				method, _ := settings["method"].(string)
				if method != "chacha20-ietf-poly1305" && method != "2022-blake3-aes-128-gcm" && method != "2022-blake3-aes-256-gcm" {
					result.Issues = append(result.Issues, Issue{ResourceID: profile.UUID, Code: "UNSUPPORTED_SHADOWSOCKS", Blocking: true})
				}
			}
		}
	}
	multipliers := map[string]int64{}
	for _, node := range snapshot.Nodes {
		multipliers[node.ID] = node.MultiplierFP
	}
	for _, account := range snapshot.Accounts {
		if account.PanelUserID != "" {
			if _, err := History(account.LegacyID, account.Stats, multipliers); err != nil {
				result.Issues = append(result.Issues, Issue{ResourceID: account.Username, Code: "INCOMPLETE_TRAFFIC_HISTORY", Blocking: true})
			}
		}
	}
	for _, raw := range snapshot.Hosts {
		var host map[string]any
		if json.Unmarshal(raw, &host) != nil {
			result.Issues = append(result.Issues, Issue{Code: "INVALID_HOST", Blocking: true})
			continue
		}
		id, _ := host["uuid"].(string)
		var metadata remnawave.Host
		if json.Unmarshal(raw, &metadata) != nil || !metadata.ValidSquadPolicy() {
			result.Issues = append(result.Issues, Issue{ResourceID: id, Code: "INVALID_HOST", Blocking: true})
		}

		if host["xrayJsonTemplateUuid"] != nil {
			result.Issues = append(result.Issues, Issue{ResourceID: id, Code: "CLIENT_TEMPLATE_REQUIRES_CONVERSION", Blocking: true})
		}
	}
	if issues, err := sharedSS2022Hosts(snapshot, "", nil); err == nil {
		result.Issues = append(result.Issues, issues...)
	}
	return result
}
