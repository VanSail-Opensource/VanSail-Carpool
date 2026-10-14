package xnode

import (
	"sort"
	"strconv"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// ParseBlockedPorts retains compact intervals without enumerating individual ports.
func ParseBlockedPorts(raw string) ([]wire.PortRange, error) {
	out := []wire.PortRange{}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	if len(raw) > 8192 {
		return nil, ErrInvalid
	}
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		parts := strings.Split(token, "-")
		if len(parts) > 2 || len(parts) == 0 {
			return nil, ErrInvalid
		}
		from, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || from < 1 || from > 65535 {
			return nil, ErrInvalid
		}
		to := from
		if len(parts) == 2 {
			to, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil || to < from || to > 65535 {
				return nil, ErrInvalid
			}
		}
		out = append(out, wire.PortRange{From: from, To: to})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].From < out[j].From })
	merged := out[:0]
	for _, interval := range out {
		if len(merged) > 0 && interval.From <= merged[len(merged)-1].To+1 {
			if interval.To > merged[len(merged)-1].To {
				merged[len(merged)-1].To = interval.To
			}
			continue
		}
		merged = append(merged, interval)
	}
	return merged, nil
}
