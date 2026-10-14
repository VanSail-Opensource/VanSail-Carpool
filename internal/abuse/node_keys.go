package abuse

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func (s *Service) SyncNodes(ctx context.Context, vault *secret.Vault, now time.Time) ([]NodeCredential, error) {
	if s.nodes == nil || vault == nil {
		return nil, ErrInvalid
	}
	nodes, err := s.liveNodes(ctx)
	if err != nil {
		return nil, err
	}
	if repository, ok := s.repo.(nodeCredentialReconciler); ok {
		if err := repository.ReconcileNodeCredentials(ctx, nodes); err != nil {
			return nil, err
		}
	}
	existing, err := s.repo.NodeCredentials(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, item := range existing {
		seen[item.UUID] = true
	}
	for _, node := range nodes {
		node.UUID = strings.TrimSpace(node.UUID)
		if node.UUID == "" {
			return nil, ErrInvalid
		}
		if seen[node.UUID] {
			continue
		}
		token, tokenErr := newToken()
		if tokenErr != nil {
			return nil, tokenErr
		}
		sealed, sealErr := vault.Encrypt("abuse.node."+node.UUID, token)
		if sealErr != nil {
			return nil, sealErr
		}
		if err = s.repo.SaveNodeCredential(ctx, node, tokenDigest(token), sealed, now.UTC()); err != nil {
			return nil, err
		}
	}
	credentials, err := s.repo.NodeCredentials(ctx)
	if err != nil {
		return nil, err
	}
	live := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		live[node.UUID] = true
	}
	result := make([]NodeCredential, 0, len(nodes))
	for _, credential := range credentials {
		if live[credential.UUID] {
			result = append(result, credential)
		}
	}
	return result, nil
}
func (s *Service) CopyNodeKey(ctx context.Context, vault *secret.Vault, nodeID string) (string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if vault == nil || nodeID == "" {
		return "", ErrInvalid
	}
	if err := s.requireLiveNode(ctx, nodeID); err != nil {
		return "", err
	}
	sealed, err := s.repo.CopyNodeCredential(ctx, nodeID)
	if err != nil {
		return "", err
	}
	return vault.Decrypt("abuse.node."+nodeID, sealed)
}
func (s *Service) RotateNodeKey(ctx context.Context, vault *secret.Vault, nodeID string, now time.Time) (string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if vault == nil || nodeID == "" {
		return "", ErrInvalid
	}
	if err := s.requireLiveNode(ctx, nodeID); err != nil {
		return "", err
	}
	if _, err := s.repo.CopyNodeCredential(ctx, nodeID); err != nil {
		return "", err
	}
	token, err := newToken()
	if err != nil {
		return "", err
	}
	sealed, err := vault.Encrypt("abuse.node."+nodeID, token)
	if err != nil {
		return "", err
	}
	if err = s.repo.SaveNodeCredential(ctx, Node{UUID: nodeID}, tokenDigest(token), sealed, now.UTC()); err != nil {
		return "", err
	}
	return token, nil
}
func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("create node key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
