// Package emby implements the local Emby account lifecycle and provisioning saga.
package emby

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const (
	// ProvisionOutboxKind identifies durable Emby account provisioning work.
	ProvisionOutboxKind = "emby_provision_account"

	// StatusQueued means provisioning is ready for the outbox worker.
	StatusQueued = "queued"
	// StatusProvisioning means the worker is advancing the remote saga.
	StatusProvisioning = "provisioning"
	// StatusActive means the remote account, password, and policy are complete.
	StatusActive = "active"
	// StatusFailed means setup terminated and its debit was refunded.
	StatusFailed = "failed"
	// StatusPendingReview means a provider mutation outcome needs explicit review.
	StatusPendingReview = "pending_review"
)

var (
	// ErrNotFound means the requested Emby account does not exist locally.
	ErrNotFound = errors.New("Emby account not found")
	// ErrAccountExists means a user already owns a non-terminal Emby account.
	ErrAccountExists = errors.New("Emby account already exists")
	// ErrInvalidSetup means setup input or configured pricing is invalid.
	ErrInvalidSetup = errors.New("invalid Emby setup")
	// ErrRemoteAccountMissing means a previously linked Emby account disappeared upstream.
	ErrRemoteAccountMissing = errors.New("linked Emby account is missing")
	// ErrPendingReview means an automatic retry could duplicate an ambiguous mutation.
	ErrPendingReview = errors.New("Emby provider outcome requires review")
)

// Policy holds a complete Emby Users.UserPolicy document. Raw values preserve
// fields added by newer Emby versions while the service overlays its restrictions.

type Policy map[string]json.RawMessage

// Clone returns a deep copy suitable for mutation and resubmission.

func (p Policy) Clone() Policy {
	cloned := make(Policy, len(p)+16)
	for key, value := range p {
		cloned[key] = append(json.RawMessage(nil), value...)
	}
	return cloned
}

// Preferences is the only user-controlled portion of an Emby policy.

type Preferences struct {
	MaxParentalRating  *int32   `json:"maxParentalRating"`
	DisabledLibraryIDs []string `json:"disabledLibraryIds"`
}

// Account is the safe local view of an Emby account. It never contains a password.

type Account struct {
	ID                 string
	UserID             string
	BaseUsername       string
	RemoteUserID       string
	RemoteUsername     string
	CandidateUsername  string
	Status             string
	SetupPriceTXBMinor int64
	SetupAttempt       int
	Retryable          bool
	Preferences        Preferences
	LastError          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ProvisionedAt      *time.Time
	RefundedAt         *time.Time
}

// ProvisioningRecord contains the server-only encrypted fields needed by the worker.

type ProvisioningRecord struct {
	Account
	PasswordCiphertext     string
	PasswordContext        string
	CreateAttempted        bool
	PendingPreferencesJSON string
}

// QueueSetupInput is a fully priced and sealed setup attempt.

type QueueSetupInput struct {
	ID                 string
	UserID             string
	BaseUsername       string
	PasswordCiphertext string
	PasswordContext    string
	SetupPriceTXBMinor int64
	Preferences        Preferences
}

// Folder is an Emby selectable media folder.

type Folder struct {
	ID   string
	Name string
}

// ParentalRating is an Emby-provided rating choice.

type ParentalRating struct {
	Name  string
	Value int32
}

// Options contains current upstream choices for account setup and preferences.

type Options struct {
	Folders []Folder
	Ratings []ParentalRating
}

// RemoteUser is the exact upstream state needed for reconciliation and policy updates.

type RemoteUser struct {
	ID     string
	Name   string
	Policy Policy
}

// Remote is the narrow, administrator-authenticated Emby API surface.

type Remote interface {
	FindUserByName(context.Context, string) (RemoteUser, bool, error)
	CreateUser(context.Context, string) (RemoteUser, error)
	GetUser(context.Context, string) (RemoteUser, error)
	SetPassword(context.Context, string, []byte, []byte) error
	UpdatePolicy(context.Context, string, Policy) error
	ListSelectableFolders(context.Context) ([]Folder, error)
	ListParentalRatings(context.Context) ([]ParentalRating, error)
	IsNotFound(error) bool
	IsTerminal(error) bool
}

// Repository is the transactional persistence surface used by the Emby service.

type Repository interface {
	EmbyBaseUsername(context.Context, string) (string, error)
	EmbyAccountForUser(context.Context, string) (Account, error)
	ListEmbyAccounts(context.Context, int) ([]Account, error)
	EmbyProvisioningByID(context.Context, string) (ProvisioningRecord, error)
	BeginEmbyProvisioning(context.Context, string, time.Time) (ProvisioningRecord, error)
	SetEmbyCandidate(context.Context, string, string, time.Time) error
	MarkEmbyCreateAttempted(context.Context, string, time.Time) error
	SetEmbyRemoteIdentity(context.Context, string, string, string, time.Time) error
	RequeueEmbyProvisioning(context.Context, string, error, time.Time) error
	MarkEmbyPendingReview(context.Context, string, error, time.Time) error
	MarkEmbyProvisioned(context.Context, string, Preferences, time.Time) error
	FailAndRefundEmbySetup(context.Context, string, string, time.Time) (Account, error)
	UpdateEmbyPreferences(context.Context, string, Preferences, time.Time) (Account, error)
	TouchEmbyAccount(context.Context, string, time.Time) error
}

// PriceSource returns the server-owned setup price in integer hundredths of TXB.

type PriceSource interface {
	EmbySetupPriceTXBMinor(context.Context) (int64, error)
}

// PriceFunc adapts a trusted setup-price function to PriceSource.

type PriceFunc func(context.Context) (int64, error)

// EmbySetupPriceTXBMinor invokes the trusted price function.

func (f PriceFunc) EmbySetupPriceTXBMinor(ctx context.Context) (int64, error) { return f(ctx) }

// SecretBox seals temporary provisioning passwords with associated context.

type SecretBox interface {
	Seal(context string, plaintext []byte) (string, error)
	Open(context, ciphertext string) ([]byte, error)
}

// Cipher is implemented by the existing AES-GCM credential vault.
