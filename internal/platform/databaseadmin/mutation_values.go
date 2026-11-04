package databaseadmin

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func parseEditorValue(column schemaColumn, value Value) (any, error) {
	if value.kind == valueNull {
		if !column.Nullable {
			return nil, fmt.Errorf("%w: %s cannot be NULL", ErrInvalidValue, column.Name)
		}
		return nil, nil
	}
	if column.Boolean {
		if boolean, ok := value.Bool(); ok {
			if boolean {
				if column.DeclaredType == "boolean" {
					return true, nil
				}
				return int64(1), nil
			}
			if column.DeclaredType == "boolean" {
				return false, nil
			}
			return int64(0), nil
		}
		if text, ok := value.Text(); ok {
			switch strings.ToLower(strings.TrimSpace(text)) {
			case "1", "true":
				if column.DeclaredType == "boolean" {
					return true, nil
				}
				return int64(1), nil
			case "0", "false":
				if column.DeclaredType == "boolean" {
					return false, nil
				}
				return int64(0), nil
			}
		}
		return nil, fmt.Errorf("%w: %s must be boolean", ErrInvalidValue, column.Name)
	}
	switch column.Family {
	case storageFamilyInteger:
		text, ok := value.Text()
		if !ok {
			return nil, fmt.Errorf("%w: %s must be a decimal integer string", ErrInvalidValue, column.Name)
		}
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: %s must be a decimal integer string", ErrInvalidValue, column.Name)
		}
		return parsed, nil
	case storageFamilyReal:
		text, ok := value.Text()
		if !ok || !decimalPattern.MatchString(text) {
			return nil, fmt.Errorf("%w: %s must be a finite decimal string", ErrInvalidValue, column.Name)
		}
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
			return nil, fmt.Errorf("%w: %s must be a finite decimal string", ErrInvalidValue, column.Name)
		}
		return parsed, nil
	case storageFamilyNumeric:
		text, ok := value.Text()
		if !ok || !decimalPattern.MatchString(text) {
			return nil, fmt.Errorf("%w: %s must be a decimal string", ErrInvalidValue, column.Name)
		}
		return text, nil
	case storageFamilyBlob:
		blob, ok := value.Blob()
		if !ok {
			return nil, fmt.Errorf("%w: %s must use blobBase64", ErrInvalidValue, column.Name)
		}
		return blob, nil
	default:
		text, ok := value.Text()
		if !ok {
			return nil, fmt.Errorf("%w: %s must be text", ErrInvalidValue, column.Name)
		}
		return text, nil
	}
}
