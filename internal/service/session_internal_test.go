package service

import "testing"
import "github.com/stretchr/testify/assert"

func TestDescribeDevice(t *testing.T) {
	cases := []struct {
		ua   string
		want string
	}{
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) AppleWebKit/605.1.15 Version/17.0 Mobile Safari/604.1", "iPhone · Safari"},
		{"Mozilla/5.0 (iPad; CPU OS 17_0) AppleWebKit/605.1.15 Version/17.0 Safari/604.1", "iPad · Safari"},
		{"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36", "Android · Chrome"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 Chrome/120.0 Safari/537.36", "macOS · Chrome"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Edg/120.0", "Windows · Edge"},
		{"Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/121.0", "Linux · Firefox"},
		{"curl/8.4.0", "未知设备"},
		{"Mozilla/5.0 (Windows NT 10.0)", "Windows"},
		{"Mozilla/5.0 Chrome/120.0", "Chrome"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, describeDevice(tc.ua), tc.ua)
	}
}
