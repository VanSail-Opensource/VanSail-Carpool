package xnode

import (
	"crypto/ecdh"
	"encoding/base64"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func clientStream(inbound map[string]any) (map[string]any, error) {
	source, _ := inbound["streamSettings"].(map[string]any)
	stream := wire.Clone(source)
	if stream == nil {
		stream = map[string]any{}
	}
	network, _ := stream["network"].(string)
	if network == "" || network == "raw" {
		stream["network"] = "tcp"
	}
	security, _ := stream["security"].(string)
	if security == "" {
		stream["security"] = "none"
	}
	delete(stream, "sockopt")
	if tls, _ := stream["tlsSettings"].(map[string]any); tls != nil {
		delete(tls, "certificates")
		delete(tls, "rejectUnknownSni")
		delete(tls, "disableSystemRoot")
		if tls["serverName"] == nil {
			if names, _ := tls["serverNames"].([]any); len(names) > 0 {
				tls["serverName"] = names[0]
			}
		}
		delete(tls, "serverNames")
		tls["fingerprint"] = "chrome"
	}
	if reality, _ := stream["realitySettings"].(map[string]any); reality != nil {
		encoded, _ := reality["privateKey"].(string)
		key, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, ErrInvalid
		}
		private, err := ecdh.X25519().NewPrivateKey(key)
		if err != nil {
			return nil, ErrInvalid
		}
		client := map[string]any{"publicKey": base64.RawURLEncoding.EncodeToString(private.PublicKey().Bytes()), "fingerprint": "chrome"}
		if names, _ := reality["serverNames"].([]any); len(names) > 0 {
			client["serverName"] = names[0]
		}
		if ids, _ := reality["shortIds"].([]any); len(ids) > 0 {
			client["shortId"] = ids[0]
		}
		stream["realitySettings"] = client
	}
	return stream, nil
}
