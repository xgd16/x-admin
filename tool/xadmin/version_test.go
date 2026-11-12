package xadmin

import "testing"

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.3.0", "v1.4.0", -1},
		{"v1.4.0", "v1.3.9", 1},
		{"v1.4.0", "1.4.0", 0},
		{"v1.4.0", "v1.4.0-rc.1", 1},
		{"v1.4.0-rc.2", "v1.4.0-rc.1", 1},
		{"v1.3.0-5-gabc", "v1.3.0", 0},
		{"v2.0.0", "v10.0.0", -1},
		{"dev", "dev", 0},
	}
	for _, tt := range tests {
		if got := CompareVersion(tt.a, tt.b); got != tt.want {
			t.Errorf("CompareVersion(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	v, ok := ParseVersion("v1.2.3-rc.1")
	if !ok {
		t.Fatal("parse failed")
	}
	if v.Major != 1 || v.Minor != 2 || v.Patch != 3 || v.Pre != "rc.1" {
		t.Fatalf("unexpected version: %+v", v)
	}
	if _, ok := ParseVersion("not-a-version"); ok {
		t.Fatal("should fail")
	}
}

func TestIsSHA(t *testing.T) {
	if !IsSHA("9434cd61673aa4e619bb8c9778838601c11f394c") {
		t.Fatal("valid sha rejected")
	}
	if IsSHA("v1.3.0") || IsSHA("9434cd6") {
		t.Fatal("invalid sha accepted")
	}
}

func TestIsUpToDate(t *testing.T) {
	if !IsUpToDate("v1.4.0", "v1.4.0") {
		t.Fatal("same version should be up to date")
	}
	if IsUpToDate("v1.3.0", "v1.4.0") {
		t.Fatal("older version should not be up to date")
	}
	if IsUpToDate("", "v1.4.0") {
		t.Fatal("empty current should not be up to date")
	}
}
