package xnode

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"time"
)

// validationAssets replaces node-local asset dependencies only in an analysis copy.
func validationAssets(native map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(native)
	if err != nil {
		return nil, err
	}
	var copy map[string]any
	if err = json.Unmarshal(raw, &copy); err != nil {
		return nil, err
	}
	delete(copy, "geodata")
	routing, _ := copy["routing"].(map[string]any)
	rules, _ := routing["rules"].([]any)
	for _, value := range rules {
		rule, _ := value.(map[string]any)
		validationGeodataMatchers(rule, "domain", "geosite:", "domain:configuration-validation.invalid")
		for _, field := range []string{"ip", "sourceIP", "source"} {
			validationGeodataMatchers(rule, field, "geoip:", "192.0.2.1/32")
		}
	}
	dns, _ := copy["dns"].(map[string]any)
	servers, _ := dns["servers"].([]any)
	for _, value := range servers {
		server, _ := value.(map[string]any)
		validationGeodataMatchers(server, "domains", "geosite:", "domain:configuration-validation.invalid")
		for _, field := range []string{"expectedIPs", "unexpectedIPs"} {
			validationGeodataMatchers(server, field, "geoip:", "192.0.2.1/32")
		}
	}
	var certificate, key []string
	var walk func(any) error
	walk = func(value any) error {
		switch value := value.(type) {
		case map[string]any:
			if value["certificateFile"] != nil || value["keyFile"] != nil {
				if certificate == nil {
					certificate, key, err = cachedValidationCertificate()
					if err != nil {
						return err
					}
				}
				delete(value, "certificateFile")
				delete(value, "keyFile")
				value["certificate"], value["key"] = certificate, key
			}
			for _, item := range value {
				if err := walk(item); err != nil {
					return err
				}
			}
		case []any:
			for _, item := range value {
				if err := walk(item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = walk(copy); err != nil {
		return nil, err
	}
	return copy, nil
}

// Preserve matcher structure in the analysis copy while deferring data-file
// contents to real agent validation. An empty matcher must not hide core errors.
func validationGeodataMatchers(object map[string]any, field, prefix, replacement string) {
	items, _ := object[field].([]any)
	for i, item := range items {
		value, _ := item.(string)
		if strings.HasPrefix(value, prefix) {
			items[i] = replacement
		}
	}
}

func validationCertificate() ([]string, []string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	certificate := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "configuration-validation"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, &certificate, &certificate, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	private, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))), "\n"), strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: private}))), "\n"), nil
}
