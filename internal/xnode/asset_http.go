package xnode

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

func publicAssetClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: publicTransport(),
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return validatePublicAssetRedirect(request, via)
		},
	}
}

func validatePublicAssetRedirect(request *http.Request, via []*http.Request) error {
	if len(via) > 3 || !validHTTPS(request.URL.String()) {
		return ErrInvalid
	}
	return nil
}

func assetRequestFailure(name string, err error) error {
	if errors.Is(err, ErrInvalid) {
		return fmt.Errorf("%s redirect or destination is invalid: %w", name, ErrInvalid)
	}
	return fmt.Errorf("%w: %s request failed", ErrUnavailable, name)
}

func assetResponseFailure(name string, status int) error {
	return fmt.Errorf("%w: %s returned HTTP %d", ErrUnavailable, name, status)
}
