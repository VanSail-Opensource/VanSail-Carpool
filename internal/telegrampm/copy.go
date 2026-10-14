package telegrampm

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func chinese(locale string) bool { return strings.HasPrefix(strings.ToLower(locale), "zh") }

func NoticeText(item model.PMNotice) string {
	if chinese(item.Locale) {
		switch item.Reason {
		case "unsupported":
			return "Telegram \u65e0\u6cd5\u8f6c\u53d1\u8fd9\u7c7b\u6d88\u606f\uff0c\u8bf7\u6539\u7528\u6587\u5b57\u6216\u5e38\u89c4\u5a92\u4f53\u3002"
		case "entry":
			return "\u8bf7\u5148\u6253\u5f00 VanSail Carpool \u9762\u677f\u5e76\u5b8c\u6210\u9996\u6b21\u8bbf\u95ee\u9a8c\u8bc1\uff0c\u518d\u76f4\u63a5\u53d1\u9001\u6d88\u606f\u3002"
		case "blocked":
			return "\u4f60\u7684\u79c1\u4fe1\u5df2\u88ab\u7ba1\u7406\u5458\u6682\u505c\u3002"
		default:
			return "\u79c1\u4fe1\u529f\u80fd\u6682\u672a\u5f00\u653e\uff0c\u8bf7\u7a0d\u540e\u518d\u8bd5\u3002"
		}
	}
	switch item.Reason {
	case "unsupported":
		return "Telegram cannot relay this message type. Please send text or ordinary media instead."
	case "entry":
		return "Open the VanSail Carpool panel and complete entry verification before sending a message directly here."
	case "blocked":
		return "Private messaging for your account has been blocked by an administrator."
	default:
		return "Private messaging is currently unavailable. Please try again later."
	}
}

func callbackText(locale, reason string) string {
	if chinese(locale) {
		switch reason {
		case "updated":
			return "\u79c1\u4fe1\u8bbe\u7f6e\u5df2\u66f4\u65b0\u3002"
		case "refresh":
			return "\u8d44\u6599\u5361\u66f4\u65b0\u5df2\u52a0\u5165\u961f\u5217\u3002"
		default:
			return "\u4ec5\u9762\u677f\u7ba1\u7406\u5458\u53ef\u4f7f\u7528\u5f53\u524d\u8bdd\u9898\u7684\u8d44\u6599\u5361\u3002"
		}
	}
	switch reason {
	case "updated":
		return "Private messaging settings updated."
	case "refresh":
		return "Profile refresh queued."
	default:
		return "Only panel admins can use the current topic profile card."
	}
}

func TopicName(item model.PMConversation) string {
	name := strings.TrimSpace(item.FirstName + " " + item.LastName)
	if item.Username != "" {
		name = "@" + item.Username
	}
	name += " \u00b7 " + strconv.FormatInt(item.TelegramID, 10)
	if utf8.RuneCountInString(name) > 128 {
		name = string([]rune(name)[:128])
	}
	return name
}
