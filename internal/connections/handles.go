package connections

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"
	"unicode"
)

// ErrInvalidHandle hides whether a capability was malformed, expired, or forged.
var ErrInvalidHandle = errors.New("invalid connection handle")

// Signer creates and verifies short-lived connection drop capabilities.
type Signer struct {
	key []byte
}

type handlePayload struct {
	UserID   string `json:"u"`
	ScanID   string `json:"s"`
	NodeUUID string `json:"n"`
	IP       string `json:"i"`
	Expires  int64  `json:"e"`
}

// NewSigner creates a handle signer from application key material.
func NewSigner(key []byte) (*Signer, error) {
	if len(key) < 32 {
		return nil, errors.New("connection signing key must contain at least 32 bytes")
	}
	return &Signer{key: append([]byte(nil), key...)}, nil
}

// Sign binds the owner, scan, node, IP, and expiry into one opaque handle.
func (s *Signer) Sign(claims HandleClaims) (string, error) {
	if s == nil || !validClaims(claims) {
		return "", ErrInvalidHandle
	}
	payload, err := json.Marshal(handlePayload{
		UserID: claims.UserID, ScanID: claims.ScanID, NodeUUID: claims.NodeUUID,
		IP: claims.IP, Expires: claims.Expires.UTC().Unix(),
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(s.mac([]byte(encoded))), nil
}

// Verify validates one handle for an expected owner and current time.
func (s *Signer) Verify(handle, expectedUserID string, now time.Time) (HandleClaims, error) {
	parts := strings.Split(handle, ".")
	if s == nil || len(parts) != 2 || len(handle) > 4096 {
		return HandleClaims{}, ErrInvalidHandle
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, s.mac([]byte(parts[0]))) {
		return HandleClaims{}, ErrInvalidHandle
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return HandleClaims{}, ErrInvalidHandle
	}
	var payload handlePayload
	if json.Unmarshal(raw, &payload) != nil {
		return HandleClaims{}, ErrInvalidHandle
	}
	claims := HandleClaims{UserID: payload.UserID, ScanID: payload.ScanID, NodeUUID: payload.NodeUUID, IP: payload.IP, Expires: time.Unix(payload.Expires, 0).UTC()}
	if payload.UserID != expectedUserID || !validClaims(claims) || !now.UTC().Before(claims.Expires) {
		return HandleClaims{}, ErrInvalidHandle
	}
	return claims, nil
}

func (s *Signer) mac(value []byte) []byte {
	digest := hmac.New(sha256.New, s.key)
	_, _ = digest.Write(value)
	return digest.Sum(nil)
}

func validClaims(claims HandleClaims) bool {
	if strings.TrimSpace(claims.UserID) == "" || strings.TrimSpace(claims.ScanID) == "" || claims.Expires.IsZero() {
		return false
	}
	if claims.NodeUUID == "" || len(claims.NodeUUID) > 256 || strings.ContainsFunc(claims.NodeUUID, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false
	}
	return net.ParseIP(claims.IP) != nil
}
