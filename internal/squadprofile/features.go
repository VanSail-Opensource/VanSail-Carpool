package squadprofile

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

// Feature is administrator-authored text and a safe icon name for a squad card.
type Feature struct {
	Icon   string `json:"icon"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

// Features marshals an absent list as an empty array for legacy squad records.
type Features []Feature

var ErrInvalidFeatures = errors.New("invalid squad features")

func (features Features) MarshalJSON() ([]byte, error) {
	if features == nil {
		return []byte("[]"), nil
	}
	type plainFeatures []Feature
	return json.Marshal(plainFeatures(features))
}

// NormalizeFeatures bounds authored content and accepts only built-in icons.
func NormalizeFeatures(features Features) (Features, error) {
	if len(features) > 12 {
		return nil, ErrInvalidFeatures
	}
	result := make(Features, 0, len(features))
	for _, feature := range features {
		feature.Icon = strings.TrimSpace(feature.Icon)
		feature.Title = strings.TrimSpace(feature.Title)
		feature.Detail = strings.TrimSpace(feature.Detail)
		switch feature.Icon {
		case "route", "globe", "lightning", "shield", "sparkle", "house", "broadcast", "clock", "stack", "gauge", "server", "arrows-clockwise", "rocket-launch", "cloud", "wifi-high", "planet", "compass", "map-pin", "devices", "desktop", "timer", "lock-key", "check-circle", "lifebuoy", "heart", "leaf", "lightbulb", "cube", "link", "code", "funnel", "faders", "sliders", "arrows-left-right", "database", "globe-hemisphere-east":
		default:
			return nil, ErrInvalidFeatures
		}
		if !utf8.ValidString(feature.Title) || !utf8.ValidString(feature.Detail) || utf8.RuneCountInString(feature.Title) < 1 || utf8.RuneCountInString(feature.Title) > 80 || utf8.RuneCountInString(feature.Detail) > 160 {
			return nil, ErrInvalidFeatures
		}
		result = append(result, feature)
	}
	return result, nil
}
