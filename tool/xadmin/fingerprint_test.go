package xadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testTag(name, sha string) Tag {
	var tag Tag
	tag.Name = name
	tag.Commit.SHA = sha
	return tag
}

func TestLocalBlobSHA(t *testing.T) {
	// git hash-object -w --stdin <<< "hello"
	const want = "ce013625030ba8dba906f756967f9e9ca394464a"
	if got := localBlobSHA([]byte("hello\n")); got != want {
		t.Fatalf("localBlobSHA = %q, want %q", got, want)
	}
	if got := localBlobSHA([]byte("hello\r\n")); got != want {
		t.Fatalf("CRLF should normalize, got %q", got)
	}
}

func TestScoreTree(t *testing.T) {
	local := testTree(map[string]string{
		"a.txt":          "a\n",
		"b.txt":          "b\n",
		StatePath:        `{"ref":"v1.3.0"}`,
		VersionPath:      "v1.3.0\n",
		"local-only.txt": "x\n",
	})
	upstream := map[string]string{
		"a.txt":     localBlobSHA([]byte("a\n")),
		"b.txt":     localBlobSHA([]byte("b\n")),
		"c.txt":     localBlobSHA([]byte("c\n")),
		StatePath:   "ignored",
		VersionPath: "ignored",
	}
	matched, total := scoreTree(local, upstream)
	if matched != 2 || total != 3 {
		t.Fatalf("scoreTree = %d/%d, want 2/3", matched, total)
	}
}

func TestSelectCandidateTags(t *testing.T) {
	tags := []Tag{
		testTag("v1.4.0", "c4"),
		testTag("v1.3.0", "c3"),
		testTag("v1.2.0", "c2"),
		testTag("docs", "cd"),
	}
	got := selectCandidateTags(tags, "v1.3.0", 10)
	if len(got) != 2 || got[0].Name != "v1.3.0" || got[1].Name != "v1.2.0" {
		t.Fatalf("unexpected candidates: %+v", got)
	}
}

func TestDetectBaseFingerprint(t *testing.T) {
	local := testTree(map[string]string{
		"a.txt": "a\n",
		"b.txt": "b\n",
	})
	shaA := localBlobSHA([]byte("a\n"))
	shaB := localBlobSHA([]byte("b\n"))

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/tags", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]Tag{
			testTag("v1.3.0", "c3"),
			testTag("v1.2.0", "c2"),
		})
	})
	mux.HandleFunc("/repos/o/r/git/commits/c3", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"tree":{"sha":"t3"}}`)
	})
	mux.HandleFunc("/repos/o/r/git/commits/c2", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"tree":{"sha":"t2"}}`)
	})
	mux.HandleFunc("/repos/o/r/git/trees/t3", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tree": []TreeEntry{
				{Path: "a.txt", Type: "blob", SHA: shaA},
				{Path: "b.txt", Type: "blob", SHA: shaB},
			},
		})
	})
	mux.HandleFunc("/repos/o/r/git/trees/t2", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tree": []TreeEntry{
				{Path: "a.txt", Type: "blob", SHA: localBlobSHA([]byte("old\n"))},
				{Path: "b.txt", Type: "blob", SHA: shaB},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewGitHubClient("o/r", "")
	client.APIBase = server.URL
	client.HTTP = server.Client()

	result, err := DetectBaseFingerprint(context.Background(), client, local, "v1.3.0", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Detected || result.Tag != "v1.3.0" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Score != 1 {
		t.Fatalf("score = %f, want 1", result.Score)
	}
}
