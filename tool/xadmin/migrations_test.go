package xadmin

import "testing"

func TestDetectMigrationIssuesExactCollision(t *testing.T) {
	base := Tree{"migrations/000001_init.up.sql": {Data: []byte("-- init")}}
	target := Tree{
		"migrations/000001_init.up.sql":    {Data: []byte("-- init")},
		"migrations/000002_billing.up.sql": {Data: []byte("-- billing")},
	}
	local := Tree{
		"migrations/000001_init.up.sql":         {Data: []byte("-- init")},
		"migrations/000002_announcement.up.sql": {Data: []byte("-- announcement")},
	}
	issues := detectMigrationIssues(base, target, local)
	if len(issues) != 1 {
		t.Fatalf("issues = %+v", issues)
	}
	if issues[0].Severity != "error" || issues[0].Number != 2 {
		t.Fatalf("unexpected issue: %+v", issues[0])
	}
}

func TestDetectMigrationIssuesLowerThanLocalMax(t *testing.T) {
	base := Tree{"migrations/000001_init.up.sql": {Data: []byte("-- init")}}
	target := Tree{
		"migrations/000001_init.up.sql":   {Data: []byte("-- init")},
		"migrations/000002_notify.up.sql": {Data: []byte("-- notify")},
	}
	local := Tree{
		"migrations/000001_init.up.sql":     {Data: []byte("-- init")},
		"migrations/000003_business.up.sql": {Data: []byte("-- business")},
	}
	issues := detectMigrationIssues(base, target, local)
	if len(issues) != 1 || issues[0].Severity != "warning" {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestDetectMigrationIssuesOrdered(t *testing.T) {
	base := Tree{"migrations/000001_init.up.sql": {Data: []byte("-- init")}}
	target := Tree{
		"migrations/000001_init.up.sql":   {Data: []byte("-- init")},
		"migrations/000004_notify.up.sql": {Data: []byte("-- notify")},
	}
	local := Tree{
		"migrations/000001_init.up.sql":     {Data: []byte("-- init")},
		"migrations/000003_business.up.sql": {Data: []byte("-- business")},
	}
	if issues := detectMigrationIssues(base, target, local); len(issues) != 0 {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestMigrationPairs(t *testing.T) {
	tree := Tree{
		"migrations/000001_init.up.sql":   {Data: []byte("-- up")},
		"migrations/000001_init.down.sql": {Data: []byte("-- down")},
		"migrations/000002_x.up.sql":      {Data: []byte("-- up")},
	}
	problems := MigrationPairs(tree)
	if len(problems) != 1 {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestNextMigrationNumber(t *testing.T) {
	tree := Tree{
		"migrations/000001_init.up.sql": {Data: []byte("-- up")},
		"migrations/000007_x.up.sql":    {Data: []byte("-- up")},
	}
	if got := NextMigrationNumber(tree); got != 8 {
		t.Fatalf("next = %d, want 8", got)
	}
}
