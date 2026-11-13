package httpapi

import (
	"bytes"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

const defaultActivityTimezone = activity.DefaultTimezone

type nullableRequestField[T any] struct {
	Set   bool
	Value *T
}

func (field *nullableRequestField[T]) UnmarshalJSON(data []byte) error {
	field.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	field.Value = &value
	return nil
}
