package pkg_test

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"x-admin/internal/pkg/totp"
)

// RFC 4226 Appendix D test vectors (six digit HOTP).
func TestHOTPVectors(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // "12345678901234567890"
	want := []string{"755224", "287082", "359152", "969429", "338314", "254676", "287922", "162583", "399871", "520489"}
	for counter, expected := range want {
		at := time.Unix(int64(counter)*30, 0)
		got, err := totp.Code(secret, at)
		if err != nil {
			t.Fatalf("counter %d: code: %v", counter, err)
		}
		if got != expected {
			t.Fatalf("counter %d: got %s want %s", counter, got, expected)
		}
	}
}

func TestCodeAndValidateWindow(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	base := time.Unix(59, 0)

	code, err := totp.Code(secret, base)
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if code != "287082" {
		t.Fatalf("unexpected code %s", code)
	}
	if !totp.Validate(secret, code, base) {
		t.Fatal("current code should validate")
	}
	// One step later the previous code is still inside the ±1 window.
	if !totp.Validate(secret, code, base.Add(30*time.Second)) {
		t.Fatal("adjacent step should validate with skew")
	}
	if totp.ValidateSkew(secret, code, base.Add(90*time.Second), 1) {
		t.Fatal("code three steps away must not validate")
	}
	if totp.Validate(secret, "000000", base) && code != "000000" {
		t.Fatal("wrong code accepted")
	}
	if totp.Validate(secret, "not-a-code", base) {
		t.Fatal("malformed code accepted")
	}
	if totp.Validate("not-base32!!", code, base) {
		t.Fatal("malformed secret accepted")
	}
}

func TestGenerateSecretIsRandom(t *testing.T) {
	first, err := totp.GenerateSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := totp.GenerateSecret()
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
	raw := totp.ProvisioningURL("x-admin", "admin@example.com", "JBSWY3DPEHPK3PXP")
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
	dataURL, err := totp.QRCodeDataURL("otpauth://totp/test")
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
	codes, err := totp.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(codes) != totp.RecoveryCodeCount {
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
		if totp.HashRecoveryCode(strings.ToLower(strings.ReplaceAll(code, "-", " "))) != totp.HashRecoveryCode(code) {
			t.Fatalf("normalization mismatch for %q", code)
		}
	}
}

func TestCodeRejectsInvalidSecret(t *testing.T) {
	_, err := totp.Code("not-base32!!", time.Now())
	if err == nil {
		t.Fatal("invalid secret accepted")
	}
	// 去掉填充符后为空，解码结果长度为 0。
	if _, err = totp.Code("=", time.Now()); err == nil {
		t.Fatal("empty secret accepted")
	}
	if _, err = totp.Code("", time.Now()); err == nil {
		t.Fatal("blank secret accepted")
	}
}

func TestValidateSkewEdgeCases(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	// counter 为负（1970 之前）：越界步长应被跳过而不是 panic。
	code := "287082" // counter 1 的 RFC 测试向量
	if !totp.ValidateSkew(secret, code, time.Unix(-45, 0), 2) {
		t.Fatal("negative-step window should still validate the previous code")
	}
	// skew 为负时按 0 处理：只检查当前步，不再回看上一步。
	// 上一步（counter 0）的验证码在当前步不应命中。
	if totp.ValidateSkew(secret, "755224", time.Unix(30, 0), -1) {
		t.Fatal("negative skew must disable backward lookup")
	}
	if !totp.ValidateSkew(secret, code, time.Unix(30, 0), 0) {
		t.Fatal("current step should validate without skew")
	}
}

func TestQRCodeRejectsOversizedPayload(t *testing.T) {
	if _, err := totp.QRCodeDataURL(strings.Repeat("x", 4000)); err == nil {
		t.Fatal("oversized payload should fail to encode")
	}
}
