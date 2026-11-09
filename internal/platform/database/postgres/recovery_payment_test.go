package postgres

import (
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func TestRecoveryPaymentCompatibilityRequiresExactHistoricalProof(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	seal := func(scope string) string {
		t.Helper()
		cipher, e := vault.Encrypt(scope, "private-fixture-value")
		if e != nil {
			t.Fatal(e)
		}
		return cipher
	}
	ezpay, bepusdt := seal("billing.ezpay.key"), seal("billing.bepusdt.api_token")
	for _, test := range []struct {
		name string
		p    recoveryPaymentProfile
		ok   bool
	}{
		{"canonical-custom", recoveryPaymentProfile{id: "custom", provider: "ezpay", cipher: seal("payment-profile:custom")}, true},
		{"canonical-builtin", recoveryPaymentProfile{id: "ezpay", provider: "ezpay", cipher: seal("payment-profile:ezpay")}, true},
		{"retained-ezpay", recoveryPaymentProfile{id: "ezpay", provider: "ezpay", cipher: ezpay, legacyValue: ezpay, legacyEncrypted: 1}, true},
		{"retained-bepusdt", recoveryPaymentProfile{id: "bepusdt", provider: "bepusdt", cipher: bepusdt, legacyValue: bepusdt, legacyEncrypted: 1}, true},
		{"custom-copied-legacy", recoveryPaymentProfile{id: "custom", provider: "ezpay", cipher: ezpay, legacyValue: ezpay, legacyEncrypted: 1}, false},
		{"provider-id-mismatch", recoveryPaymentProfile{id: "ezpay", provider: "bepusdt", cipher: ezpay, legacyValue: ezpay, legacyEncrypted: 1}, false},
		{"setting-not-encrypted", recoveryPaymentProfile{id: "ezpay", provider: "ezpay", cipher: ezpay, legacyValue: ezpay}, false},
		{"different-retained-cipher", recoveryPaymentProfile{id: "ezpay", provider: "ezpay", cipher: ezpay, legacyValue: seal("billing.ezpay.key"), legacyEncrypted: 1}, false},
		{"missing-retained-setting", recoveryPaymentProfile{id: "ezpay", provider: "ezpay", cipher: ezpay, legacyEncrypted: 1}, false},
		{"wrong-legacy-context", recoveryPaymentProfile{id: "ezpay", provider: "ezpay", cipher: bepusdt, legacyValue: bepusdt, legacyEncrypted: 1}, false},
		{"matching-malformed-envelope", recoveryPaymentProfile{id: "ezpay", provider: "ezpay", cipher: "invalid", legacyValue: "invalid", legacyEncrypted: 1}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := verifyRecoveryPaymentProfile(vault, test.p)
			if (err == nil) != test.ok {
				t.Fatal("unexpected recovery classification", err)
			}
			if err != nil && (err.Error() != "encrypted document verification failed" || strings.Contains(err.Error(), "private-fixture-value")) {
				t.Fatal("recovery error exposed a protected value")
			}
		})
	}
	wrongVault, e := secret.NewVault([]byte(strings.Repeat("x", 32)))
	if e != nil {
		t.Fatal(e)
	}
	if verifyRecoveryPaymentProfile(wrongVault, recoveryPaymentProfile{id: "ezpay", provider: "ezpay", cipher: ezpay, legacyValue: ezpay, legacyEncrypted: 1}) == nil {
		t.Fatal("wrong master key accepted")
	}
}
