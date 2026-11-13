package usdt

import (
	"bytes"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/jsonlimit"
)

type transferPage[T any] []T

func (p *transferPage[T]) UnmarshalJSON(raw []byte) error {
	values, err := decodeRequiredPage[T](raw, 100)
	if err == nil {
		*p = values
	}
	return err
}

type accountPage[T any] []T

func (p *accountPage[T]) UnmarshalJSON(raw []byte) error {
	values, err := decodeRequiredPage[T](raw, 128)
	if err == nil {
		*p = values
	}
	return err
}

type logPage[T any] []T

func (p *logPage[T]) UnmarshalJSON(raw []byte) error {
	values, err := decodeRequiredPage[T](raw, 2000)
	if err == nil {
		*p = values
	}
	return err
}

type historyPage[T any] []T

func (p *historyPage[T]) UnmarshalJSON(raw []byte) error {
	values, err := decodeRequiredPage[T](raw, 200)
	if err == nil {
		*p = values
	}
	return err
}

// A required RPC/indexer page must distinguish [] from omitted or null data.
// Only a proven empty array may authorize advancing past a history window.
func decodeRequiredPage[T any](raw []byte, limit int) ([]T, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return nil, ErrRPC
	}
	values, err := jsonlimit.DecodeSlice[T](raw, limit)
	if err != nil {
		return nil, err
	}
	if values == nil {
		values = []T{}
	}
	return values, nil
}
