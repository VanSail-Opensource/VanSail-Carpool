package subscription

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type endpointHopSettings struct {
	ports       []TuningPortRange
	interval    TuningRange
	settings    map[string]any
	opaque      bool
	disabled    bool
	inherited   bool
	customTime  bool
	invalidTime bool
}

func endpointHop(p Proxy) (endpointHopSettings, error) {
	h := endpointHopSettings{interval: TuningRange{Min: 30, Max: 30}}
	if p.Protocol != "hysteria" {
		return h, nil
	}
	options := p.EndpointTuning
	if options != nil && options.HysteriaPorts != nil && len(*options.HysteriaPorts) == 0 {
		h.disabled = true
		return h, nil
	}
	for _, item := range array(object(cloneTuningObject(p.Stream), "finalmask"), "udp") {
		mask, _ := item.(map[string]any)
		if !strings.EqualFold(text(mask, "type"), "udphop") {
			continue
		}
		if h.inherited {
			return h, fmt.Errorf("%w: multiple Hysteria hopping masks", ErrUnsupported)
		}
		h.inherited, h.settings = true, object(mask, "settings")
		if h.settings == nil {
			return h, fmt.Errorf("%w: missing Hysteria hopping settings", ErrUnsupported)
		}
		h.opaque = !strings.EqualFold(text(h.settings, "mode"), "intervalRemote") || len(array(h.settings, "remoteIPs")) > 0
		if value, ok := h.settings["remotePorts"]; ok {
			for _, part := range strings.Split(fmt.Sprint(value), ",") {
				r, err := endpointHopRange(part, 1, 65535)
				if err != nil {
					return h, err
				}
				h.ports = append(h.ports, TuningPortRange{Min: r.Min, Max: r.Max})
			}
		}
		if value, ok := h.settings["interval"]; ok {
			interval, err := endpointHopRange(fmt.Sprint(value), 5, math.MaxInt32)
			if err != nil {
				h.invalidTime = true
			} else {
				h.interval = interval
			}
			h.customTime = h.interval.Min != 30 || h.interval.Max != 30
		}
	}
	if options != nil {
		if h.opaque && (options.HysteriaPorts != nil || options.HysteriaHopInterval != nil) {
			return h, fmt.Errorf("%w: Hysteria hopping overrides require an intervalRemote mask", ErrUnsupported)
		}
		if options.HysteriaPorts != nil {
			h.ports = append([]TuningPortRange(nil), (*options.HysteriaPorts)...)
		}
		if options.HysteriaHopInterval != nil && len(h.ports) > 0 {
			h.interval, h.customTime, h.invalidTime = *options.HysteriaHopInterval, true, false
		}
	}
	for _, r := range h.ports {
		if r.Min < 1 || r.Max > 65535 || r.Max < r.Min {
			return h, fmt.Errorf("%w: invalid Hysteria hopping ports", ErrUnsupported)
		}
	}
	if h.interval.Min < 5 || h.interval.Max < h.interval.Min || h.interval.Max > math.MaxInt32 {
		h.interval, h.invalidTime = TuningRange{Min: 30, Max: 30}, true
	}
	return h, nil
}

func hasEndpointHysteriaPorts(p Proxy) bool {
	h, err := endpointHop(p)
	return err == nil && len(h.ports) > 0 && !h.disabled
}

func endpointHopRange(value string, minimum, maximum int64) (TuningRange, error) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) < 1 || len(parts) > 2 {
		return TuningRange{}, fmt.Errorf("%w: invalid Hysteria hopping range", ErrUnsupported)
	}
	low, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	high := low
	if len(parts) == 2 && err == nil {
		high, err = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	}
	if err != nil || low < minimum || high < low || high > maximum {
		return TuningRange{}, fmt.Errorf("%w: invalid Hysteria hopping range", ErrUnsupported)
	}
	return TuningRange{Min: low, Max: high}, nil
}

func endpointHopPorts(ports []TuningPortRange, separator string) []string {
	values := make([]string, 0, len(ports))
	for _, r := range ports {
		value := strconv.FormatInt(r.Min, 10)
		if r.Min != r.Max {
			value += separator + strconv.FormatInt(r.Max, 10)
		}
		values = append(values, value)
	}
	return values
}

func endpointHopInterval(r TuningRange) string {
	if r.Min == r.Max {
		return strconv.FormatInt(r.Min, 10)
	}
	return fmt.Sprintf("%d-%d", r.Min, r.Max)
}
