package augur

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// sampleSessionLine returns a minimal valid user-event JSON line with the given session content.
func writeTempProjectDir(t *testing.T, sessions map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	projDir := filepath.Join(dir, "project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range sessions {
		if err := os.WriteFile(filepath.Join(projDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestScanSessionMeta_EmptyDir(t *testing.T) {
	sessions, err := scanSessionMeta(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestScanSessionMeta_NonExistentDir(t *testing.T) {
	// WalkDir swallows the root error, so a missing dir returns empty with no error.
	sessions, err := scanSessionMeta("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestScanSessionMeta_ValidSessions(t *testing.T) {
	const s1 = `{"type":"user","sessionId":"aaa","timestamp":"2026-05-28T10:00:00.000Z","cwd":"/repo","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}`
	const s2 = `{"type":"user","sessionId":"bbb","timestamp":"2026-05-29T10:00:00.000Z","cwd":"/repo2","gitBranch":"feat","message":{"role":"user","content":[{"type":"text","text":"world"}]}}`

	dir := writeTempProjectDir(t, map[string]string{
		"session1.jsonl": s1,
		"session2.jsonl": s2,
		"notes.txt":      "ignored",
	})

	sessions, err := scanSessionMeta(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestScanSessionMeta_FiltersSubagents(t *testing.T) {
	const content = `{"type":"user","sessionId":"sub","timestamp":"2026-05-28T10:00:00.000Z","cwd":"/repo","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"hi"}]}}`

	dir := t.TempDir()
	subDir := filepath.Join(dir, "project", "subagents")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sub.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	sessions, err := scanSessionMeta(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (subagents filtered), got %d", len(sessions))
	}
}

func TestScanSessionMeta_SkipsUnreadableFile(t *testing.T) {
	const s1 = `{"type":"user","sessionId":"aaa","timestamp":"2026-05-28T10:00:00.000Z","cwd":"/repo","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"hello"}]}}`

	dir := writeTempProjectDir(t, map[string]string{
		"good.jsonl": s1,
		"bad.jsonl":  "not json at all",
	})

	sessions, err := scanSessionMeta(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// bad.jsonl has no user event, so only the good one is returned
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
}

func TestReadSessionMeta_BadTimestamp(t *testing.T) {
	const content = `{"type":"user","sessionId":"abc","timestamp":"not-a-timestamp","cwd":"/repo","gitBranch":"main","message":{}}`
	path := writeTempSession(t, content)
	_, err := readSessionMeta(path)
	if err == nil {
		t.Error("expected error for bad timestamp")
	}
}

const sampleSession = `{"type":"user","sessionId":"c95ad6fe-0000-0000-0000-000000000000","timestamp":"2026-05-28T10:00:00.000Z","cwd":"/home/user/myproject","gitBranch":"main","slug":"test-session","message":{"role":"user","content":[{"type":"text","text":"add error handling to auth"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","id":"t1","input":{"path":"/home/user/myproject/src/auth.go"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"file contents"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","id":"t2","input":{"path":"/home/user/myproject/src/auth.go","old_string":"func auth()","new_string":"func auth() error"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":""}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"Done! I've added error handling."}]}}
`

func writeTempSession(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadSessionMeta(t *testing.T) {
	path := writeTempSession(t, sampleSession)
	meta, err := readSessionMeta(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta == nil {
		t.Fatal("got nil meta")
	}
	if meta.ID != "c95ad6fe-0000-0000-0000-000000000000" {
		t.Errorf("wrong ID: %s", meta.ID)
	}
	if meta.CWD != "/home/user/myproject" {
		t.Errorf("wrong CWD: %s", meta.CWD)
	}
	if meta.Branch != "main" {
		t.Errorf("wrong branch: %s", meta.Branch)
	}
	want := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	if !meta.Timestamp.Equal(want) {
		t.Errorf("wrong timestamp: %v", meta.Timestamp)
	}
}

func TestReadSessionMeta_Empty(t *testing.T) {
	path := writeTempSession(t, "")
	meta, err := readSessionMeta(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta != nil {
		t.Fatalf("expected nil meta for empty file, got %+v", meta)
	}
}

func TestReadSessionMeta_NoUserEvent(t *testing.T) {
	path := writeTempSession(t, `{"type":"assistant","message":{"content":[]}}`)
	meta, err := readSessionMeta(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta != nil {
		t.Fatalf("expected nil for no user event")
	}
}

func TestParseTurns_BasicFlow(t *testing.T) {
	path := writeTempSession(t, sampleSession)
	turns, err := parseTurns(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: user turn (prompt), assistant turn (Edit)
	// (tool-result-only user events are filtered out)
	var userTurns, asstTurns int
	var foundPrompt string
	var foundEdit EditRef

	for _, tt := range turns {
		if tt.IsUser {
			userTurns++
			if tt.Prompt != "" {
				foundPrompt = tt.Prompt
			}
		} else {
			asstTurns++
			if len(tt.Edits) > 0 {
				foundEdit = tt.Edits[0]
			}
		}
	}

	if userTurns != 1 {
		t.Errorf("expected 1 user turn, got %d", userTurns)
	}
	if foundPrompt != "add error handling to auth" {
		t.Errorf("wrong prompt: %q", foundPrompt)
	}
	if foundEdit.Path != "/home/user/myproject/src/auth.go" {
		t.Errorf("wrong edit path: %s", foundEdit.Path)
	}
	if foundEdit.Tool != "Edit" {
		t.Errorf("wrong tool: %s", foundEdit.Tool)
	}
	_ = asstTurns
}

func TestParseTurns_MultiplePrompts(t *testing.T) {
	content := `{"type":"user","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","cwd":"/proj","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"first prompt"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","id":"w1","input":{"path":"/proj/main.go","content":"package main"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"w1","content":""}]}}
{"type":"user","message":{"role":"user","content":[{"type":"text","text":"second prompt"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","id":"e1","input":{"path":"/proj/util.go","old_string":"x","new_string":"y"}}]}}
`
	path := writeTempSession(t, content)
	turns, err := parseTurns(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pairs := buildPromptEdits(turns)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].Prompt != "first prompt" {
		t.Errorf("wrong first prompt: %q", pairs[0].Prompt)
	}
	if pairs[0].Edits[0].Path != "/proj/main.go" {
		t.Errorf("wrong first edit path: %s", pairs[0].Edits[0].Path)
	}
	if pairs[1].Prompt != "second prompt" {
		t.Errorf("wrong second prompt: %q", pairs[1].Prompt)
	}
	if pairs[1].Edits[0].Path != "/proj/util.go" {
		t.Errorf("wrong second edit path: %s", pairs[1].Edits[0].Path)
	}
}
