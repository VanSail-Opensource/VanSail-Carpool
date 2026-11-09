package xnode

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestCertificateFormsValidateModesAndBoundPEMMaterial(t *testing.T) {
	for _, config := range []wire.CertConfig{
		{CertMode: "http", Domain: "*.example.com"}, {CertMode: "http", Domain: "https://example.com"},
		{CertMode: "http", Domain: "example.com", Email: "invalid"}, {CertMode: "dns", Domain: "*.example.com"},
		{CertMode: "file", CertFile: "/etc/cert.pem"}, {CertMode: "content", Domain: "example.com", CertContent: "invalid", KeyContent: "invalid"},
	} {
		if err := validateCertificateSettings(NodeSettings{Certificates: map[string]wire.Certificate{"primary": {Config: config}}}, nil); err == nil {
			t.Fatal("invalid managed certificate accepted", config.CertMode)
		}
	}
	for _, config := range []wire.CertConfig{
		{CertMode: "http", Domain: "example.com"}, {CertMode: "file", CertFile: "/etc/cert.pem", KeyFile: "/etc/key.pem"},
		{CertMode: "self", Domain: "local.example"}, {CertMode: "dns", Domain: "*.example.com", DNSProvider: "cloudflare"},
	} {
		if err := validateCertificateSettings(NodeSettings{Certificates: map[string]wire.Certificate{"primary": {Config: config}}}, nil); err != nil {
			t.Fatal("existing certificate mode rejected", config.CertMode, err)
		}
	}
}

func TestCertificateContentRejectsWrongPrivateKeyAndHostname(t *testing.T) {
	cert, key := certificateFormMaterial(t)
	valid := wire.CertConfig{CertMode: "content", Domain: "proxy.example", CertContent: cert, KeyContent: key}
	check := func(config wire.CertConfig) error {
		return validateCertificateSettings(NodeSettings{Certificates: map[string]wire.Certificate{"primary": {Config: config}}}, nil)
	}
	if err := check(valid); err != nil {
		t.Fatal(err)
	}
	_, wrong := certificateFormMaterial(t)
	invalid := valid
	invalid.KeyContent = wrong
	if check(invalid) == nil {
		t.Fatal("unrelated private key accepted")
	}
	invalid = valid
	invalid.Domain = "other.example"
	if check(invalid) == nil {
		t.Fatal("certificate for another name accepted")
	}
}

func certificateFormMaterial(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	spec := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "proxy.example"}, DNSNames: []string{"proxy.example"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, &spec, &spec, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: secret}))
}
