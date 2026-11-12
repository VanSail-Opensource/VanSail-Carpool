package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPMForumMethodsPreserveCopyReferencesAndProfileRouting(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		method := strings.TrimPrefix(r.URL.Path, "/bot123:token/")
		switch method {
		case "getChat":
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100123,"type":"supergroup","is_forum":true}}`))
		case "createForumTopic":
			if string(input["name"]) != `"Member"` {
				t.Error("missing topic name")
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_thread_id":50}}`))
		case "copyMessage":
			for _, key := range []string{"text", "caption", "parse_mode", "caption_entities"} {
				if _, exists := input[key]; exists {
					t.Errorf("copy overrides %s", key)
				}
			}
			if string(input["chat_id"]) != "-100123" || string(input["from_chat_id"]) != "42" || string(input["message_id"]) != "4" || string(input["message_thread_id"]) != "50" || string(input["disable_notification"]) != "true" {
				t.Errorf("copy refs=%v", input)
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":52}}`))
		case "sendMessage":
			if string(input["message_thread_id"]) != "50" || string(input["parse_mode"]) != `"MarkdownV2"` || input["reply_markup"] == nil {
				t.Error("profile loses routing or markup")
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":51,"message_thread_id":50,"chat":{"id":-100123,"type":"supergroup"}}}`))
		case "editMessageText":
			if input["message_thread_id"] != nil {
				t.Error("undocumented editMessageText thread field")
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":51,"message_thread_id":50,"chat":{"id":-100123,"type":"supergroup"}}}`))
		case "answerCallbackQuery":
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			t.Errorf("undocumented method %s", method)
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	client, err := NewClient("123:token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if chat, err := client.GetChat(ctx, -100123); err != nil || !chat.IsForum {
		t.Fatalf("chat=%+v,%v", chat, err)
	}
	if topic, err := client.CreateForumTopic(ctx, -100123, "Member"); err != nil || topic.MessageThreadID != 50 {
		t.Fatalf("topic=%+v,%v", topic, err)
	}
	if message, err := client.CopyMessage(ctx, CopyMessageRequest{ChatID: -100123, MessageThreadID: 50, FromChatID: 42, MessageID: 4, DisableNotification: true}); err != nil || message != 52 {
		t.Fatalf("copy=%d,%v", message, err)
	}
	profile := TopicProfileRequest{ChatID: -100123, MessageThreadID: 50, Text: "Profile", ReplyMarkup: InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{{Text: "Mute", CallbackData: "pm:mute:uuid"}}}}}
	if message, err := client.PublishTopicProfile(ctx, profile); err != nil || message != 51 {
		t.Fatalf("profile=%d,%v", message, err)
	}
	profile.MessageID = 51
	if message, err := client.EditTopicProfile(ctx, profile); err != nil || message.MessageThreadID != 50 {
		t.Fatalf("edited=%+v,%v", message, err)
	}
	if err := client.AnswerCallbackQuery(ctx, "callback", "updated", false); err != nil {
		t.Fatal(err)
	}
}

func TestTopicProfileEditPreservesContradictoryError(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusServiceUnavailable} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: message is not modified"}`))
		}))
		client, err := NewClient("123:token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.EditTopicProfile(context.Background(), TopicProfileRequest{ChatID: -100123, MessageID: 51, Text: "Profile"})
		server.Close()
		if (err == nil) != (status == http.StatusBadRequest) {
			t.Fatalf("HTTP%d unchanged-message classification: %v", status, err)
		}
	}
}
