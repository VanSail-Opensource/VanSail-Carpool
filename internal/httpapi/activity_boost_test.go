package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestActivityBoostErrors(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{activity.ErrGroupBoostRequired, 403, "GROUP_BOOST_REQUIRED"},
		{activity.ErrGroupBoostUnavailable, 503, "GROUP_BOOST_UNAVAILABLE"},
	} {
		response := httptest.NewRecorder()
		(&Server{}).communityFailure(response, httptest.NewRequest("POST", "/api/v1/activity/check-ins", nil), test.err)
		var body apiError
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != test.status || body.Code != test.code {
			t.Fatalf("boost refusal = %d %+v", response.Code, body)
		}
	}
}

type httpBoostSource struct{}

func (httpBoostSource) GroupBoost(context.Context, string) (activity.GroupBoostStatus, error) {
	count := 0
	return activity.GroupBoostStatus{State: "required", Count: &count}, nil
}

type boostReplyCapture struct{ replies []string }

func (capture *boostReplyCapture) SendMarkdownV2Message(_ context.Context, _, _ int64, body string) error {
	capture.replies = append(capture.replies, body)
	return nil
}
func (*boostReplyCapture) AnswerPreCheckoutQuery(context.Context, string, bool, string) error {
	return nil
}

func TestHTTPAndTelegramCheckInRefuseButOrdinaryMessagesStaySilent(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "boost.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	store := database.NewStore(db)
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 42, FirstName: "Member"}, false)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"telegram.group_chat_id": "-100123456", "activity.group_message_threshold": "1", "activity.group_message_reward_txb": "1.25",
	} {
		if err := store.PutSetting(ctx, key, value, false, nil); err != nil {
			t.Fatal(err)
		}
	}
	service := activity.NewService(store, nil, nil)
	service.SetGroupBoostSource(httpBoostSource{})
	capture := &boostReplyCapture{}
	server := &Server{deps: Dependencies{
		Store: store, Activity: service, Telegram: capture, Settings: admin.NewSettingsService(store, nil),
		Affiliates: affiliates.NewService(store, nil), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}
	user.OnboardingState = "complete"
	request := communityRequest("POST", "/api/v1/activity/check-ins", user)
	request.Header.Set("Idempotency-Key", "boost-http-refusal")
	response := httptest.NewRecorder()
	server.activityCheckIn(response, request)
	if response.Code != 403 || !strings.Contains(response.Body.String(), "GROUP_BOOST_REQUIRED") {
		t.Fatalf("HTTP refusal: %d %s", response.Code, response.Body.String())
	}
	message := &telegram.Message{MessageID: 1, Chat: telegram.Chat{ID: -100123456, Type: "supergroup"}, From: &telegram.User{ID: 42}, Text: "/signin"}
	server.processTelegramGroupMessage(ctx, message)
	if len(capture.replies) != 1 || !strings.Contains(capture.replies[0], "not boosted the group yet") {
		t.Fatalf("command replies: %v", capture.replies)
	}
	message.MessageID, message.Text = 2, "An ordinary group message"
	server.processTelegramGroupMessage(ctx, message)
	if len(capture.replies) != 1 {
		t.Fatalf("ordinary message produced refusal: %v", capture.replies)
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.GroupMessageStatus(ctx, user.ID, activity.GroupMessageRewardConfig{Timezone: location.String(), Threshold: 1, RewardMinor: 125})
	if err != nil || status.MessageCount != 0 || status.Rewarded {
		t.Fatalf("unboosted progress: %+v, %v", status, err)
	}
	message.MessageID, message.Chat.ID, message.SenderBoostCount = 3, -100999999, 3
	server.processTelegramGroupMessage(ctx, message)
	var otherGroupEvents int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_group_message_events WHERE chat_id=-100999999`).Scan(&otherGroupEvents); err != nil || otherGroupEvents != 0 {
		t.Fatalf("other group events: %d %v", otherGroupEvents, err)
	}
}
