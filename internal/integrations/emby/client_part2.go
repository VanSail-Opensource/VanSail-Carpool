package emby

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"io"
	"net/http"
	"strings"
)

func (c *Client) ListSelectableFolders(ctx context.Context) ([]domain.Folder, error) {
	var response []mediaFolder
	if err := c.do(ctx, http.MethodGet, "/Library/SelectableMediaFolders", nil, &response); err != nil {
		return nil, err
	}
	folders := make([]domain.Folder, 0, len(response))
	for _, folder := range response {
		if folder.ID != "" {
			folders = append(folders, domain.Folder{ID: folder.ID, Name: folder.Name})
		}
	}
	return folders, nil
}

// ListParentalRatings returns GET /Localization/ParentalRatings.

func (c *Client) ListParentalRatings(ctx context.Context) ([]domain.ParentalRating, error) {
	var response []parentalRating
	if err := c.do(ctx, http.MethodGet, "/Localization/ParentalRatings", nil, &response); err != nil {
		return nil, err
	}
	ratings := make([]domain.ParentalRating, 0, len(response))
	for _, rating := range response {
		ratings = append(ratings, domain.ParentalRating{Name: rating.Name, Value: rating.Value})
	}
	return ratings, nil
}

// IsNotFound reports an authoritative Emby HTTP 404.

func (c *Client) IsNotFound(err error) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.HTTPStatus == http.StatusNotFound
}

// IsTerminal classifies non-retryable client-side Emby responses.

func (c *Client) IsTerminal(err error) bool {
	var apiError *APIError
	if !errors.As(err, &apiError) {
		return false
	}
	return apiError.HTTPStatus >= 400 && apiError.HTTPStatus < 500 &&
		apiError.HTTPStatus != http.StatusRequestTimeout && apiError.HTTPStatus != http.StatusTooManyRequests
}

type userDTO struct {
	Name   string        `json:"Name"`
	ID     string        `json:"Id"`
	Policy domain.Policy `json:"Policy"`
}

type createUserByName struct {
	Name string `json:"Name"`
}

type updateUserPassword struct {
	ID              string `json:"Id"`
	CurrentPassword string `json:"CurrentPw"`
	NewPassword     string `json:"NewPw"`
	ResetPassword   bool   `json:"ResetPassword"`
}

type mediaFolder struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

type parentalRating struct {
	Name  string `json:"Name"`
	Value int32  `json:"Value"`
}

func mapUser(user userDTO) domain.RemoteUser {
	if user.Policy == nil {
		user.Policy = make(domain.Policy)
	}
	return domain.RemoteUser{ID: user.ID, Name: user.Name, Policy: user.Policy}
}

func (c *Client) do(ctx context.Context, method, endpoint string, input, output any) error {
	if ctx == nil {
		return errors.New("Emby request context is nil")
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode Emby request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	target := *c.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + endpoint
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return fmt.Errorf("create Emby request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Emby-Token", c.token)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("perform Emby request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read Emby response: %w", err)
	}
	if len(responseBody) > maxResponseBytes {
		return fmt.Errorf("Emby response exceeds %d bytes", maxResponseBytes)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(response.StatusCode, responseBody)
	}
	if output == nil || response.StatusCode == http.StatusNoContent || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, output); err != nil {
		return fmt.Errorf("decode Emby response: %w", err)
	}
	return nil
}

func decodeAPIError(status int, _ []byte) error {
	// Do not retain provider error bodies: a password validation response could
	// echo secret material and later be persisted in outbox/account diagnostics.
	return &APIError{HTTPStatus: status, Message: http.StatusText(status)}
}

var _ domain.Remote = (*Client)(nil)
