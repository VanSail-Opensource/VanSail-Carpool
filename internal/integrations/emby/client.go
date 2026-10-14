package emby

import (
	"context"
	"errors"
	"fmt"
	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 4 << 20

// APIError is a non-success response from Emby.

type APIError struct {
	HTTPStatus int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Emby API returned HTTP %d: %s", e.HTTPStatus, e.Message)
}

// Option configures an Emby client.

type Option func(*Client)

// WithHTTPClient replaces the bounded default HTTP client.

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// Client calls the exact Emby endpoints required by the account saga.

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

// NewClient validates configuration and creates an Emby API client.

func NewClient(rawBaseURL, token string, options ...Option) (*Client, error) {
	baseURL, err := parseBaseURL(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Emby base URL: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("Emby API token is empty")
	}
	client := &Client{baseURL: baseURL, token: token, httpClient: &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
	for _, option := range options {
		option(client)
	}
	return client, nil
}

// ListUsers returns all Emby users from GET /Users.

func (c *Client) ListUsers(ctx context.Context) ([]domain.RemoteUser, error) {
	var response []userDTO
	if err := c.do(ctx, http.MethodGet, "/Users", nil, &response); err != nil {
		return nil, err
	}
	users := make([]domain.RemoteUser, 0, len(response))
	for _, item := range response {
		user := mapUser(item)
		if err := validateID(user.ID); err != nil {
			return nil, errors.New("Emby user list response contains an invalid Id")
		}
		users = append(users, user)
	}
	return users, nil
}

// FindUserByName performs the documented list call and matches Name exactly.

func (c *Client) FindUserByName(ctx context.Context, name string) (domain.RemoteUser, bool, error) {
	if strings.TrimSpace(name) == "" {
		return domain.RemoteUser{}, false, errors.New("Emby user name is empty")
	}
	users, err := c.ListUsers(ctx)
	if err != nil {
		return domain.RemoteUser{}, false, err
	}
	for _, user := range users {
		if user.Name == name {
			return user, true, nil
		}
	}
	return domain.RemoteUser{}, false, nil
}

// CreateUser creates a named Emby user through POST /Users/New.

func (c *Client) CreateUser(ctx context.Context, name string) (domain.RemoteUser, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.RemoteUser{}, errors.New("Emby user name is empty")
	}
	var response userDTO
	if err := c.do(ctx, http.MethodPost, "/Users/New", createUserByName{Name: name}, &response); err != nil {
		return domain.RemoteUser{}, err
	}
	user := mapUser(response)
	if validateID(user.ID) != nil || user.Name != name {
		return domain.RemoteUser{}, errors.New("Emby create response has an unexpected identity")
	}
	return user, nil
}

// GetUser retrieves one exact Emby user through GET /Users/{Id}.

func (c *Client) GetUser(ctx context.Context, id string) (domain.RemoteUser, error) {
	if err := validateID(id); err != nil {
		return domain.RemoteUser{}, err
	}
	var response userDTO
	if err := c.do(ctx, http.MethodGet, "/Users/"+id, nil, &response); err != nil {
		return domain.RemoteUser{}, err
	}
	if response.Policy == nil {
		return domain.RemoteUser{}, errors.New("Emby user response is missing Policy")
	}
	user := mapUser(response)
	if user.ID != id {
		return domain.RemoteUser{}, errors.New("Emby user response identity does not match request")
	}
	return user, nil
}

// SetPassword posts the complete documented UpdateUserPassword body.

func (c *Client) SetPassword(ctx context.Context, id string, currentPassword, newPassword []byte) error {
	if err := validateID(id); err != nil {
		return err
	}
	if len(newPassword) == 0 {
		return errors.New("new Emby password is empty")
	}
	payload := updateUserPassword{
		ID: id, CurrentPassword: string(currentPassword), NewPassword: string(newPassword), ResetPassword: false,
	}
	return c.do(ctx, http.MethodPost, "/Users/"+id+"/Password", payload, nil)
}

// UpdatePolicy posts the complete fetched-and-overlaid Users.UserPolicy.

func (c *Client) UpdatePolicy(ctx context.Context, id string, policy domain.Policy) error {
	if err := validateID(id); err != nil {
		return err
	}
	if policy == nil {
		return errors.New("Emby policy is nil")
	}
	return c.do(ctx, http.MethodPost, "/Users/"+id+"/Policy", policy, nil)
}

// ListSelectableFolders returns GET /Library/SelectableMediaFolders.
