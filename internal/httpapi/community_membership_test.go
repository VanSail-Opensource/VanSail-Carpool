package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/go-chi/chi/v5"
)

var errCommunityTransport = errors.New("telegram unavailable")

type communityHTTPRepository struct {
	user   model.User
	active bool
}

func (r *communityHTTPRepository) UpsertTelegramUser(context.Context, model.TelegramProfile, bool) (model.User, bool, error) {
	return r.user, false, nil
}
func (r *communityHTTPRepository) ReplaceSession(context.Context, []byte, string, time.Time) error {
	return nil
}
func (r *communityHTTPRepository) UserBySession(context.Context, []byte, time.Time) (model.User, error) {
	return r.user, nil
}
func (r *communityHTTPRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, nil
}
func (r *communityHTTPRepository) UserByTelegramID(context.Context, int64) (model.User, error) {
	return r.user, nil
}
func (r *communityHTTPRepository) UpdateMembership(_ context.Context, _ string, group, channel bool) (model.User, error) {
	r.user.GroupJoined, r.user.ChannelJoined = group, channel
	return r.user, nil
}
func (r *communityHTTPRepository) ReserveUsername(context.Context, string, string) error { return nil }
func (r *communityHTTPRepository) CurrentAgreementContract(context.Context) (int, []string, error) {
	return 1, nil, nil
}
func (r *communityHTTPRepository) CompleteOnboardingRevision(context.Context, string, int, []string, time.Time) (model.User, error) {
	return r.user, nil
}
func (r *communityHTTPRepository) HasActiveCombo(context.Context, string, time.Time) (bool, error) {
	return r.active, nil
}

type communityHTTPTelegram struct {
	joined    bool
	checkErr  error
	createErr error
}

func (t *communityHTTPTelegram) CreateJoinRequestInvite(context.Context, string, string, time.Time) (string, error) {
	return "https://t.me/+community", t.createErr
}
func (t *communityHTTPTelegram) GetMembership(context.Context, string, int64) (bool, error) {
	return t.joined, t.checkErr
}
func (t *communityHTTPTelegram) ApproveJoinRequest(context.Context, string, int64) error { return nil }
func (t *communityHTTPTelegram) DeclineJoinRequest(context.Context, string, int64) error { return nil }
func (t *communityHTTPTelegram) RevokeInviteLink(context.Context, string, string) error  { return nil }

type communityHTTPProxy struct{}

func (communityHTTPProxy) FindUserByUsername(context.Context, string) (accounts.RemoteUser, bool, error) {
	return accounts.RemoteUser{}, false, nil
}
func (communityHTTPProxy) FindUserByTelegramID(context.Context, int64) (accounts.RemoteUser, bool, error) {
	return accounts.RemoteUser{}, false, nil
}
func (communityHTTPProxy) CreateUser(context.Context, accounts.RemoteCreateUser) (accounts.RemoteUser, error) {
	return accounts.RemoteUser{}, nil
}
func (communityHTTPProxy) IsDuplicateError(error) bool { return false }

type communityHTTPSettings struct{}

func (communityHTTPSettings) Plaintext(_ context.Context, key string) (string, error) {
	return map[string]string{"telegram.group_chat_id": "-1001", "telegram.channel_chat_id": "-1002", "telegram.webhook_secret": "community-secret"}[key], nil
}

func communityMembershipServer(repository *communityHTTPRepository, telegram *communityHTTPTelegram) *Server {
	service := accounts.NewService(repository, repository, communityHTTPValidator{}, telegram, communityHTTPProxy{}, communityHTTPSettings{}, []int64{99}, time.Hour)
	return &Server{deps: Dependencies{Accounts: service}}
}

type communityHTTPValidator struct{}

func (communityHTTPValidator) Validate(string) (model.TelegramProfile, error) {
	return model.TelegramProfile{}, nil
}

func communityRequest(method, path string, user model.User) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	return request.WithContext(context.WithValue(request.Context(), userContextKey, user))
}

func communityInviteRequest(user model.User) *http.Request {
	request := communityRequest(http.MethodPost, "/api/v1/community/invites/group", user)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("kind", "group")
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func TestCommunityMembershipTransportErrors(t *testing.T) {
	t.Parallel()
	completed := model.User{ID: "user-1", TelegramID: 42, OnboardingState: "complete"}
	tests := []struct {
		name       string
		user       model.User
		active     bool
		joined     bool
		checkErr   error
		createErr  error
		membership bool
		wantStatus int
		wantCode   string
	}{
		{name: "onboarding required", user: model.User{OnboardingState: "agreement"}, membership: true, wantStatus: http.StatusConflict, wantCode: "ONBOARDING_REQUIRED"},
		{name: "membership check failure", user: completed, checkErr: errCommunityTransport, membership: true, wantStatus: http.StatusBadGateway, wantCode: "MEMBERSHIP_CHECK_FAILED"},
		{name: "active combo required", user: completed, wantStatus: http.StatusConflict, wantCode: "ACTIVE_COMBO_REQUIRED"},
		{name: "already joined", user: completed, active: true, joined: true, wantStatus: http.StatusConflict, wantCode: "COMMUNITY_ALREADY_JOINED"},
		{name: "invite provider unavailable", user: completed, active: true, createErr: errCommunityTransport, wantStatus: http.StatusServiceUnavailable, wantCode: "INVITES_UNAVAILABLE"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			repository := &communityHTTPRepository{user: test.user, active: test.active}
			telegram := &communityHTTPTelegram{joined: test.joined, checkErr: test.checkErr, createErr: test.createErr}
			server := communityMembershipServer(repository, telegram)
			response := httptest.NewRecorder()
			if test.membership {
				server.communityMembershipCheck(response, communityRequest(http.MethodPost, "/api/v1/community/membership/check", test.user))
			} else {
				server.createCommunityInvite(response, communityInviteRequest(test.user))
			}
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			var body apiError
			if err := decodeJSON(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", response.Body), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != test.wantCode {
				t.Fatalf("error code = %q, want %q", body.Code, test.wantCode)
			}
		})
	}
}

func TestCommunityAccessRequiresOnboardingAndUsesCurrentCombo(t *testing.T) {
	t.Parallel()
	completed := model.User{ID: "user-1", TelegramID: 42, OnboardingState: "complete"}
	for _, test := range []struct {
		name       string
		user       model.User
		active     bool
		wantStatus int
		wantActive bool
	}{
		{name: "onboarding required", user: model.User{OnboardingState: "agreement"}, wantStatus: http.StatusConflict},
		{name: "active combo", user: completed, active: true, wantStatus: http.StatusOK, wantActive: true},
		{name: "no active combo", user: completed, wantStatus: http.StatusOK},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &communityHTTPRepository{user: test.user, active: test.active}
			server := communityMembershipServer(repository, &communityHTTPTelegram{})
			response := httptest.NewRecorder()
			server.communityAccess(response, communityRequest(http.MethodGet, "/api/v1/community/membership/check", test.user))
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Code != http.StatusOK {
				return
			}
			var body communityAccessResponse
			if err := decodeJSON(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", response.Body), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.ActiveCombo != test.wantActive {
				t.Fatalf("active combo = %t, want %t", body.ActiveCombo, test.wantActive)
			}
		})
	}
}
