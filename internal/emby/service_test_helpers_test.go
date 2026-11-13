package emby

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	terminalServiceError  = errors.New("terminal provider error")
	transientServiceError = errors.New("transient provider error")
	missingServiceError   = errors.New("provider not found")
)

type fixedPrice int64

func (p fixedPrice) EmbySetupPriceTXBMinor(context.Context) (int64, error) { return int64(p), nil }

type serviceSecrets struct {
	context   string
	plaintext string
	openErr   error
}

func (s *serviceSecrets) Seal(context string, plaintext []byte) (string, error) {
	s.context, s.plaintext = context, string(plaintext)
	return "sealed", nil
}
func (s *serviceSecrets) Open(string, string) ([]byte, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	return []byte("temporary"), nil
}

type serviceRepository struct {
	baseUsername string
	record       ProvisioningRecord
	refundCalls  int
	retryCalls   int
	touchCalls   int
	retryError   error
	refundReason string
}

func (r *serviceRepository) EmbyBaseUsername(context.Context, string) (string, error) {
	return r.baseUsername, nil
}
func (r *serviceRepository) EmbyAccountForUser(context.Context, string) (Account, error) {
	if r.record.ID == "" {
		return Account{}, ErrNotFound
	}
	return r.record.Account, nil
}
func (r *serviceRepository) ListEmbyAccounts(context.Context, int) ([]Account, error) {
	if r.record.ID == "" {
		return []Account{}, nil
	}
	return []Account{r.record.Account}, nil
}
func (r *serviceRepository) EmbyProvisioningByID(context.Context, string) (ProvisioningRecord, error) {
	return r.record, nil
}
func (r *serviceRepository) BeginEmbyProvisioning(context.Context, string, time.Time) (ProvisioningRecord, error) {
	if r.record.Status == StatusQueued {
		r.record.Status = StatusProvisioning
	}
	return r.record, nil
}
func (r *serviceRepository) SetEmbyCandidate(_ context.Context, _ string, candidate string, _ time.Time) error {
	r.record.CandidateUsername = candidate
	return nil
}
func (r *serviceRepository) MarkEmbyCreateAttempted(context.Context, string, time.Time) error {
	r.record.CreateAttempted = true
	return nil
}
func (r *serviceRepository) SetEmbyRemoteIdentity(_ context.Context, _ string, id, name string, _ time.Time) error {
	r.record.RemoteUserID, r.record.RemoteUsername, r.record.CandidateUsername = id, name, name
	return nil
}
func (r *serviceRepository) RequeueEmbyProvisioning(_ context.Context, _ string, provisionErr error, _ time.Time) error {
	r.retryCalls++
	r.retryError = provisionErr
	r.record.Status = StatusQueued
	return nil
}
func (r *serviceRepository) MarkEmbyPendingReview(_ context.Context, _ string, provisionErr error, _ time.Time) error {
	r.retryError = provisionErr
	r.record.Status = StatusPendingReview
	return nil
}
func (r *serviceRepository) MarkEmbyProvisioned(_ context.Context, _ string, preferences Preferences, _ time.Time) error {
	r.record.Preferences = preferences
	r.record.Status, r.record.PasswordCiphertext, r.record.PasswordContext = StatusActive, "", ""
	return nil
}
func (r *serviceRepository) FailAndRefundEmbySetup(_ context.Context, _ string, reason string, _ time.Time) (Account, error) {
	r.refundCalls++
	r.refundReason = reason
	r.record.Status, r.record.PasswordCiphertext, r.record.PasswordContext = StatusFailed, "", ""
	return r.record.Account, nil
}
func (r *serviceRepository) UpdateEmbyPreferences(_ context.Context, _ string, preferences Preferences, _ time.Time) (Account, error) {
	r.record.Preferences = preferences
	return r.record.Account, nil
}
func (r *serviceRepository) TouchEmbyAccount(context.Context, string, time.Time) error {
	r.touchCalls++
	return nil
}

type serviceRemote struct {
	byName          map[string]RemoteUser
	byID            map[string]RemoteUser
	findErr         error
	createCalls     int
	createdName     string
	createErr       error
	createCommits   bool
	currentPassword []byte
	password        []byte
	passwordErr     error
	updatedPolicy   Policy
}

func newServiceRemote() *serviceRemote {
	return &serviceRemote{byName: make(map[string]RemoteUser), byID: make(map[string]RemoteUser)}
}
func (r *serviceRemote) addUser(user RemoteUser) {
	r.byName[user.Name], r.byID[user.ID] = user, user
}
func (r *serviceRemote) FindUserByName(_ context.Context, name string) (RemoteUser, bool, error) {
	if r.findErr != nil {
		return RemoteUser{}, false, r.findErr
	}
	user, exists := r.byName[name]
	return user, exists, nil
}
func (r *serviceRemote) CreateUser(_ context.Context, name string) (RemoteUser, error) {
	r.createCalls++
	r.createdName = name
	user := RemoteUser{ID: "created", Name: name, Policy: basePolicy()}
	if r.createErr == nil || r.createCommits {
		r.addUser(user)
	}
	if r.createErr != nil {
		return RemoteUser{}, r.createErr
	}
	return user, nil
}
func (r *serviceRemote) GetUser(_ context.Context, id string) (RemoteUser, error) {
	user, exists := r.byID[id]
	if !exists {
		return RemoteUser{}, missingServiceError
	}
	return user, nil
}

func (r *serviceRemote) SetPassword(_ context.Context, _ string, current, password []byte) error {
	r.currentPassword = append([]byte(nil), current...)
	r.password = append([]byte(nil), password...)
	return r.passwordErr
}
func (r *serviceRemote) UpdatePolicy(_ context.Context, id string, policy Policy) error {
	r.updatedPolicy = policy.Clone()
	user := r.byID[id]
	user.Policy = policy.Clone()
	r.byID[id], r.byName[user.Name] = user, user
	return nil
}
func (*serviceRemote) ListSelectableFolders(context.Context) ([]Folder, error) {
	return []Folder{{ID: "movies", Name: "Movies"}}, nil
}
func (*serviceRemote) ListParentalRatings(context.Context) ([]ParentalRating, error) {
	return []ParentalRating{{Name: "PG-13", Value: 13}}, nil
}
func (*serviceRemote) IsNotFound(err error) bool { return errors.Is(err, missingServiceError) }
func (*serviceRemote) IsTerminal(err error) bool { return errors.Is(err, terminalServiceError) }

func basePolicy() Policy {
	return Policy{"EnableRemoteAccess": json.RawMessage(`true`), "FutureField": json.RawMessage(`"kept"`)}
}

func int32Pointer(value int32) *int32 { return &value }

type serviceCipher struct {
	encryptContext string
	plaintext      string
}

func (c *serviceCipher) Encrypt(context, plaintext string) (string, error) {
	c.encryptContext, c.plaintext = context, plaintext
	return "encrypted", nil
}

func (c *serviceCipher) Decrypt(context, ciphertext string) (string, error) {
	if context != c.encryptContext || ciphertext != "encrypted" {
		return "", errors.New("cipher context mismatch")
	}
	return c.plaintext, nil
}
