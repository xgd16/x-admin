package totp

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"
)

// RFC 4226 Appendix D test vectors (six digit HOTP).
func TestHOTPVectors(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // "12345678901234567890"
	key, err := decodeSecret(secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	want := []string{"755224", "287082", "359152", "969429", "338314", "254676", "287922", "162583", "399871", "520489"}
	for counter, expected := range want {
		if got := hotp(key, uint64(counter), Digits); got != expected {
			t.Fatalf("counter %d: got %s want %s", counter, got, expected)
		}
	}
}

func TestCodeAndValidateWindow(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	base := time.Unix(59, 0)

	code, err := Code(secret, base)
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if code != "287082" {
		t.Fatalf("unexpected code %s", code)
	}
	if !Validate(secret, code, base) {
		t.Fatal("current code should validate")
	}
	// One step later the previous code is still inside the ±1 window.
	if !Validate(secret, code, base.Add(30*time.Second)) {
		t.Fatal("adjacent step should validate with skew")
	}
	if ValidateSkew(secret, code, base.Add(90*time.Second), 1) {
		t.Fatal("code three steps away must not validate")
	}
	if Validate(secret, "000000", base) && code != "000000" {
		t.Fatal("wrong code accepted")
	}
	if Validate(secret, "not-a-code", base) {
		t.Fatal("malformed code accepted")
	}
	if Validate("not-base32!!", code, base) {
		t.Fatal("malformed secret accepted")
	}
}

func TestGenerateSecretIsRandom(t *testing.T) {
	first, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if first == second {
		t.Fatal("secrets must differ")
	}
	if len(first) != 32 {
		t.Fatalf("unexpected secret length %d", len(first))
	}
}

func TestProvisioningURL(t *testing.T) {
	raw := ProvisioningURL("x-admin", "admin@example.com", "JBSWY3DPEHPK3PXP")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Scheme != "otpauth" || parsed.Host != "totp" {
		t.Fatalf("unexpected scheme/host: %s", raw)
	}
	if !strings.Contains(parsed.Path, "x-admin:") {
		t.Fatalf("issuer missing from label: %s", parsed.Path)
	}
	q := parsed.Query()
	if q.Get("secret") != "JBSWY3DPEHPK3PXP" || q.Get("issuer") != "x-admin" || q.Get("digits") != "6" || q.Get("period") != "30" {
		t.Fatalf("unexpected query: %v", q)
	}
}

func TestQRCodeDataURL(t *testing.T) {
	dataURL, err := QRCodeDataURL("otpauth://totp/test")
	if err != nil {
		t.Fatalf("qr: %v", err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		t.Fatalf("unexpected data URL prefix: %.40s", dataURL)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, prefix))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw) < 8 || string(raw[1:4]) != "PNG" {
		t.Fatal("payload is not a PNG")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("unexpected count %d", len(codes))
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if len(code) != 9 || code[4] != '-' {
			t.Fatalf("unexpected format %q", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
		if HashRecoveryCode(strings.ToLower(strings.ReplaceAll(code, "-", " "))) != HashRecoveryCode(code) {
			t.Fatalf("normalization mismatch for %q", code)
		}
	}
}
