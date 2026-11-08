package telegram

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

const (
	ReachabilityUnknown     = "unknown"
	ReachabilityReachable   = "reachable"
	ReachabilityBlocked     = "blocked"
	ReachabilityDeactivated = "deactivated"
	ReachabilityUnreachable = "unreachable"
	ReachabilityPrivate     = "private"
)

// SendChatAction sends only the standard typing action to a numeric private
// identity. It is reachability evidence, never proof that an account exists.
func (c *Client) SendChatAction(ctx context.Context, chatID int64) (bool, error) {
	if chatID <= 0 || chatID > 1<<52-1 {
		return false, errors.New("invalid private chat id")
	}
	var result bool
	err := c.call(ctx, "sendChatAction", struct {
		ChatID int64  `json:"chat_id"`
		Action string `json:"action"`
	}{chatID, "typing"}, &result)
	return result, err
}

func ClassifyChatAction(success bool, err error) string {
	if success && err != nil {
		return ReachabilityUnknown
	}
	if err == nil {
		if success {
			return ReachabilityReachable
		}
		return ReachabilityUnknown
	}
	return classifyReachabilityFailure("sendChatAction", err)
}

func ClassifyPrivateChat(expectedID int64, chat ChatInfo, err error) string {
	if err == nil {
		if expectedID > 0 && expectedID <= 1<<52-1 && chat.ID == expectedID && chat.Type == "private" {
			return ReachabilityPrivate
		}
		return ReachabilityUnknown
	}
	if chat.ID != 0 || chat.Type != "" {
		return ReachabilityUnknown
	}
	return classifyReachabilityFailure("getChat", err)
}

func classifyReachabilityFailure(method string, err error) string {
	var api *APIError
	if !errors.As(err, &api) || api == nil || api.Method != method || api.HTTPStatus != api.ErrorCode || api.RetryAfter != 0 {
		return ReachabilityUnknown
	}
	description := strings.ToLower(strings.TrimSpace(api.Description))
	if api.ErrorCode == http.StatusForbidden {
		switch description {
		case "forbidden: bot was blocked by the user":
			if method == "sendChatAction" {
				return ReachabilityBlocked
			}
		case "forbidden: user is deactivated":
			return ReachabilityDeactivated
		case "forbidden: bot can't initiate conversation with a user":
			return ReachabilityUnreachable
		}
	}
	if api.ErrorCode == http.StatusBadRequest && description == "bad request: chat not found" {
		return ReachabilityUnreachable
	}
	return ReachabilityUnknown
}

// BotIdentityUncertain identifies configuration/authentication failures only.
// Transport errors, throttling and unrecognized messages remain unknown probes.
func BotIdentityUncertain(err error) bool {
	var api *APIError
	return errors.As(err, &api) && api != nil && (api.Method == "sendChatAction" || api.Method == "getChat") && api.HTTPStatus == api.ErrorCode && (api.ErrorCode == http.StatusUnauthorized || api.ErrorCode == http.StatusNotFound)
}
