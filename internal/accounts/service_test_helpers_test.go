package accounts

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

type accountsRepository struct {
	user                 model.User
	sessionUser          model.User
	upsertErr            error
	replaceSessionErr    error
	sessionLookupErr     error
	userByIDErr          error
	reserveErr           error
	completeErr          error
	userByTelegramErr    error
	upsertAdmin          bool
	sessionHash          []byte
	sessionUserID        string
	sessionExpires       time.Time
	lookupSessionHash    []byte
	groupJoined          bool
	channelJoined        bool
	reservedUsername     string
	completedUserID      string
	recoveryUser         model.User
	recoveryStarted      bool
	recoveryReason       string
	agreementRevision    int
	requiredAgreementIDs []string
	activeCombo          bool
	activeComboErr       error
}

func (r *accountsRepository) UpsertTelegramUser(_ context.Context, _ model.TelegramProfile, admin bool) (model.User, bool, error) {
	r.upsertAdmin = admin
	return r.user, true, r.upsertErr
}
func (r *accountsRepository) ReplaceSession(_ context.Context, hash []byte, userID string, expires time.Time) error {
	r.sessionHash, r.sessionUserID, r.sessionExpires = append([]byte(nil), hash...), userID, expires
	return r.replaceSessionErr
}
func (r *accountsRepository) UserBySession(_ context.Context, hash []byte, _ time.Time) (model.User, error) {
	r.lookupSessionHash = append([]byte(nil), hash...)
	return r.sessionUser, r.sessionLookupErr
}
func (r *accountsRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, r.userByIDErr
}
func (r *accountsRepository) UserByTelegramID(context.Context, int64) (model.User, error) {
	return r.user, r.userByTelegramErr
}
func (r *accountsRepository) UpdateMembership(_ context.Context, _ string, group, channel bool) (model.User, error) {
	r.groupJoined, r.channelJoined = group, channel
	r.user.GroupJoined, r.user.ChannelJoined = group, channel
	return r.user, nil
}
func (r *accountsRepository) HasActiveCombo(context.Context, string, time.Time) (bool, error) {
	return r.activeCombo, r.activeComboErr
}
func (r *accountsRepository) ReserveUsername(_ context.Context, _ string, username string) error {
	r.reservedUsername = username
	return r.reserveErr
}
func (r *accountsRepository) CurrentAgreementContract(context.Context) (int, []string, error) {
	return r.agreementRevision, append([]string(nil), r.requiredAgreementIDs...), nil
}
func (r *accountsRepository) CompleteOnboardingRevision(_ context.Context, userID string, _ int, _ []string, _ time.Time) (model.User, error) {
	r.completedUserID = userID
	return r.user, r.completeErr
}
func (r *accountsRepository) QueueRemnawaveRepair(_ context.Context, _ string, missingID string, _ time.Time) (model.User, error) {
	r.recoveryStarted, r.recoveryReason = true, missingID
	return r.recoveryUser, nil
}

type accountsValidator struct {
	profile model.TelegramProfile
	err     error
}

func (v *accountsValidator) Validate(string) (model.TelegramProfile, error) { return v.profile, v.err }

type accountsTelegram struct {
	memberships   map[string]bool
	createErr     error
	membershipErr error
	approveErr    error
	revokeErr     error
	declineErr    error
	inviteNames   []string
	inviteChatIDs []string
	revokedLinks  []string
	approveCalls  int
	declineCalls  int
}

func (t *accountsTelegram) CreateJoinRequestInvite(_ context.Context, chatID, name string, _ time.Time) (string, error) {
	t.inviteNames = append(t.inviteNames, name)
	t.inviteChatIDs = append(t.inviteChatIDs, chatID)
	return "https://invite.test/" + chatID, t.createErr
}
func (t *accountsTelegram) GetMembership(_ context.Context, chatID string, _ int64) (bool, error) {
	return t.memberships[chatID], t.membershipErr
}
func (t *accountsTelegram) ApproveJoinRequest(context.Context, string, int64) error {
	t.approveCalls++
	return t.approveErr
}
func (t *accountsTelegram) DeclineJoinRequest(context.Context, string, int64) error {
	t.declineCalls++
	return t.declineErr
}
func (t *accountsTelegram) RevokeInviteLink(_ context.Context, _ string, link string) error {
	t.revokedLinks = append(t.revokedLinks, link)
	return t.revokeErr
}

type accountsFindResponse struct {
	remote RemoteUser
	exists bool
	err    error
}

var errAccountsDuplicate = errors.New("duplicate")

type accountsRemnawave struct {
	findResponses    []accountsFindResponse
	findIndex        int
	telegramResponse accountsFindResponse
	createRemote     RemoteUser
	createErr        error
	created          []RemoteCreateUser
	linkedResponse   accountsFindResponse
}

func (r *accountsRemnawave) FindUserByUsername(context.Context, string) (RemoteUser, bool, error) {
	if r.findIndex >= len(r.findResponses) {
		return RemoteUser{}, false, nil
	}
	response := r.findResponses[r.findIndex]
	r.findIndex++
	return response.remote, response.exists, response.err
}
func (r *accountsRemnawave) FindUserByTelegramID(context.Context, int64) (RemoteUser, bool, error) {
	return r.telegramResponse.remote, r.telegramResponse.exists, r.telegramResponse.err
}
func (r *accountsRemnawave) FindUserByID(context.Context, string) (RemoteUser, bool, error) {
	return r.linkedResponse.remote, r.linkedResponse.exists, r.linkedResponse.err
}
func (r *accountsRemnawave) CreateUser(_ context.Context, input RemoteCreateUser) (RemoteUser, error) {
	r.created = append(r.created, input)
	return r.createRemote, r.createErr
}
func (r *accountsRemnawave) IsDuplicateError(err error) bool {
	return errors.Is(err, errAccountsDuplicate)
}

type accountsSettings struct {
	values map[string]string
	errs   map[string]error
}

func (s *accountsSettings) Plaintext(_ context.Context, key string) (string, error) {
	if s.errs != nil && s.errs[key] != nil {
		return "", s.errs[key]
	}
	return s.values[key], nil
}

func validAccountsSettings() *accountsSettings {
	return &accountsSettings{values: map[string]string{"telegram.group_chat_id": "-1001", "telegram.channel_chat_id": "-1002", "telegram.webhook_secret": "test-invite-secret"}}
}

func newAccountsServiceForTest(repository Repository, validator InitDataValidator, telegram TelegramClient, remnawave RemnawaveClient, settings Settings, adminID int64) *Service {
	eligibility, _ := repository.(CommunityEligibility)
	service := NewService(repository, eligibility, validator, telegram, remnawave, settings, []int64{adminID}, 7*24*time.Hour)
	service.now = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }
	return service
}
