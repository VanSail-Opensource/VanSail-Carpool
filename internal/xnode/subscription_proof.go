package xnode

import (
	"context"
	"crypto/sha256"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// FirstSyncSubject is private applied-configuration evidence, never a wire or
// API payload. Hashes bind the exact encrypted source material without retaining
// another copy of credential ciphertext for every cached node configuration.
type FirstSyncSubject struct {
	ProxyUserID         int64    `json:"-"`
	OwnerProxyUserID    int64    `json:"-"`
	OwnerID             string   `json:"-"`
	AccountGeneration   int64    `json:"-"`
	PeriodID            string   `json:"-"`
	CredentialHash      [32]byte `json:"-"`
	OwnerCredentialHash [32]byte `json:"-"`
}

func (s *Service) compileWithFirstSync(ctx context.Context, node string) (wire.Config, map[int64]FirstSyncSubject, error) {
	candidate := s.withCompileReads(node)
	config, err := candidate.Compile(ctx, node)
	if err != nil {
		return config, nil, err
	}
	subjects, err := compiledFirstSyncSubjects(config, candidate.repo.(*compileReads))
	return config, subjects, err
}

// Every lookup is from the same per-compilation read set used for native
// credentials. A later repository read must not relabel an older configuration.
func compiledFirstSyncSubjects(config wire.Config, reads *compileReads) (map[int64]FirstSyncSubject, error) {
	subjects := make(map[int64]FirstSyncSubject)
	for _, user := range config.Users {
		principal, ok := reads.byID[int64(user.ID)]
		if !ok {
			return nil, ErrConflict
		}
		ownerID := principal.MemberOwnerID()
		if ownerID == "" {
			continue
		}
		owner, knownOwner := reads.byUser[ownerID]
		account, knownAccount := reads.users[ownerID]
		if !knownOwner || !knownAccount || owner.ProxyUserID <= 0 ||
			owner.UserID != ownerID || account.ID != ownerID || account.AccountGeneration < 0 ||
			principal.CredentialsCipher == "" || owner.CredentialsCipher == "" {
			return nil, ErrConflict
		}
		subjects[int64(user.ID)] = FirstSyncSubject{
			ProxyUserID:         int64(user.ID),
			OwnerProxyUserID:    owner.ProxyUserID,
			OwnerID:             ownerID,
			AccountGeneration:   account.AccountGeneration,
			PeriodID:            user.AccountingPeriod,
			CredentialHash:      sha256.Sum256([]byte(principal.CredentialsCipher)),
			OwnerCredentialHash: sha256.Sum256([]byte(owner.CredentialsCipher)),
		}
	}
	return subjects, nil
}
