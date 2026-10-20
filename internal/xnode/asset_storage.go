package xnode

import (
	"context"
	"errors"
	"os"
)

// assetFailure retains only a fixed diagnostic. Asset URLs and filesystem paths
// can contain secrets and never belong in policy responses or warning logs.
type assetFailure struct{ reason string }

func (e *assetFailure) Error() string { return e.reason }
func (e *assetFailure) Unwrap() error { return ErrUnavailable }

func assetStorageFailure() error { return &assetFailure{"asset scratch storage is unavailable"} }
func assetTransferFailure(name string, err error) error {
	var path *os.PathError
	if errors.As(err, &path) {
		return assetStorageFailure()
	}
	return assetRequestFailure(name, err)
}

func assetDecodeFailure(err error) error {
	var path *os.PathError
	if errors.As(err, &path) {
		return assetStorageFailure()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return ErrInvalid
}

// SafeOperationCause returns a bounded diagnostic without URLs, tokens or paths.
func SafeOperationCause(err error) string {
	var asset *assetFailure
	if errors.As(err, &asset) {
		return asset.reason
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "operation deadline exceeded"
	case errors.Is(err, context.Canceled):
		return "operation canceled"
	case errors.Is(err, ErrInvalid):
		return "invalid policy or asset data"
	case errors.Is(err, ErrUnavailable):
		return "upstream or node operation unavailable"
	default:
		return "policy operation failed"
	}
}
