package xnode

import (
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// Remove only the new typed layer from the analysis copy, retaining encrypted
// legacy overrides and inherited transport/security configuration intact.
func endpointDraftOptions(raw json.RawMessage) (map[string]any, subscription.EndpointOptions, error) {
	values := map[string]any{}
	options := subscription.EndpointOptions{}
	if len(raw) > 1<<20 || len(raw) > 0 && json.Unmarshal(raw, &values) != nil {
		return map[string]any{}, options, ErrInvalid
	}
	if values == nil {
		values = map[string]any{}
	}
	value, exists := values["clientOptions"]
	delete(values, "clientOptions")
	if exists {
		encoded, err := json.Marshal(value)
		if err != nil || json.Unmarshal(encoded, &options) != nil {
			return values, options, ErrInvalid
		}
	}
	return values, options, nil
}

// A stale or unsupported endpoint must not remove its compatible peers. Store
// and credential failures remain fatal; rendering exclusions stay per format.
func unusableEndpoint(err error) bool {
	return errors.Is(err, ErrInvalid) || errors.Is(err, subscription.ErrUnsupported)
}

func validateEndpointProxy(proxy subscription.Proxy) error {
	out, err := proxy.Outbound()
	if err != nil {
		return ErrInvalid
	}
	data, err := json.Marshal(map[string]any{"inbounds": []any{}, "outbounds": []any{out}})
	if err != nil {
		return err
	}
	if validation := ValidateClientConfigurationDocument(data); !validation.Valid {
		return rejectedCompilation("client endpoint", validation.Detail)
	}
	return nil
}
