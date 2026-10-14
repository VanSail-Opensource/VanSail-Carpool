package notifications

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type fieldSpec struct {
	label, key string
	format     func(string) (string, error)
	value      string
}

func pair(label, key string) fieldSpec          { return fieldSpec{label: label, key: key} }
func literalPair(label, value string) fieldSpec { return fieldSpec{label: label, value: value} }
func moneyPair(label, key string) fieldSpec     { return fieldSpec{label: label, key: key, format: money} }
func debitPair(label, key string) fieldSpec {
	return fieldSpec{label: label, key: key, format: debitMoney}
}
func bytesPair(label, key string) fieldSpec {
	return fieldSpec{label: label, key: key, format: bytesValue}
}

func datePair(label, key string, location *time.Location) fieldSpec {
	return fieldSpec{label: label, key: key, format: func(value string) (string, error) { return dateValue(value, location) }}
}

func fixedPair(label, value string, copy copySet) fieldSpec {
	return literalPair(label, copy.values[value])
}

func translatedPair(label, key string, copy copySet) fieldSpec {
	return fieldSpec{label: label, key: key, format: func(value string) (string, error) {
		if translated := copy.values[value]; translated != "" {
			return translated, nil
		}
		return value, nil
	}}
}

func requiredFields(copy copySet, facts map[string]string, specs ...fieldSpec) ([]cardField, error) {
	fields := make([]cardField, 0, len(specs))
	for _, spec := range specs {
		value := spec.value
		if spec.key != "" {
			value = strings.TrimSpace(facts[spec.key])
			if value == "" {
				return nil, errors.New("notification fact is missing: " + spec.key)
			}
		}
		if spec.format != nil {
			formatted, err := spec.format(value)
			if err != nil {
				return nil, fmt.Errorf("format notification fact %s: %w", spec.key, err)
			}
			value = formatted
		}
		fields = append(fields, cardField{copy.labels[spec.label], value})
	}
	return fields, nil
}

func insertOptional(fields []cardField, index int, text copySet, facts map[string]string, label, key string) []cardField {
	value := strings.TrimSpace(facts[key])
	if value == "" {
		return fields
	}
	fields = append(fields, cardField{})
	copy(fields[index+1:], fields[index:])
	fields[index] = cardField{text.labels[label], value}
	return fields
}

func money(value string) (string, error) {
	minor, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return "", err
	}
	return model.TXBMoney(minor).Display, nil
}

func debitMoney(value string) (string, error) {
	minor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || minor > 0 || minor == -1<<63 {
		return "", errors.New("invalid renewal debit")
	}
	return model.TXBMoney(-minor).Display, nil
}

func resetFailurePair(copy copySet) fieldSpec {
	return fieldSpec{label: "reason", key: FactReason, format: func(value string) (string, error) {
		if reason := copy.values[value]; reason != "" {
			return reason, nil
		}
		return copy.values["resetFailed"], nil
	}}
}

func bytesValue(value string) (string, error) {
	bytes, err := strconv.ParseInt(value, 10, 64)
	if err != nil || bytes < 0 {
		return "", errors.New("invalid byte amount")
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	amount, unit := float64(bytes), units[0]
	for index := 1; index < len(units) && amount >= 1024; index++ {
		amount /= 1024
		unit = units[index]
	}
	if unit == "B" {
		return fmt.Sprintf("%d B", bytes), nil
	}
	return fmt.Sprintf("%.2f %s", amount, unit), nil
}

func byteRatio(facts map[string]string, leftKey, rightKey string) (string, error) {
	left, err := bytesValue(facts[leftKey])
	if err != nil {
		return "", err
	}
	right, err := bytesValue(facts[rightKey])
	if err != nil {
		return "", err
	}
	return left + " / " + right, nil
}

func dateValue(value string, location *time.Location) (string, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", err
	}
	if location == nil {
		location = time.UTC
	}
	return parsed.In(location).Format("2006-01-02 15:04 MST"), nil
}
