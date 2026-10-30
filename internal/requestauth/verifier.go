package requestauth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

const (
	defaultWindow = 5 * time.Minute
	// Bound signed request bodies; format-specific handlers apply tighter limits.
	defaultMaxBody   = int64(6 << 20)
	defaultMaxNonces = 512
	masterKeyBytes   = 32
)

var (
	timestampPattern = regexp.MustCompile(`^[0-9]{10,11}$`)
	noncePattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{22,64}$`)
	signaturePattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	clientKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	sessionPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

	// ErrRequired marks a request missing one or more signing values.
	ErrRequired = errors.New("request signature is required")
	// ErrMalformed marks a signing value outside its strict grammar.
	ErrMalformed = errors.New("request signature is malformed")
	// ErrStale marks a timestamp outside the five-minute acceptance window.
	ErrStale = errors.New("request signature timestamp is stale")
	// ErrInvalid marks a key or signature that does not authenticate.
	ErrInvalid = errors.New("request signature is invalid")
	// ErrReplay marks a nonce already accepted for the same session.
	ErrReplay = errors.New("request signature nonce was replayed")
	// ErrRateLimited marks a session that exhausted its bounded nonce window.
	ErrRateLimited = errors.New("request signature rate limit exceeded")
	// ErrBodyTooLarge marks a body beyond the signed-request limit.
	ErrBodyTooLarge = errors.New("request body is too large")
)

// Verifier derives per-session client keys and verifies signed requests.
type Verifier struct {
	master  []byte
	window  time.Duration
	maxBody int64
	replays *replayCache
	now     func() time.Time
}

// New constructs a verifier from the configured 32-byte server master key.
func New(masterKey []byte) (*Verifier, error) {
	if len(masterKey) != masterKeyBytes {
		return nil, fmt.Errorf("request signing master key must contain %d bytes", masterKeyBytes)
	}
	return &Verifier{
		master: append([]byte(nil), masterKey...), window: defaultWindow,
		maxBody: defaultMaxBody, replays: newReplayCache(defaultMaxNonces), now: time.Now,
	}, nil
}

// ClientKey returns the same-origin companion-cookie value for a session token.
func (verifier *Verifier) ClientKey(sessionToken string) (string, error) {
	if verifier == nil || !sessionPattern.MatchString(sessionToken) {
		return "", ErrMalformed
	}
	return base64.RawURLEncoding.EncodeToString(deriveClientKey(verifier.master, sessionToken)), nil
}

// Verify authenticates a request, rejects replay, and restores the exact body.
func (verifier *Verifier) Verify(request *http.Request, sessionToken, clientKey string) error {
	if verifier == nil || request == nil {
		return ErrRequired
	}
	timestamp := request.Header.Get(TimestampHeader)
	nonce := request.Header.Get(NonceHeader)
	signature := request.Header.Get(SignatureHeader)
	if sessionToken == "" || clientKey == "" || timestamp == "" || nonce == "" || signature == "" {
		return ErrRequired
	}
	if !sessionPattern.MatchString(sessionToken) || !clientKeyPattern.MatchString(clientKey) ||
		!timestampPattern.MatchString(timestamp) || !noncePattern.MatchString(nonce) || !signaturePattern.MatchString(signature) {
		return ErrMalformed
	}
	signedAtUnix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || !fresh(verifier.now().Unix(), signedAtUnix, int64(verifier.window/time.Second)) {
		return ErrStale
	}
	expectedKey := deriveClientKey(verifier.master, sessionToken)
	providedKey, err := base64.RawURLEncoding.DecodeString(clientKey)
	if err != nil || !hmac.Equal(providedKey, expectedKey) {
		return ErrInvalid
	}
	body, err := readAndRestore(request, verifier.maxBody)
	if err != nil {
		return err
	}
	wantSignature, err := Sign(clientKey, request.Method, CanonicalTarget(request.URL), timestamp, nonce, body)
	if err != nil {
		return ErrInvalid
	}
	if !hmac.Equal([]byte(signature), []byte(wantSignature)) {
		return ErrInvalid
	}
	sessionDigest := sha256.Sum256([]byte(sessionToken))
	expiresAt := time.Unix(signedAtUnix, 0).Add(verifier.window)
	if err := verifier.replays.add(string(sessionDigest[:]), nonce, verifier.now(), expiresAt); err != nil {
		return err
	}
	return nil
}

func fresh(now, signedAt, windowSeconds int64) bool {
	difference := now - signedAt
	return difference >= -windowSeconds && difference < windowSeconds
}

func readAndRestore(request *http.Request, maxBytes int64) ([]byte, error) {
	if request.ContentLength > maxBytes {
		return nil, ErrBodyTooLarge
	}
	if request.Body == nil {
		request.Body = http.NoBody
		return []byte{}, nil
	}
	body, readErr := io.ReadAll(io.LimitReader(request.Body, maxBytes+1))
	closeErr := request.Body.Close()
	request.Body = io.NopCloser(bytes.NewReader(body))
	if readErr != nil {
		return nil, fmt.Errorf("read signed request body: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close signed request body: %w", closeErr)
	}
	if int64(len(body)) > maxBytes {
		return nil, ErrBodyTooLarge
	}
	return body, nil
}
