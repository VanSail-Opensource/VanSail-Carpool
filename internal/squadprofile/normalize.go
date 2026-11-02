package squadprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalid = errors.New("invalid squad profile")

// Normalize validates one profile and returns a canonical copy suitable for storage.
func Normalize(input *Profile) (*Profile, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: profile is required", ErrInvalid)
	}
	result := &Profile{Type: input.Type}
	switch input.Type {
	case Broadband:
		result.ISP = strings.TrimSpace(input.ISP)
		result.Location = strings.TrimSpace(input.Location)
		result.PortMbps = positivePort(input.PortMbps)
		if input.Dynamic != nil {
			value := *input.Dynamic
			result.Dynamic = &value
		} else {
			value := false
			result.Dynamic = &value
		}
		if result.ISP == "" || result.Location == "" || result.PortMbps == nil {
			return nil, fmt.Errorf("%w: broadband fields are incomplete", ErrInvalid)
		}
	case ChinaOptimized:
		result.CT = strings.TrimSpace(input.CT)
		result.CU = strings.TrimSpace(input.CU)
		result.CM = strings.TrimSpace(input.CM)
		result.PortMbps = positivePort(input.PortMbps)
		result.CountryCode = validCountry(input.CountryCode)
		if result.CT == "" || result.CU == "" || result.CM == "" || result.CountryCode == "" {
			return nil, fmt.Errorf("%w: China Optimized fields are incomplete", ErrInvalid)
		}
		if input.PortMbps != nil && result.PortMbps == nil {
			return nil, fmt.Errorf("%w: port must be a positive integer", ErrInvalid)
		}
	case InternationalNetwork:
		result.PortMbps = positivePort(input.PortMbps)
		result.CountryCode = validCountry(input.CountryCode)
		result.UpstreamCarriers = normalizeCarriers(input.UpstreamCarriers)
		if result.CountryCode == "" || len(result.UpstreamCarriers) == 0 {
			return nil, fmt.Errorf("%w: International Network fields are incomplete", ErrInvalid)
		}
		if input.PortMbps != nil && result.PortMbps == nil {
			return nil, fmt.Errorf("%w: port must be a positive integer", ErrInvalid)
		}
	default:
		return nil, fmt.Errorf("%w: unknown profile type", ErrInvalid)
	}
	return result, nil
}

func validCountry(value string) string {
	code, valid := normalizeCountry(value)
	if !valid {
		return ""
	}
	return code
}

func positivePort(value *int) *int {
	if value == nil || *value <= 0 {
		return nil
	}
	result := *value
	return &result
}

func normalizeCarriers(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			carrier := strings.TrimSpace(item)
			if carrier == "" {
				continue
			}
			if _, exists := seen[carrier]; exists {
				continue
			}
			seen[carrier] = struct{}{}
			result = append(result, carrier)
		}
	}
	return result
}

// ParseJSON decodes and validates the JSON representation stored as PostgreSQL text.
func ParseJSON(raw string) (*Profile, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var input Profile
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, fmt.Errorf("decode squad profile: %w", err)
	}
	return Normalize(&input)
}
