package xnode

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func NewCredentials() (Credentials, error) {
	uuid, err := ids.New()
	if err != nil {
		return Credentials{}, err
	}
	password, err := ids.Token(32)
	if err != nil {
		return Credentials{}, err
	}
	classic, err := ids.Token(32)
	if err != nil {
		return Credentials{}, err
	}
	one, two := make([]byte, 16), make([]byte, 32)
	if _, err = rand.Read(one); err != nil {
		return Credentials{}, err
	}
	if _, err = rand.Read(two); err != nil {
		return Credentials{}, err
	}
	return Credentials{UUID: uuid, Trojan: password, Shadowsocks: classic, SS2022_128: base64.StdEncoding.EncodeToString(one), SS2022_256: base64.StdEncoding.EncodeToString(two), Inbound: map[string]json.RawMessage{}}, nil
}
func identityScope(identity Identity) string {
	if identity.UserID != "" {
		return "user." + identity.UserID
	}
	return "service." + identity.ServiceName
}
func loadCredentials(vault *secret.Vault, identity Identity) (Credentials, error) {
	raw, err := vault.Decrypt("xnode.identity."+identityScope(identity), identity.CredentialsCipher)
	if err != nil {
		return Credentials{}, err
	}
	var credentials Credentials
	err = wire.Decode([]byte(raw), &credentials)
	return credentials, err
}
func sealCredentials(vault *secret.Vault, identity Identity, credentials Credentials) (string, error) {
	raw, err := json.Marshal(credentials)
	if err != nil {
		return "", err
	}
	return vault.Encrypt("xnode.identity."+identityScope(identity), string(raw))
}

func inboundCredential(inbound map[string]any, credentials Credentials) (json.RawMessage, error) {
	tag, _ := inbound["tag"].(string)
	if value := credentials.Inbound[tag]; len(value) > 0 {
		return value, nil
	}
	settings, _ := inbound["settings"].(map[string]any)
	protocol, _ := inbound["protocol"].(string)
	account := map[string]any{}
	switch protocol {
	case "vless", "vmess":
		account["id"] = credentials.UUID
		stream, _ := inbound["streamSettings"].(map[string]any)
		if protocol == "vless" {
			flow, explicit := settings["flow"]
			if flow == "xtls-rprx-vision" || !explicit && (stream["network"] == "tcp" || stream["network"] == "raw") && (stream["security"] == "tls" || stream["security"] == "reality") {
				account["flow"] = "xtls-rprx-vision"
			}
		}
	case "trojan":
		account["password"] = credentials.Trojan
	case "hysteria":
		account["auth"] = credentials.UUID
	case "shadowsocks":
		method, _ := settings["method"].(string)
		switch method {
		case "chacha20-ietf-poly1305":
			account["method"], account["password"] = method, credentials.Shadowsocks
		case "2022-blake3-aes-128-gcm":
			account["password"] = credentials.SS2022_128
		case "2022-blake3-aes-256-gcm":
			account["password"] = credentials.SS2022_256
		default:
			return nil, ErrInvalid
		}
	case "socks", "http", "mixed":
		account["pass"] = credentials.Trojan
	default:
		return nil, ErrInvalid
	}
	if strings.HasPrefix(protocol, "_") {
		return nil, ErrInvalid
	}
	return json.Marshal(account)
}
