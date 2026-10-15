package usdt

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/jsonlimit"

type transferPage[T any] []T

func (p *transferPage[T]) UnmarshalJSON(raw []byte) error {
	values, err := jsonlimit.DecodeSlice[T](raw, 100)
	if err == nil {
		*p = values
	}
	return err
}

type accountPage[T any] []T

func (p *accountPage[T]) UnmarshalJSON(raw []byte) error {
	values, err := jsonlimit.DecodeSlice[T](raw, 128)
	if err == nil {
		*p = values
	}
	return err
}

type logPage[T any] []T

func (p *logPage[T]) UnmarshalJSON(raw []byte) error {
	values, err := jsonlimit.DecodeSlice[T](raw, 2000)
	if err == nil {
		*p = values
	}
	return err
}

type historyPage[T any] []T

func (p *historyPage[T]) UnmarshalJSON(raw []byte) error {
	values, err := jsonlimit.DecodeSlice[T](raw, 200)
	if err == nil {
		*p = values
	}
	return err
}
