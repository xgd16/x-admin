package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/totp"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

type loginPayload struct {
	MFARequired  bool   `json:"mfa_required"`
	MFAToken     string `json:"mfa_token"`
	MFAExpiresIn int64  `json:"mfa_expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func captchaFor(t *testing.T, app *testsupport.App) (string, string) {
	t.Helper()
	rr := app.Do(http.MethodGet, "/admin/v1/auth/captcha", nil, "")
	env := testsupport.Decode(t, rr)
	require.Equal(t, 0, env.Code, rr.Body.String())
	var challenge captcha.Challenge
	require.NoError(t, json.Unmarshal(env.Data, &challenge))
	answer, err := app.Cache.Get(context.Background(), "captcha:"+challenge.ID)
	require.NoError(t, err)
	return challenge.ID, answer
}

func passwordLogin(t *testing.T, app *testsupport.App, username, password string) loginPayload {
	t.Helper()
	captchaID, captchaCode := captchaFor(t, app)
	rr := app.Do(http.MethodPost, "/admin/v1/auth/login", map[string]string{
		"username": username, "password": password,
		"captcha_id": captchaID, "captcha_code": captchaCode,
	}, "")
	env := testsupport.Decode(t, rr)
	require.Equal(t, 0, env.Code, rr.Body.String())
	var payload loginPayload
	require.NoError(t, json.Unmarshal(env.Data, &payload))
	return payload
}

func mfaLogin(t *testing.T, app *testsupport.App, challenge, code string) (int, string) {
	t.Helper()
	rr := app.Do(http.MethodPost, "/admin/v1/auth/login/mfa", map[string]string{
		"mfa_token": challenge, "otp_code": code,
	}, "")
	env := testsupport.Decode(t, rr)
	if env.Code != 0 {
		return env.Code, ""
	}
	var payload loginPayload
	require.NoError(t, json.Unmarshal(env.Data, &payload))
	return 0, payload.AccessToken
}

func wrongCode(t *testing.T, secret string) string {
	t.Helper()
	current, err := totp.Code(secret, time.Now())
	require.NoError(t, err)
	if current == "000000" {
		return "111111"
	}
	return "000000"
}

func TestTOTPEnrollmentLoginAndDisable(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	initial := app.Login(t, "admin", "Admin@123456")
	token := initial.AccessToken
	// A second device that logged in before 2FA was enabled.
	other := app.Login(t, "admin", "Admin@123456")
	ctx := context.Background()

	// Initial status.
	rr := app.Do(http.MethodGet, "/admin/v1/profile/totp", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	var status service.TOTPStatus
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &status))
	assert.False(t, status.Enabled)
	assert.Equal(t, 0, status.RecoveryCodesLeft)

	// Setup returns a scannable QR payload and stores a pending secret.
	rr = app.Do(http.MethodPost, "/admin/v1/profile/totp/setup", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	var setup service.TOTPSetup
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &setup))
	require.NotEmpty(t, setup.Secret)
	assert.Contains(t, setup.OtpauthURL, "otpauth://totp/")
	assert.True(t, strings.HasPrefix(setup.QRCode, "data:image/png;base64,"), "qr should be a PNG data URL")
	assert.Equal(t, "admin", setup.Account)

	code, err := totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)

	// Password is required so a stolen session cannot silently enroll.
	rr = app.Do(http.MethodPost, "/admin/v1/profile/totp/enable", map[string]string{
		"password": "wrong", "otp_code": code,
	}, token)
	assert.Equal(t, 40001, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodPost, "/admin/v1/profile/totp/enable", map[string]string{
		"password": "Admin@123456", "otp_code": wrongCode(t, setup.Secret),
	}, token)
	assert.Equal(t, 40001, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodPost, "/admin/v1/profile/totp/enable", map[string]string{
		"password": "Admin@123456", "otp_code": code, "refresh_token": initial.RefreshToken,
	}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	var enabled struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &enabled))
	require.Len(t, enabled.RecoveryCodes, totp.RecoveryCodeCount)

	// Other devices' refresh tokens are invalidated; the enrolling session is kept.
	rotated, err := app.Services.Auth.Refresh(ctx, initial.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated.AccessToken)
	_, err = app.Services.Auth.Refresh(ctx, other.RefreshToken)
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	rr = app.Do(http.MethodGet, "/admin/v1/profile/totp", nil, token)
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &status))
	assert.True(t, status.Enabled)
	assert.Equal(t, totp.RecoveryCodeCount, status.RecoveryCodesLeft)

	// Password login now stops at the MFA challenge and issues no tokens.
	first := passwordLogin(t, app, "admin", "Admin@123456")
	require.True(t, first.MFARequired)
	require.NotEmpty(t, first.MFAToken)
	assert.Empty(t, first.AccessToken)
	assert.Greater(t, first.MFAExpiresIn, int64(0))

	code, err = totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	codeCode, access := mfaLogin(t, app, first.MFAToken, code)
	require.Zero(t, codeCode)
	require.NotEmpty(t, access)
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, access)
	assert.Equal(t, 0, testsupport.Decode(t, rr).Code)

	// A recovery code works exactly once.
	second := passwordLogin(t, app, "admin", "Admin@123456")
	require.True(t, second.MFARequired)
	codeCode, access = mfaLogin(t, app, second.MFAToken, enabled.RecoveryCodes[0])
	require.Zero(t, codeCode)
	require.NotEmpty(t, access)

	third := passwordLogin(t, app, "admin", "Admin@123456")
	require.True(t, third.MFARequired)
	codeCode, _ = mfaLogin(t, app, third.MFAToken, enabled.RecoveryCodes[0])
	assert.Equal(t, 40101, codeCode)

	// Disable requires the password and a valid code.
	rr = app.Do(http.MethodDelete, "/admin/v1/profile/totp", map[string]string{
		"password": "Admin@123456", "otp_code": wrongCode(t, setup.Secret),
	}, token)
	assert.Equal(t, 40001, testsupport.Decode(t, rr).Code)

	code, err = totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	rr = app.Do(http.MethodDelete, "/admin/v1/profile/totp", map[string]string{
		"password": "Admin@123456", "otp_code": code,
	}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())

	// Password-only login works again.
	again := passwordLogin(t, app, "admin", "Admin@123456")
	assert.False(t, again.MFARequired)
	assert.NotEmpty(t, again.AccessToken)
}

func TestTOTPChallengeLimitsAttempts(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken

	rr := app.Do(http.MethodPost, "/admin/v1/profile/totp/setup", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	var setup service.TOTPSetup
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &setup))
	code, err := totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	rr = app.Do(http.MethodPost, "/admin/v1/profile/totp/enable", map[string]string{
		"password": "Admin@123456", "otp_code": code,
	}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	_ = admin

	first := passwordLogin(t, app, "admin", "Admin@123456")
	require.True(t, first.MFARequired)
	for i := 0; i < 5; i++ {
		codeCode, _ := mfaLogin(t, app, first.MFAToken, wrongCode(t, setup.Secret))
		assert.NotZero(t, codeCode)
	}
	// The challenge is burned after the fifth failure.
	codeCode, _ := mfaLogin(t, app, first.MFAToken, wrongCode(t, setup.Secret))
	assert.Equal(t, 40102, codeCode)
}
