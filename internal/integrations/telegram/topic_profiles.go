package telegram

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}
type TopicProfileRequest struct {
	ChatID              int64                `json:"chat_id"`
	MessageThreadID     int64                `json:"message_thread_id,omitempty"`
	MessageID           int64                `json:"message_id,omitempty"`
	Text                string               `json:"text"`
	ParseMode           string               `json:"parse_mode"`
	DisableNotification bool                 `json:"disable_notification,omitempty"`
	ReplyMarkup         InlineKeyboardMarkup `json:"reply_markup"`
}

func (c *Client) PublishTopicProfile(ctx context.Context, input TopicProfileRequest) (int64, error) {
	if input.ChatID >= 0 || input.MessageThreadID <= 1 || input.Text == "" {
		return 0, errors.New("invalid topic profile")
	}
	input.Text = telegramformat.Limit(input.Text)
	input.MessageID = 0
	input.ParseMode = "MarkdownV2"
	input.DisableNotification = true
	var message Message
	err := c.call(ctx, "sendMessage", input, &message)
	if err == nil && (message.MessageID <= 0 || message.Chat.ID != input.ChatID || message.MessageThreadID != input.MessageThreadID) {
		return 0, errors.New("Telegram returned an invalid profile message id")
	}
	return message.MessageID, err
}

func (c *Client) EditTopicProfile(ctx context.Context, input TopicProfileRequest) (Message, error) {
	if input.ChatID >= 0 || input.MessageID <= 0 || input.Text == "" {
		return Message{}, errors.New("invalid topic profile references")
	}
	// editMessageText has no message_thread_id or disable_notification parameters.
	request := struct {
		ChatID      int64                `json:"chat_id"`
		MessageID   int64                `json:"message_id"`
		Text        string               `json:"text"`
		ParseMode   string               `json:"parse_mode"`
		ReplyMarkup InlineKeyboardMarkup `json:"reply_markup"`
	}{input.ChatID, input.MessageID, telegramformat.Limit(input.Text), "MarkdownV2", input.ReplyMarkup}
	var result Message
	err := c.call(ctx, "editMessageText", request, &result)
	var apiError *APIError
	if errors.As(err, &apiError) && apiError.ErrorCode == 400 && strings.Contains(strings.ToLower(apiError.Description), "message is not modified") {
		return Message{}, nil
	}
	return result, err
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

func (c *Client) AnswerCallbackQuery(ctx context.Context, id, text string, alert bool) error {
	if id == "" {
		return errors.New("callback query id is required")
	}
	return c.booleanCall(ctx, "answerCallbackQuery", struct {
		ID    string `json:"callback_query_id"`
		Text  string `json:"text,omitempty"`
		Alert bool   `json:"show_alert"`
	}{id, text, alert})
}
