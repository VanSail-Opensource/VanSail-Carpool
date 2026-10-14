package telegrampm

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

func TestAttributionPreservesEntitiesAndUsesMinimalPendingContent(t *testing.T) {
	admin := &telegram.User{ID: 99, Username: "admin_ops"}
	entity := telegram.MessageEntity{Type: "bold", Offset: 0, Length: 2}
	for _, tc := range []struct {
		name    string
		message telegram.Message
		mode    string
		reply   bool
	}{
		{"text", telegram.Message{From: admin, Text: "Hi 👋", Entities: []telegram.MessageEntity{entity}}, "text", false},
		{"caption", telegram.Message{From: admin, Photo: json.RawMessage(`[{"file_id":"private-media"}]`), Caption: "Hi 👋", CaptionEntities: []telegram.MessageEntity{entity}, ShowCaptionAboveMedia: true}, "caption", false},
		{"text overflow", telegram.Message{From: admin, Text: strings.Repeat("x", 4096)}, "copy-footer", true},
		{"caption overflow", telegram.Message{From: admin, Video: json.RawMessage(`{"file_id":"private-media"}`), Caption: strings.Repeat("x", 1024)}, "copy-footer", true},
		{"sticker", telegram.Message{From: admin, Sticker: json.RawMessage(`{"file_id":"private-media"}`)}, "copy-footer", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, reply, err := prepareContent(&tc.message)
			if err != nil {
				t.Fatal(err)
			}
			var c OutboundContent
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatal(err)
			}
			if c.Mode != tc.mode || reply != tc.reply || c.Footer != "By @admin_ops" {
				t.Fatalf("%+v reply=%v", c, reply)
			}
			if strings.Contains(string(data), "private-media") {
				t.Fatal("stored media identifier")
			}
			if c.Mode == "text" && (c.Text != tc.message.Text || len(c.Entities) != 1 || c.Entities[0] != entity) {
				t.Fatal("lost text/entity offsets")
			}
			if c.Mode == "caption" && (!c.ShowCaptionAboveMedia || c.Caption != tc.message.Caption || len(c.CaptionEntities) != 1) {
				t.Fatal("lost caption formatting")
			}
			if c.Mode == "copy-footer" && (c.Text != "" || c.Caption != "") {
				t.Fatal("unnecessary overflow body persisted")
			}
		})
	}
	for _, tc := range []struct {
		user   telegram.User
		footer string
	}{{telegram.User{ID: 99, FirstName: "Casey", LastName: "Taylor"}, "By Casey Taylor"}, {telegram.User{ID: 99}, "By 99"}} {
		if adminFooter(tc.user) != tc.footer {
			t.Fatal("missing username fallback")
		}
	}
}
