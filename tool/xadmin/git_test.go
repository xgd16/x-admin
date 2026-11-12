package xadmin

import "testing"

func TestRepoFromRemote(t *testing.T) {
	tests := []struct {
		remote string
		want   string
	}{
		{"git@github.com:xgd16/x-admin.git", "xgd16/x-admin"},
		{"https://github.com/xgd16/x-admin.git", "xgd16/x-admin"},
		{"https://github.com/xgd16/x-admin", "xgd16/x-admin"},
		{"git@github.com:owner/my.repo.git", "owner/my.repo"},
		{"https://gitlab.com/owner/repo.git", ""},
	}
	for _, tt := range tests {
		if got := RepoFromRemote(tt.remote); got != tt.want {
			t.Errorf("RepoFromRemote(%q) = %q, want %q", tt.remote, got, tt.want)
		}
	}
}

func TestDefaultBranchName(t *testing.T) {
	tests := map[string]string{
		"v1.4.0":       "chore/upgrade-xadmin-v1.4.0",
		"main":         "chore/upgrade-xadmin-main",
		"release/v1.4": "chore/upgrade-xadmin-release-v1.4",
		"local:target": "chore/upgrade-xadmin-target",
	}
	for ref, want := range tests {
		if got := defaultBranchName(ref); got != want {
			t.Errorf("defaultBranchName(%q) = %q, want %q", ref, got, want)
		}
	}
}
