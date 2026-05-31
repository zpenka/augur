package augur

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsRepoSession(t *testing.T) {
	sep := string(os.PathSeparator)
	root := sep + "home" + sep + "user" + sep + "myproject"

	cases := []struct {
		cwd  string
		root string
		want bool
	}{
		{root, root, true},
		{root + sep + "src", root, true},
		{root + sep + "src" + sep + "deep", root, true},
		{sep + "home" + sep + "user" + sep + "other", root, false},
		{"", root, false},
		{root, "", false},
	}

	for _, c := range cases {
		got := isRepoSession(c.cwd, c.root)
		if got != c.want {
			t.Errorf("isRepoSession(%q, %q) = %v, want %v", c.cwd, c.root, got, c.want)
		}
	}
}

func TestSameFile(t *testing.T) {
	if !sameFile("/a/b/c.go", "/a/b/c.go") {
		t.Error("same path should match")
	}
	if !sameFile("/a/b/../b/c.go", "/a/b/c.go") {
		t.Error("equivalent paths should match")
	}
	if sameFile("/a/b/c.go", "/a/b/d.go") {
		t.Error("different paths should not match")
	}
}

func TestBuildPromptEdits_Basic(t *testing.T) {
	turns := []ParsedTurn{
		{IsUser: true, Prompt: "do the thing"},
		{IsUser: false, Edits: []EditRef{{Path: "/proj/a.go", Tool: "Edit"}}},
	}
	pairs := buildPromptEdits(turns)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Prompt != "do the thing" {
		t.Errorf("wrong prompt: %q", pairs[0].Prompt)
	}
	if pairs[0].Edits[0].Path != "/proj/a.go" {
		t.Errorf("wrong edit path")
	}
}

func TestBuildPromptEdits_NoEdits(t *testing.T) {
	// Prompt without any following edits → no pair produced.
	turns := []ParsedTurn{
		{IsUser: true, Prompt: "explain the code"},
	}
	pairs := buildPromptEdits(turns)
	if len(pairs) != 0 {
		t.Fatalf("expected 0 pairs, got %d", len(pairs))
	}
}

func TestBuildPromptEdits_Empty(t *testing.T) {
	pairs := buildPromptEdits(nil)
	if len(pairs) != 0 {
		t.Fatalf("expected 0 pairs for nil input")
	}
}

func TestFindMatch_NoSessions(t *testing.T) {
	m, err := findMatch(nil, "/repo", "/repo/main.go", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("expected nil match with no sessions")
	}
}

func TestFindMatch_SessionOutsideWindow(t *testing.T) {
	commitTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	oldSession := SessionMeta{
		ID:        "old",
		CWD:       "/repo",
		Timestamp: commitTime.Add(-10 * 24 * time.Hour), // 10 days before commit
	}
	m, err := findMatch([]SessionMeta{oldSession}, "/repo", "/repo/main.go", commitTime)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("expected nil: session is outside the 7-day window")
	}
}

func TestFindMatch_SessionAfterCommit(t *testing.T) {
	commitTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	futureSession := SessionMeta{
		ID:        "future",
		CWD:       "/repo",
		Timestamp: commitTime.Add(2 * time.Hour), // 2h after commit
	}
	m, err := findMatch([]SessionMeta{futureSession}, "/repo", "/repo/main.go", commitTime)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("expected nil: session starts after commit")
	}
}

func TestFindMatch_HitsFile(t *testing.T) {
	const sessionContent = `{"type":"user","sessionId":"abc","timestamp":"2026-05-28T10:00:00.000Z","cwd":"/repo","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"fix the handler"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","id":"e1","input":{"path":"/repo/main.go","old_string":"x","new_string":"y"}}]}}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(sessionContent), 0600); err != nil {
		t.Fatal(err)
	}

	commitTime := time.Date(2026, 5, 28, 14, 0, 0, 0, time.UTC)
	sessions := []SessionMeta{
		{
			ID:        "abc",
			Path:      path,
			CWD:       "/repo",
			Branch:    "main",
			Timestamp: time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
		},
	}

	m, err := findMatch(sessions, "/repo", "/repo/main.go", commitTime)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected a match")
	}
	if m.Prompt != "fix the handler" {
		t.Errorf("wrong prompt: %q", m.Prompt)
	}
	if m.TurnIndex != 1 {
		t.Errorf("expected turn 1, got %d", m.TurnIndex)
	}
}

func TestFindMatch_ParseTurnsError(t *testing.T) {
	// Session path points to a nonexistent file — parseTurns will error, session is skipped.
	commitTime := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	sessions := []SessionMeta{
		{
			ID:        "bad",
			Path:      "/nonexistent/session.jsonl",
			CWD:       "/repo",
			Timestamp: commitTime.Add(-time.Hour),
		},
	}
	m, err := findMatch(sessions, "/repo", "/repo/main.go", commitTime)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("expected nil when session file is unreadable")
	}
}

func TestFindMatch_RelativeEditPath(t *testing.T) {
	const sessionContent = `{"type":"user","sessionId":"abc","timestamp":"2026-05-28T10:00:00.000Z","cwd":"/repo","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"fix it"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","id":"e1","input":{"path":"main.go","old_string":"x","new_string":"y"}}]}}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(sessionContent), 0600); err != nil {
		t.Fatal(err)
	}

	commitTime := time.Date(2026, 5, 28, 14, 0, 0, 0, time.UTC)
	sessions := []SessionMeta{{
		ID:        "abc",
		Path:      path,
		CWD:       "/repo",
		Timestamp: time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
	}}

	m, err := findMatch(sessions, "/repo", "/repo/main.go", commitTime)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected match: relative edit path 'main.go' + CWD '/repo' should match '/repo/main.go'")
	}
	if m.Prompt != "fix it" {
		t.Errorf("wrong prompt: %q", m.Prompt)
	}
}

func TestFindMatch_RelativeEditPathSubdir(t *testing.T) {
	const sessionContent = `{"type":"user","sessionId":"abc","timestamp":"2026-05-28T10:00:00.000Z","cwd":"/repo","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"fix auth"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","id":"e1","input":{"path":"src/auth.go","old_string":"x","new_string":"y"}}]}}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(sessionContent), 0600); err != nil {
		t.Fatal(err)
	}

	commitTime := time.Date(2026, 5, 28, 14, 0, 0, 0, time.UTC)
	sessions := []SessionMeta{{
		ID:        "abc",
		Path:      path,
		CWD:       "/repo",
		Timestamp: time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
	}}

	m, err := findMatch(sessions, "/repo", "/repo/src/auth.go", commitTime)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected match: relative edit 'src/auth.go' + CWD '/repo' should match '/repo/src/auth.go'")
	}
}

func TestSameFile_Symlink(t *testing.T) {
	real := t.TempDir()
	linkDir := filepath.Join(t.TempDir(), "symlink")
	if err := os.Symlink(real, linkDir); err != nil {
		t.Skip("cannot create symlink:", err)
	}

	realFile := filepath.Join(real, "test.go")
	if err := os.WriteFile(realFile, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	linkedFile := filepath.Join(linkDir, "test.go")

	if !sameFile(realFile, linkedFile) {
		t.Errorf("symlinked paths should be considered equal: %q vs %q", realFile, linkedFile)
	}
}

func TestIsRepoSession_Symlink(t *testing.T) {
	real := t.TempDir()
	linkDir := filepath.Join(t.TempDir(), "symlink")
	if err := os.Symlink(real, linkDir); err != nil {
		t.Skip("cannot create symlink:", err)
	}

	subdir := filepath.Join(real, "src")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	if !isRepoSession(linkDir, real) {
		t.Errorf("symlinked CWD should match real root: %q vs %q", linkDir, real)
	}
	if !isRepoSession(subdir, linkDir) {
		t.Errorf("real subdir should match symlinked root: %q vs %q", subdir, linkDir)
	}
}

func TestFindMatch_WrongFile(t *testing.T) {
	const sessionContent = `{"type":"user","sessionId":"abc","timestamp":"2026-05-28T10:00:00.000Z","cwd":"/repo","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"fix the handler"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","id":"e1","input":{"path":"/repo/other.go","old_string":"x","new_string":"y"}}]}}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(sessionContent), 0600); err != nil {
		t.Fatal(err)
	}

	commitTime := time.Date(2026, 5, 28, 14, 0, 0, 0, time.UTC)
	sessions := []SessionMeta{{
		ID:        "abc",
		Path:      path,
		CWD:       "/repo",
		Timestamp: time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
	}}

	m, err := findMatch(sessions, "/repo", "/repo/main.go", commitTime)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("expected nil: session edited a different file")
	}
}
