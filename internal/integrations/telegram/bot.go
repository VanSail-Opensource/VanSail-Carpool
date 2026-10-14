package telegram

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

const telegramMessageLimit = 4096

// GetMe returns the authenticated bot identity used to construct referral links.
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var user User
	if err := c.call(ctx, "getMe", struct{}{}, &user); err != nil {
		return User{}, err
	}
	if !user.IsBot || user.ID <= 0 || strings.TrimSpace(user.Username) == "" {
		return User{}, errors.New("telegram getMe returned an invalid bot identity")
	}
	return user, nil
}

// BotCommand is one command advertised by Telegram's command menu.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// BotCommandScope selects the chats receiving a command menu.
type BotCommandScope struct {
	Type   string `json:"type"`
	ChatID int64  `json:"chat_id,omitempty"`
}

// SetMyCommands installs one localized command list for a Bot API scope.
func (c *Client) SetMyCommands(ctx context.Context, commands []BotCommand, scope BotCommandScope, languageCode string) error {
	if len(commands) == 0 || len(commands) > 100 {
		return errors.New("telegram command list must contain 1-100 commands")
	}
	if scope.Type != "all_private_chats" && scope.Type != "chat" {
		return errors.New("telegram command scope is invalid")
	}
	if scope.Type == "chat" && scope.ChatID == 0 {
		return errors.New("telegram chat command scope requires a chat id")
	}
	for _, command := range commands {
		if !validCommand(command.Command) || strings.TrimSpace(command.Description) == "" || utf8.RuneCountInString(command.Description) > 256 {
			return errors.New("telegram command contains an invalid name or description")
		}
	}
	payload := struct {
		Commands     []BotCommand    `json:"commands"`
		Scope        BotCommandScope `json:"scope"`
		LanguageCode string          `json:"language_code,omitempty"`
	}{Commands: commands, Scope: scope, LanguageCode: strings.TrimSpace(languageCode)}
	return c.booleanCall(ctx, "setMyCommands", payload)
}

// SendMessage sends a bounded plain-text reply to a Telegram message.
func (c *Client) SendMessage(ctx context.Context, chatID, replyToMessageID int64, text string) error {
	_, err := c.sendMessage(ctx, chatID, replyToMessageID, text, "")
	return err
}

// SendMarkdownV2Message sends a bounded MarkdownV2-formatted Telegram message.
func (c *Client) SendMarkdownV2Message(ctx context.Context, chatID, replyToMessageID int64, text string) error {
	_, err := c.sendMessage(ctx, chatID, replyToMessageID, telegramformat.Limit(text), "MarkdownV2")
	return err
}

// PublishMarkdownV2Message returns the Telegram message ID for subsequent edits.
func (c *Client) PublishMarkdownV2Message(ctx context.Context, chatID int64, text string) (int64, error) {
	return c.sendMessage(ctx, chatID, 0, telegramformat.Limit(text), "MarkdownV2")
}

// ReplyMarkdownV2Message returns the new reply's message ID.
func (c *Client) ReplyMarkdownV2Message(ctx context.Context, chatID, replyID int64, text string) (int64, error) {
	return c.sendMessage(ctx, chatID, replyID, telegramformat.Limit(text), "MarkdownV2")
}

func (c *Client) sendMessage(ctx context.Context, chatID, replyToMessageID int64, text, parseMode string) (int64, error) {
	text = strings.TrimSpace(text)
	if chatID == 0 || text == "" || utf8.RuneCountInString(text) > telegramMessageLimit {
		return 0, errors.New("telegram reply requires a chat id and 1-4096 characters")
	}
	payload := struct {
		ChatID          int64  `json:"chat_id"`
		Text            string `json:"text"`
		ParseMode       string `json:"parse_mode,omitempty"`
		ReplyParameters *struct {
			MessageID                int64 `json:"message_id"`
			AllowSendingWithoutReply bool  `json:"allow_sending_without_reply"`
		} `json:"reply_parameters,omitempty"`
	}{ChatID: chatID, Text: text, ParseMode: parseMode}
	if replyToMessageID > 0 {
		payload.ReplyParameters = &struct {
			MessageID                int64 `json:"message_id"`
			AllowSendingWithoutReply bool  `json:"allow_sending_without_reply"`
		}{MessageID: replyToMessageID, AllowSendingWithoutReply: true}
	}
	var result struct {
		MessageID int64 `json:"message_id"`
	}
	if err := c.call(ctx, "sendMessage", payload, &result); err != nil {
		if parseMode != "MarkdownV2" || !isEntityParseError(err) {
			return 0, err
		}
		payload.Text = telegramformat.PlainText(text)
		payload.ParseMode = ""
		if err = c.call(ctx, "sendMessage", payload, &result); err != nil {
			return 0, err
		}
	}
	if result.MessageID <= 0 {
		return 0, errors.New("telegram sendMessage returned an invalid message")
	}
	return result.MessageID, nil
}

// EditMarkdownV2Message updates an outgoing bot message in place.
func (c *Client) EditMarkdownV2Message(ctx context.Context, chatID, messageID int64, text string) error {
	if chatID == 0 || messageID <= 0 || strings.TrimSpace(text) == "" {
		return errors.New("invalid Telegram edit target")
	}
	payload := struct {
		ChatID    int64  `json:"chat_id"`
		MessageID int64  `json:"message_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}{
		chatID, messageID, telegramformat.Limit(text), "MarkdownV2",
	}
	var result Message
	if err := c.call(ctx, "editMessageText", payload, &result); err != nil {
		if !isEntityParseError(err) {
			return err
		}
		payload.Text = telegramformat.PlainText(payload.Text)
		payload.ParseMode = ""
		return c.call(ctx, "editMessageText", payload, &result)
	}
	return nil
}

func isEntityParseError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode != 400 {
		return false
	}
	description := strings.ToLower(apiErr.Description)
	return strings.Contains(description, "can't parse entities") ||
		strings.Contains(description, "reserved and must be escaped") ||
		strings.Contains(description, "can't find end of the entity")
}

// DeleteMessage removes an outgoing message while Telegram permits deletion.
func (c *Client) DeleteMessage(ctx context.Context, chatID, messageID int64) error {
	if chatID == 0 || messageID <= 0 {
		return errors.New("invalid Telegram delete target")
	}
	return c.booleanCall(ctx, "deleteMessage", struct {
		ChatID    int64 `json:"chat_id"`
		MessageID int64 `json:"message_id"`
	}{chatID, messageID})
}

func validCommand(value string) bool {
	if len(value) < 1 || len(value) > 32 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' {
			continue
		}
		return false
	}
	return true
}
