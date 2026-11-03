// Package totp 实现 Google Authenticator 使用的时间型一次性密码（RFC 6238），
// 以及双重验证所需的一次性恢复码工具。
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// TOTP 参数。Google Authenticator 默认使用 6 位、30 秒、SHA1。
const (
	// Digits 是动态验证码位数。
	Digits = 6
	// Period 是时间步长（秒）。
	Period = 30
	// SecretBytes 是共享密钥熵（160 bit，RFC 4226 推荐）。
	SecretBytes = 20
	// RecoveryCodeCount 是开启双重验证时签发的一次性恢复码数量。
	RecoveryCodeCount = 10
)

// recoveryAlphabet 与验证码一样去掉了易混淆字符。
const recoveryAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// b32 使用无填充的 Base32，符合 otpauth 约定。
var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret 生成新的随机 Base32 密钥（无填充）。
func GenerateSecret() (string, error) {
	raw := make([]byte, SecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return b32.EncodeToString(raw), nil
}

// Code 计算指定时刻的 TOTP 动态码。
func Code(secret string, at time.Time) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	return hotp(key, uint64(at.Unix()/Period), Digits), nil
}

// Validate 校验动态码，允许前后各一个时间步的时钟偏差。
func Validate(secret, code string, at time.Time) bool {
	return ValidateSkew(secret, code, at, 1)
}

// ValidateSkew 在 ±skew 个时间步范围内校验动态码。
func ValidateSkew(secret, code string, at time.Time, skew int) bool {
	code = normalizeCode(code)
	if len(code) != Digits {
		return false
	}
	key, err := decodeSecret(secret)
	if err != nil {
		return false
	}
	counter := at.Unix() / Period
	if skew < 0 {
		skew = 0
	}
	for delta := -skew; delta <= skew; delta++ {
		step := counter + int64(delta)
		if step < 0 {
			continue
		}
		expected := hotp(key, uint64(step), Digits)
		// 常量时间比较，避免通过响应时间推测验证码。
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// ProvisioningURL 生成认证器 App 可导入的 otpauth:// URI。
func ProvisioningURL(issuer, account, secret string) string {
	issuer = strings.TrimSpace(issuer)
	account = strings.TrimSpace(account)
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", strconv.Itoa(Digits))
	query.Set("period", strconv.Itoa(Period))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// QRCodeDataURL 把内容渲染为 PNG 二维码 data URL。
// 320px 在视网膜屏上足够清晰，又能直接内联到 JSON 响应中。
func QRCodeDataURL(content string) (string, error) {
	png, err := qrcode.Encode(content, qrcode.Medium, 320)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// GenerateRecoveryCodes 生成 10 个格式为 XXXX-XXXX 的一次性恢复码。
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, 0, RecoveryCodeCount)
	for i := 0; i < RecoveryCodeCount; i++ {
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			return nil, err
		}
		var b strings.Builder
		for j := 0; j < 8; j++ {
			b.WriteByte(recoveryAlphabet[int(raw[j])%len(recoveryAlphabet)])
		}
		code := b.String()
		codes = append(codes, code[:4]+"-"+code[4:])
	}
	return codes, nil
}

// NormalizeRecoveryCode 去除空格、制表符和短横线并转大写，兼容用户的不同输入方式。
func NormalizeRecoveryCode(code string) string {
	replacer := strings.NewReplacer("-", "", " ", "", "\t", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(code)))
}

// HashRecoveryCode 计算恢复码的 SHA-256 摘要。
// 恢复码本身具有足够熵，纯 SHA-256 摘要即可安全存储。
func HashRecoveryCode(code string) string {
	sum := sha256.Sum256([]byte(NormalizeRecoveryCode(code)))
	return hex.EncodeToString(sum[:])
}

// normalizeCode 去掉动态码中的空格、短横线和制表符。
func normalizeCode(code string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "\t", "")
	return replacer.Replace(strings.TrimSpace(code))
}

// decodeSecret 解码 Base32 密钥，容忍大小写、空格和填充符差异。
func decodeSecret(secret string) ([]byte, error) {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	s = strings.TrimRight(s, "=")
	key, err := b32.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("totp: invalid secret: %w", err)
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("totp: empty secret")
	}
	return key, nil
}

// hotp 实现 RFC 4226 的 HMAC-SHA1 动态截断算法。
func hotp(key []byte, counter uint64, digits int) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, value%mod)
}
