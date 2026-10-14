package telegrampm

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

// OutboundContent contains only the delivery data unavailable through message references.
type OutboundContent struct {
	Mode                  string                   `json:"mode"`
	Text                  string                   `json:"text,omitempty"`
	Caption               string                   `json:"caption,omitempty"`
	Entities              []telegram.MessageEntity `json:"entities,omitempty"`
	CaptionEntities       []telegram.MessageEntity `json:"captionEntities,omitempty"`
	Footer                string                   `json:"footer"`
	ShowCaptionAboveMedia bool                     `json:"showCaptionAboveMedia,omitempty"`
}

func adminFooter(user telegram.User) string {
	name := strings.TrimSpace(user.Username)
	if name != "" {
		name = "@" + strings.TrimPrefix(name, "@")
	} else {
		name = strings.Join(strings.Fields(user.FirstName+" "+user.LastName), " ")
	}
	if name == "" {
		name = strconv.FormatInt(user.ID, 10)
	}
	return "By " + name
}

func prepareContent(message *telegram.Message) (json.RawMessage, bool, error) {
	c := OutboundContent{Mode: "copy-footer", Footer: adminFooter(*message.From)}
	if message.Text != "" && telegram.UTF16Length(message.Text+"\n\n"+c.Footer) <= 4096 {
		c.Mode, c.Text, c.Entities = "text", message.Text, message.Entities
	} else if captionCapable(message) && telegram.UTF16Length(message.Caption+"\n\n"+c.Footer) <= 1024 {
		c.Mode, c.Caption, c.CaptionEntities = "caption", message.Caption, message.CaptionEntities
		c.ShowCaptionAboveMedia = message.ShowCaptionAboveMedia
	}
	// Overflow and non-caption media need only the footer; the original stays in Telegram.
	data, err := json.Marshal(c)
	return data, c.Mode == "copy-footer", err
}

func captionCapable(m *telegram.Message) bool {
	for _, raw := range []json.RawMessage{m.Photo, m.Video, m.Audio, m.Voice, m.Document, m.Animation, m.LivePhoto} {
		if len(raw) > 0 && string(raw) != "null" && string(raw) != "[]" {
			return true
		}
	}
	return false
}
