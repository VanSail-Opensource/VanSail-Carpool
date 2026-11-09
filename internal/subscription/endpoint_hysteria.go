package subscription

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func endpointHysteria(p Proxy, format string, out map[string]any) error {
	h, err := endpointHop(p)
	if err != nil {
		return err
	}
	if p.Protocol != "hysteria" {
		return nil
	}
	if h.opaque {
		if format == "xray" || format == "xrayJson" {
			return nil
		}
		return fmt.Errorf("%w: Hysteria hopping mask cannot be represented", ErrUnsupported)
	}
	if format == "xray" || format == "xrayJson" {
		if len(h.ports) > 0 || h.disabled {
			endpointXrayHopping(out, h)
		}
		return nil
	}
	if len(h.ports) == 0 {
		if h.disabled {
			delete(out, "ports")
			delete(out, "hop-interval")
			delete(out, "server_ports")
			delete(out, "hop_interval")
			delete(out, "hop_interval_max")
			if format == "sing-box" || format == "singbox" {
				out["server_port"] = p.Port
			}
		}
		return nil
	}
	switch format {
	case "sing-box", "singbox":
		delete(out, "server_port")
		out["server_ports"] = endpointHopPorts(h.ports, ":")
		out["hop_interval"] = strconv.FormatInt(h.interval.Min, 10) + "s"
		if h.interval.Min != h.interval.Max {
			out["hop_interval_max"] = strconv.FormatInt(h.interval.Max, 10) + "s"
		} else {
			delete(out, "hop_interval_max")
		}
	case "clash", "mihomo", "stash":
		out["ports"] = strings.Join(endpointHopPorts(h.ports, "-"), ",")
		if h.interval.Min == h.interval.Max {
			out["hop-interval"] = h.interval.Min
		} else if format != "stash" {
			out["hop-interval"] = endpointHopInterval(h.interval)
		} else {
			delete(out, "hop-interval")
		}
	default:
		return fmt.Errorf("%w: Hysteria hopping format", ErrUnsupported)
	}
	return nil
}

func endpointXrayHopping(out map[string]any, h endpointHopSettings) {
	stream := cloneTuningObject(object(out, "streamSettings"))
	if stream == nil {
		stream = map[string]any{}
	}
	final := object(stream, "finalmask")
	if final == nil {
		final = map[string]any{}
		stream["finalmask"] = final
	}
	var masks []any
	inserted := false
	for _, value := range array(final, "udp") {
		mask, _ := value.(map[string]any)
		if strings.EqualFold(text(mask, "type"), "udphop") {
			if !inserted && !h.disabled {
				masks = append(masks, endpointXrayHopMask(h))
				inserted = true
			}
		} else {
			masks = append(masks, value)
		}
	}
	if !inserted && !h.disabled {
		masks = append(masks, endpointXrayHopMask(h))
	}
	if len(masks) == 0 {
		delete(final, "udp")
	} else {
		final["udp"] = masks
	}
	out["streamSettings"] = stream
}

func endpointXrayHopMask(h endpointHopSettings) map[string]any {
	settings := cloneTuningObject(h.settings)
	if settings == nil {
		settings = map[string]any{}
	}
	settings["mode"] = "intervalRemote"
	settings["remotePorts"] = strings.Join(endpointHopPorts(h.ports, "-"), ",")
	settings["interval"] = endpointHopInterval(h.interval)
	return map[string]any{"type": "udphop", "settings": settings}
}

func endpointHysteriaNative(p Proxy, link *url.URL) error {
	h, err := endpointHop(p)
	if err != nil {
		return err
	}
	if h.opaque {
		return fmt.Errorf("%w: native Hysteria hopping mask", ErrUnsupported)
	}
	if len(h.ports) > 0 {
		link.Host = net.JoinHostPort(p.Address, strings.Join(endpointHopPorts(h.ports, "-"), ","))
	} else if h.disabled {
		link.Host = net.JoinHostPort(p.Address, strconv.Itoa(p.Port))
	}
	return nil
}

func endpointHysteriaTimingOmitted(p Proxy, format string) bool {
	h, err := endpointHop(p)
	if err != nil || len(h.ports) == 0 {
		return false
	}
	if h.invalidTime {
		return true
	}
	if !h.customTime {
		return false
	}
	return format == "native" || format == "base64" || format == "stash" && h.interval.Min != h.interval.Max
}
