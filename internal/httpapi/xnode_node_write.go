package httpapi

import (
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// nodeWrite preserves the existing node fields while requiring explicit precision.
type nodeWrite struct {
	xnode.Node
	Multiplier json.RawMessage `json:"multiplier"`
}

var nodeMultiplierPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,3})(\.[0-9]{1,6})?$`)

func explicitNodeMultiplier(raw json.RawMessage) (float64, error) {
	if !nodeMultiplierPattern.Match(raw) {
		return 0, xnode.ErrInvalid
	}
	value, err := strconv.ParseFloat(string(raw), 64)
	if err != nil || value > 1000 {
		return 0, xnode.ErrInvalid
	}
	return value, nil
}
