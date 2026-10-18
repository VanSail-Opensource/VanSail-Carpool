package connections

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// Repository persists only connection scan metadata.
type Repository interface {
	CreateConnectionScan(context.Context, providerops.ConnectionScanInput, time.Time) (providerops.ConnectionScan, bool, error)
	ConnectionScanForUser(context.Context, string, string) (providerops.ConnectionScan, error)
	ConnectionScanByID(context.Context, string) (providerops.ConnectionScan, error)
	UpdateConnectionScan(context.Context, string, providerops.ConnectionScanUpdate, time.Time) (providerops.ConnectionScan, error)
	UserByID(context.Context, string) (model.User, error)
}

// Remote performs connection calls through the bounded Remnawave queue.
type Remote interface {
	RequestConnectionScan(context.Context, string) (string, error)
	PollConnectionScan(context.Context, string) (ProviderScan, error)
}

// NativeRemote starts an X-Node-only scan using a panel identity reference.
type NativeRemote interface {
	RequestNativeConnectionScan(context.Context, string) (string, error)
}

// Service owns connection scan creation, polling, and signed projections.
type Service struct {
	repository Repository
	remote     Remote
	signer     *Signer
	now        func() time.Time
}

// NewService creates the member connection service.
func NewService(repository Repository, remote Remote, signer *Signer) *Service {
	return &Service{repository: repository, remote: remote, signer: signer, now: time.Now}
}

// Request creates or replays one durable scan command.
func (s *Service) Request(ctx context.Context, user model.User, idempotencyKey string) (Scan, error) {
	if user.RemnaUserID == nil || strings.TrimSpace(*user.RemnaUserID) == "" {
		return Scan{}, ErrIdentityRequired
	}
	return s.request(ctx, user, idempotencyKey, scanFingerprint())
}

// RequestAdmin uses the existing durable scan/outbox with a native-only fingerprint.
func (s *Service) RequestAdmin(ctx context.Context, user model.User, idempotencyKey string) (Scan, error) {
	return s.request(ctx, user, idempotencyKey, nativeScanFingerprint())
}

func (s *Service) request(ctx context.Context, user model.User, idempotencyKey, fingerprint string) (Scan, error) {
	now := s.now().UTC()
	record, _, err := s.repository.CreateConnectionScan(ctx, providerops.ConnectionScanInput{
		UserID: user.ID, IdempotencyKey: idempotencyKey, RequestFingerprint: fingerprint, ExpiresAt: now.Add(ScanTTL),
	}, now)
	if err != nil {
		return Scan{}, err
	}
	return projectMetadata(record), nil
}

// Poll refreshes provider progress and returns ephemeral signed IP handles.
func (s *Service) Poll(ctx context.Context, userID, scanID string) (Scan, error) {
	return s.poll(ctx, userID, scanID, false)
}

// PollAdmin refuses old bridge scans rather than initiating a legacy provider read.
func (s *Service) PollAdmin(ctx context.Context, userID, scanID string) (Scan, error) {
	return s.poll(ctx, userID, scanID, true)
}

func (s *Service) poll(ctx context.Context, userID, scanID string, nativeOnly bool) (Scan, error) {
	now := s.now().UTC()
	record, err := s.repository.ConnectionScanForUser(ctx, scanID, userID)
	if err != nil || !now.Before(record.ExpiresAt) {
		return Scan{}, ErrScanNotFound
	}
	if nativeOnly && record.RequestFingerprint != nativeScanFingerprint() {
		return Scan{}, ErrScanNotFound
	}
	if record.Status == providerops.StatusFailed || record.Status == providerops.StatusPendingReview || record.ProviderJobID == "" {
		return projectMetadata(record), nil
	}
	result, err := s.remote.PollConnectionScan(ctx, record.ProviderJobID)
	if err != nil {
		return Scan{}, err
	}
	status := providerops.StatusProcessing
	errorCode := ""
	if result.Failed {
		status, errorCode = providerops.StatusFailed, "CONNECTION_SCAN_FAILED"
	} else if result.Completed {
		status = providerops.StatusSucceeded
	}
	if record.Status != providerops.StatusSucceeded {
		record, err = s.repository.UpdateConnectionScan(ctx, record.ID, providerops.ConnectionScanUpdate{
			Status: status, ProviderJobID: record.ProviderJobID, ProgressPercent: result.ProgressPercent, ErrorCode: errorCode,
		}, now)
		if err != nil {
			return Scan{}, err
		}
	}
	projection := projectMetadata(record)
	if result.Completed && !result.Failed {
		projection.Nodes, err = s.signNodes(userID, record, result.Nodes, now)
	}
	return projection, err
}

func (s *Service) signNodes(userID string, scan providerops.ConnectionScan, nodes []ProviderNode, now time.Time) ([]Node, error) {
	expires := now.Add(HandleTTL)
	if scan.ExpiresAt.Before(expires) {
		expires = scan.ExpiresAt
	}
	result := make([]Node, 0, len(nodes))
	for _, providerNode := range nodes {
		node := Node{UUID: providerNode.UUID, Name: providerNode.Name, CountryCode: countryCode(providerNode.CountryCode), Unavailable: providerNode.Unavailable, IPs: make([]IP, 0, len(providerNode.IPs))}
		for _, observation := range providerNode.IPs {
			handle, err := s.signer.Sign(HandleClaims{UserID: userID, ScanID: scan.ID, NodeUUID: providerNode.UUID, IP: observation.Address, Expires: expires})
			if err != nil {
				return nil, err
			}
			node.IPs = append(node.IPs, IP{Address: observation.Address, LastSeen: observation.LastSeen, Handle: handle})
		}
		result = append(result, node)
	}
	return result, nil
}

func scanFingerprint() string {
	digest := sha256.Sum256([]byte("connection-scan:v1"))
	return hex.EncodeToString(digest[:])
}

func nativeScanFingerprint() string {
	digest := sha256.Sum256([]byte("connection-scan:xnode-v1"))
	return hex.EncodeToString(digest[:])
}

func countryCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 2 || value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z' {
		return "ZZ"
	}
	return value
}

func projectMetadata(record providerops.ConnectionScan) Scan {
	return Scan{ID: record.ID, Completed: record.Status == providerops.StatusSucceeded,
		Failed:          record.Status == providerops.StatusFailed || record.Status == providerops.StatusPendingReview,
		ProgressPercent: record.ProgressPercent, Nodes: []Node{}, CreatedAt: record.CreatedAt, ExpiresAt: record.ExpiresAt}
}
