package xnode

import (
	"crypto/tls"
	"crypto/x509"
	"net/mail"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"golang.org/x/net/idna"
)

// Match X-Node's effective-mode precedence without rewriting encrypted settings.
func certificateMode(config wire.CertConfig) string {
	if mode := strings.ToLower(strings.TrimSpace(config.CertMode)); mode != "" {
		return mode
	}
	if config.AutoTLS {
		return "http"
	}
	if config.CertContent != "" && config.KeyContent != "" {
		return "content"
	}
	if config.CertFile != "" && config.KeyFile != "" {
		return "file"
	}
	return "none"
}

func validCertificateDomain(domain string, wildcard bool) bool {
	domain = strings.TrimSuffix(domain, ".")
	if wildcard && strings.HasPrefix(domain, "*.") {
		domain = domain[2:]
	}
	name, err := idna.Lookup.ToASCII(domain)
	if err != nil || len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, value := range label {
			if !(value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '-') {
				return false
			}
		}
	}
	return true
}

// Local checks never read node paths, issue certificates or call DNS providers.
func validateCertificateSettings(settings NodeSettings, providers map[string]wire.Provider) error {
	if len(settings.Certificates) > 64 {
		return ErrInvalid
	}
	for id, profile := range settings.Certificates {
		config := profile.Config
		mode := certificateMode(config)
		if !wire.ValidName(id) || len(config.CertContent) > 1<<20 || len(config.KeyContent) > 1<<20 {
			return ErrInvalid
		}
		switch mode {
		case "http", "dns":
			if !validCertificateDomain(config.Domain, mode == "dns") {
				return ErrInvalid
			}
			if config.Email != "" {
				address, err := mail.ParseAddress(config.Email)
				if err != nil || address.Address != config.Email {
					return ErrInvalid
				}
			}
			if mode == "dns" {
				if profile.Provider == "" && config.DNSProvider == "" || profile.Provider != "" && providers[profile.Provider].Type == "" {
					return ErrInvalid
				}
			}
		case "file":
			if strings.TrimSpace(config.CertFile) == "" || strings.TrimSpace(config.KeyFile) == "" || strings.ContainsRune(config.CertFile+config.KeyFile, 0) {
				return ErrInvalid
			}
		case "content":
			pair, err := tls.X509KeyPair([]byte(config.CertContent), []byte(config.KeyContent))
			if err != nil || len(pair.Certificate) == 0 {
				return ErrInvalid
			}
			certificate, err := x509.ParseCertificate(pair.Certificate[0])
			if err != nil || config.Domain != "" && certificate.VerifyHostname(config.Domain) != nil {
				return ErrInvalid
			}
		case "self":
			if strings.TrimSpace(config.Domain) == "" {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
	}
	return nil
}
