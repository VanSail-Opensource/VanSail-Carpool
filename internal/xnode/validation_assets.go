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
		for _, field := range []string{"domain", "ip"} {
			items, _ := rule[field].([]any)
			if items == nil {
				continue
			}
			safe := []any{}
			for _, item := range items {
				value, _ := item.(string)
				if !strings.HasPrefix(value, "geoip:") && !strings.HasPrefix(value, "geosite:") {
					safe = append(safe, item)
				}
			}
			if len(safe) == 0 {
				delete(rule, field)
			} else {
				rule[field] = safe
			}
		}
	}
	var certificate, key []string
	var walk func(any) error
	walk = func(value any) error {
		switch value := value.(type) {
		case map[string]any:
			if value["certificateFile"] != nil || value["keyFile"] != nil {
				if certificate == nil {
					certificate, key, err = validationCertificate()
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
