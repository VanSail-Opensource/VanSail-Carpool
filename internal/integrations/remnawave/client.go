package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 4 << 20

const remnawaveProxyIP = "127.0.0.1"

// HTTPDoer is implemented by *http.Client and permits transport injection in tests.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// APIError is a sanitized Remnawave API error.
type APIError struct {
	HTTPStatus int
	ErrorCode  string
	Message    string
	Issues     []ValidationIssue
}

// Error implements error.
func (e *APIError) Error() string {
	if e == nil {
		return "remnawave API error"
	}
	if e.ErrorCode != "" {
		return fmt.Sprintf("remnawave API failed (http=%d code=%s): %s", e.HTTPStatus, e.ErrorCode, e.Message)
	}
	return fmt.Sprintf("remnawave API failed (http=%d): %s", e.HTTPStatus, e.Message)
}

// IsErrorCode reports whether err is a Remnawave API error with code, such as A019.
func IsErrorCode(err error, code string) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.ErrorCode == code
}

// IsNotFound reports whether Remnawave returned HTTP 404.
func IsNotFound(err error) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.HTTPStatus == http.StatusNotFound
}

// Client calls the protected Remnawave v3.3.0 API using bearer authentication.
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
			return errors.New("remnawave HTTP client is nil")
		}
		c.httpClient = client
		return nil
	}
}

// NewClient creates a Remnawave client from the panel origin and API bearer token.
func NewClient(rawBaseURL, token string, options ...Option) (*Client, error) {
	baseURL, err := parseBaseURL(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("remnawave base URL: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("remnawave bearer token is empty")
	}
	c := &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(c); err != nil {
			return nil, fmt.Errorf("configure remnawave client: %w", err)
		}
	}
	return c, nil
}

func (c *Client) do(ctx context.Context, method, endpoint string, query url.Values, input, output any) error {
	if ctx == nil {
		return errors.New("remnawave request context is nil")
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("remnawave encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	target := *c.baseURL
	target.Path = strings.TrimRight(target.Path, "/") + endpoint
	if query != nil {
		target.RawQuery = query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return fmt.Errorf("remnawave create request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("X-Real-IP", remnawaveProxyIP)
	request.Header.Set("X-Forwarded-For", remnawaveProxyIP)
	request.Header.Set("X-Forwarded-Proto", "https")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("remnawave request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	responseLimit := maxResponseBytes
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, int64(responseLimit)+1))
	if err != nil {
		return fmt.Errorf("remnawave read response: %w", err)
	}
	if len(responseBody) > responseLimit {
		return fmt.Errorf("remnawave response exceeds %d bytes", responseLimit)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(response.StatusCode, responseBody)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.Unmarshal(responseBody, output); err != nil {
		return fmt.Errorf("remnawave decode response (http=%d): %w", response.StatusCode, err)
	}
	return nil
}

func decodeAPIError(status int, body []byte) error {
	var payload struct {
		ErrorCode  string            `json:"errorCode"`
		Message    string            `json:"message"`
		StatusCode int               `json:"statusCode"`
		Errors     []ValidationIssue `json:"errors"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return &APIError{HTTPStatus: status, Message: http.StatusText(status)}
	}
	if payload.Message == "" {
		payload.Message = http.StatusText(status)
	}
	return &APIError{HTTPStatus: status, ErrorCode: payload.ErrorCode, Message: payload.Message, Issues: payload.Errors}
}

func parseBaseURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, errors.New("URL must use http or https")
	}
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("URL must be absolute and contain no credentials, query, or fragment")
	}
	return u, nil
}
