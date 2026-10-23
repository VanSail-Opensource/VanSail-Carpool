package secret

import (
	"bytes"
	"strings"
	"testing"
)

func TestVaultRoundTripAndSettingBinding(t *testing.T) {
	t.Parallel()

	vault, err := NewVault(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatalf("NewVault(): %v", err)
	}
	ciphertext, err := vault.Encrypt("provider.api_token", "bearer-secret")
	if err != nil {
		t.Fatalf("Encrypt(): %v", err)
	}
	if strings.Contains(ciphertext, "bearer-secret") {
		t.Fatal("ciphertext contains plaintext")
	}
	plaintext, err := vault.Decrypt("provider.api_token", ciphertext)
	if err != nil || plaintext != "bearer-secret" {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
	if _, err := vault.Decrypt("billing.ezpay.key", ciphertext); err == nil {
		t.Fatal("Decrypt() accepted ciphertext under a different setting name")
	}
}

func TestVaultRejectsInvalidKeyAndCiphertext(t *testing.T) {
	t.Parallel()

	if _, err := NewVault(make([]byte, 31)); err == nil {
		t.Fatal("NewVault() accepted a non-256-bit key")
	}
	vault, err := NewVault(make([]byte, 32))
	if err != nil {
		t.Fatalf("NewVault(): %v", err)
	}
	if _, err := vault.Decrypt("setting", "not-base64"); err == nil {
		t.Fatal("Decrypt() accepted malformed ciphertext")
	}
}
