package iplookup

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Repository atomically applies local costs and references queued shared reports.
type Repository interface {
	IPLookupState(context.Context, string, time.Time) (State, error)
	IPLookupQuote(context.Context, string, string, bool, time.Time) (Quote, error)
	IPLookupQuoteResponse(context.Context, string, string, bool, time.Time) (QuoteResponse, error)
	CreateIPLookupCheck(context.Context, string, string, Quote, time.Time) (model.OperationReceipt, error)
	IPLookupCheck(context.Context, string, string) (Check, error)
}

// Service signs owner-bound quotes; all financial decisions remain transactional.
type Service struct {
	Live       *LiveService
	Enhanced   *EnhancedService
	repository Repository
	signingKey []byte
	now        func() time.Time
}

// NewService constructs the IP check facade with the existing application signing key.
func NewService(repo Repository, key []byte) *Service {
	return &Service{repository: repo, signingKey: append([]byte(nil), key...), now: time.Now}
}

// State reads current availability without consuming quota or requesting providers.
func (s *Service) State(ctx context.Context, user string) (State, error) {
	return s.repository.IPLookupState(ctx, user, s.now().UTC())
}

// Progress reads shared work without creating a receipt or changing costs.
func (s *Service) Progress(ctx context.Context, raw string) (*SharedProgress, error) {
	ip, err := CanonicalIP(raw)
	if err != nil {
		return nil, err
	}
	if repo, ok := s.repository.(interface {
		IPLookupSharedProgress(context.Context, string) (*SharedProgress, error)
	}); ok {
		return repo.IPLookupSharedProgress(ctx, ip)
	}
	return nil, nil
}

// Quote returns a two-minute signed price for a canonical public IP.
func (s *Service) Quote(ctx context.Context, user, rawIP string, refresh bool) (Quote, error) {
	ip, err := CanonicalIP(rawIP)
	if err != nil {
		return Quote{}, err
	}
	q, err := s.repository.IPLookupQuote(ctx, user, ip, refresh, s.now().UTC())
	if err != nil {
		return q, err
	}
	q.ExpiresAt = s.now().Add(2 * time.Minute).Unix()
	q.Token = s.signature(user, q)
	return q, nil
}

// PreviewQuote reads shared evidence without billing and signs only the submission fields.
func (s *Service) PreviewQuote(ctx context.Context, user, rawIP string, refresh bool) (QuoteResponse, error) {
	ip, err := CanonicalIP(rawIP)
	if err != nil {
		return QuoteResponse{}, err
	}
	result, err := s.repository.IPLookupQuoteResponse(ctx, user, ip, refresh, s.now().UTC())
	if err != nil {
		return result, err
	}
	result.ExpiresAt = s.now().Add(2 * time.Minute).Unix()
	result.Token = s.signature(user, result.Quote)
	if s.Enhanced != nil && result.CachedReport != nil {
		report := s.Enhanced.EffectiveReport(*result.CachedReport, ip)
		result.CachedReport = &report
	}
	return result, nil
}

// Submit verifies quote integrity; replay and expiry are checked inside persistence.
func (s *Service) Submit(ctx context.Context, user, key string, q Quote) (model.OperationReceipt, error) {
	ip, err := CanonicalIP(q.IP)
	if err != nil || ip != q.IP || !hmac.Equal([]byte(q.Token), []byte(s.signature(user, q))) {
		return model.OperationReceipt{}, &CodeError{"IP_LOOKUP_QUOTE_CHANGED"}
	}
	return s.repository.CreateIPLookupCheck(ctx, user, key, q, s.now().UTC())
}

// Check reads only an operation owned by the authenticated member.
func (s *Service) Check(ctx context.Context, user, id string) (Check, error) {
	result, err := s.repository.IPLookupCheck(ctx, user, id)
	if err == nil && s.Enhanced != nil && result.Report != nil {
		report := s.Enhanced.EffectiveReport(*result.Report, result.RequestedIP)
		result.Report = &report
	}
	return result, err
}

func (s *Service) signature(user string, q Quote) string {
	q.Token = ""
	b, _ := json.Marshal(q)
	mac := hmac.New(sha256.New, s.signingKey)
	_, _ = mac.Write([]byte(user + "\x00"))
	_, _ = mac.Write(b)
	return hex.EncodeToString(mac.Sum(nil))
}
