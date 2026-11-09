package xnode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type EndpointFieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

type EndpointInbound struct {
	Tag string `json:"tag"`
	EndpointMetadata
}

type EndpointPreview struct {
	Valid       bool                                       `json:"valid"`
	Metadata    EndpointMetadata                           `json:"metadata"`
	Inbounds    []EndpointInbound                          `json:"inbounds"`
	Fields      []subscription.TuningField                 `json:"fields"`
	FieldErrors []EndpointFieldError                       `json:"fieldErrors"`
	Clients     []subscription.EndpointClientCompatibility `json:"clients"`
}

// PreviewEndpoint uses only local desired state and synthetic credentials. It
// never changes a node, stores a draft or contacts a client/provider/agent.
func (s *Service) PreviewEndpoint(ctx context.Context, item Endpoint) (EndpointPreview, error) {
	s = s.withCompileReads()
	result := EndpointPreview{Inbounds: []EndpointInbound{}, Fields: []subscription.TuningField{}, FieldErrors: []EndpointFieldError{}, Clients: []subscription.EndpointClientCompatibility{}}
	add := func(field, code string) {
		result.FieldErrors = append(result.FieldErrors, EndpointFieldError{field, code})
	}
	if strings.TrimSpace(item.Name) == "" {
		add("name", "required")
	}
	if item.Address == "" || strings.ContainsAny(item.Address, "\x00\r\n /?#@") {
		add("address", "invalid")
	}
	if item.Port < 1 || item.Port > 65535 {
		add("port", "invalid")
	}
	if item.NodeID == "" {
		add("nodeId", "required")
	}
	if !wire.ValidName(item.InboundTag) {
		add("inboundTag", "invalid")
	}
	if item.NodeID == "" {
		return result, nil
	}
	_, settings, inbounds, err := s.endpointInbounds(ctx, item.NodeID)
	if err != nil {
		if isMissing(err) || errors.Is(err, ErrInvalid) {
			add("inboundTag", "binding")
			return result, nil
		}
		return result, err
	}
	var inbound map[string]any
	for _, candidate := range inbounds {
		tag := stringField(candidate, "tag")
		result.Inbounds = append(result.Inbounds, EndpointInbound{tag, inboundMetadata(candidate, settings, tag)})
		if tag == item.InboundTag {
			inbound = candidate
		}
	}
	if inbound == nil {
		if wire.ValidName(item.InboundTag) {
			add("inboundTag", "binding")
		}
		return result, nil
	}
	result.Metadata = inboundMetadata(inbound, settings, item.InboundTag)
	overrides, options, decodeErr := endpointDraftOptions(item.Overrides)
	if decodeErr != nil {
		add("clientOptions", "invalid")
	}
	// Incomplete display identity must not prevent deriving applicable controls.
	draft := item
	draft.ID, draft.Name = "endpoint-preview", "endpoint-preview"
	if item.Address == "" || strings.ContainsAny(item.Address, "\x00\r\n /?#@") {
		draft.Address = "preview.invalid"
	}
	if item.Port < 1 || item.Port > 65535 {
		draft.Port = 443
	}
	draft.Overrides, err = json.Marshal(overrides)
	if err != nil {
		return result, err
	}
	if _, err = filterEndpointGroups(nil, draft.Overrides); err != nil {
		add("overrides", "invalid")
	}
	proxy, err := s.endpointProxyCredentials(ctx, draft, endpointPreviewCredentials())
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			add("overrides", "xrayValidation")
			return result, nil
		}
		return result, err
	}
	if decodeErr == nil {
		proxy.EndpointTuning = &options
	}
	result.Fields = subscription.EndpointFields(proxy)
	if decodeErr == nil {
		failures := subscription.ValidateEndpointOptions(options, proxy)
		for _, field := range slices.Sorted(maps.Keys(failures)) {
			if field == "clientOptions" {
				add(field, failures[field])
			} else {
				add("clientOptions."+field, failures[field])
			}
		}
		result.Clients = subscription.EndpointCompatibility(options, proxy)
		if len(result.FieldErrors) == 0 && validateEndpointProxy(proxy) != nil {
			add("clientOptions", "xrayValidation")
		}
	}
	if result.Fields == nil {
		result.Fields = []subscription.TuningField{}
	}
	if result.Clients == nil {
		result.Clients = []subscription.EndpointClientCompatibility{}
	}
	result.Valid = len(result.FieldErrors) == 0
	return result, nil
}

func endpointPreviewCredentials() Credentials {
	return Credentials{UUID: "00000000-0000-4000-8000-000000000001", Trojan: "synthetic-preview", Shadowsocks: "synthetic-preview",
		SS2022_128: base64.StdEncoding.EncodeToString(make([]byte, 16)), SS2022_256: base64.StdEncoding.EncodeToString(make([]byte, 32))}
}
