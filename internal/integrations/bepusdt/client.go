package bepusdt

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 2 << 20

// HTTPDoer is implemented by *http.Client and permits transport injection in tests.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// APIError describes a BEPusdt HTTP or application error. Message and RequestID
// are untrusted provider metadata and must not be exposed in receipts or logs.
type APIError struct {
	HTTPStatus int
	StatusCode int
	Message    string
	RequestID  string
}

// Error implements error.
func (e *APIError) Error() string {
	if e == nil {
		return "bepusdt API error"
	}
	return fmt.Sprintf("bepusdt API failed (http=%d api=%d)", e.HTTPStatus, e.StatusCode)
}

// Client calls a BEPusdt server.
type Client struct {
	baseURL    *url.URL
	token      string
	httpClient HTTPDoer
}

// Option configures a Client.
type Option func(*Client) error

// WithHTTPClient installs a custom HTTP transport. Production transports must retain TLS verification.
func WithHTTPClient(client HTTPDoer) Option {
	return func(c *Client) error {
		if client == nil {
			return errors.New("bepusdt HTTP client is nil")
		}
		c.httpClient = client
		return nil
	}
}

// NewClient creates a BEPusdt API client.
func NewClient(rawBaseURL, token string, options ...Option) (*Client, error) {
	rawBaseURL = strings.TrimSpace(rawBaseURL)
	token = strings.TrimSpace(token)
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("bepusdt parse base URL: %w", err)
	}
	if baseURL.Scheme != "https" && baseURL.Scheme != "http" {
		return nil, errors.New("bepusdt base URL must use http or https")
	}
	if baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, errors.New("bepusdt base URL must be absolute and contain no credentials, query, or fragment")
	}
	if token == "" {
		return nil, errors.New("bepusdt API token is empty")
	}
	c := &Client{baseURL: baseURL, token: token, httpClient: &http.Client{
		Timeout:       20 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(c); err != nil {
			return nil, fmt.Errorf("configure bepusdt client: %w", err)
		}
	}
	return c, nil
}
