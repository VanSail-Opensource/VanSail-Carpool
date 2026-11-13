package requestauth

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type advancingRequestBody struct {
	io.Reader
	advance func()
}

func (body *advancingRequestBody) Read(value []byte) (int, error) {
	body.advance()
	return body.Reader.Read(value)
}
func (*advancingRequestBody) Close() error { return nil }

func TestRequestBodyReadCannotOutliveSignatureAndReplayReservation(t *testing.T) {
	t.Parallel()
	for _, previousUse := range []bool{false, true} {
		name := "first use"
		if previousUse {
			name = "replay crossing expiration"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			verifier := newTestVerifier(t)
			signedAt := verifier.now()
			current := signedAt
			verifier.now = func() time.Time { return current }
			key, err := verifier.ClientKey(testSessionToken)
			if err != nil {
				t.Fatal(err)
			}
			const payload = `{"amount":"100"}`
			const nonce = "AAAAAAAAAAAAAAAAAAAAAA"
			if previousUse {
				first := signedRequest(t, verifier, "POST", "/api/v1/purchases", payload, nonce, signedAt)
				if err := verifier.Verify(first, testSessionToken, key); err != nil {
					t.Fatal(err)
				}
			}
			current = signedAt.Add(defaultWindow - time.Nanosecond)
			request := signedRequest(t, verifier, "POST", "/api/v1/purchases", payload, nonce, signedAt)
			request.Body = &advancingRequestBody{Reader: strings.NewReader(payload), advance: func() {
				current = signedAt.Add(defaultWindow)
			}}
			if err := verifier.Verify(request, testSessionToken, key); !errors.Is(err, ErrStale) {
				t.Fatal("expired signature entered replay cache after delayed body read", err)
			}
		})
	}
}
