package augur

import (
	"os"
	"path/filepath"
	"testing"
)

// Integration tests that exercise git-backed functions against the real augur repository.
// They require a committed file to blame and will skip if the working directory is unexpected.

func repoFile(t *testing.T, name string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Skip("cannot determine working directory")
	}
	return filepath.Join(wd, name)
}

// --- gitRoot ---

func TestGitRoot_RepoFile(t *testing.T) {
	wd, _ := os.Getwd()
	root, err := gitRoot(filepath.Join(wd, "augur.go"))
	if err != nil {
		t.Fatalf("gitRoot failed: %v", err)
	}
	if root != wd {
		t.Errorf("expected %q, got %q", wd, root)
	}
}

func TestGitRoot_NonRepoPath(t *testing.T) {
	// TempDir is outside any git repo.
	dir := t.TempDir()
	_, err := gitRoot(filepath.Join(dir, "file.go"))
	if err == nil {
		t.Error("expected error for path outside git repo")
	}
}

// --- blameLine ---

func TestBlameLine_RealFile(t *testing.T) {
	b, err := blameLine(repoFile(t, "augur.go"), 1)
	if err != nil {
		t.Fatalf("blameLine failed: %v", err)
	}
	if b.CommitHash == "" {
		t.Error("expected non-empty commit hash")
	}
	if b.Author == "" {
		t.Error("expected non-empty author")
	}
}

func TestBlameLine_InvalidFile(t *testing.T) {
	_, err := blameLine("/nonexistent/path/file.go", 1)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// --- blameFile ---

func TestBlameFile_RealFile(t *testing.T) {
	m, err := blameFile(repoFile(t, "augur.go"))
	if err != nil {
		t.Fatalf("blameFile failed: %v", err)
	}
	if len(m) == 0 {
		t.Error("expected blame entries")
	}
	// Line 1 must exist.
	if _, ok := m[1]; !ok {
		t.Error("expected blame entry for line 1")
	}
}

func TestBlameFile_InvalidFile(t *testing.T) {
	_, err := blameFile("/nonexistent/path/file.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// --- lookupLine ---

func TestLookupLine_RealFile(t *testing.T) {
	absFile := repoFile(t, "augur.go")
	result, err := lookupLine(absFile, 1, t.TempDir())
	if err != nil {
		t.Fatalf("lookupLine failed: %v", err)
	}
	if result.Blame == nil {
		t.Fatal("expected blame info")
	}
	if result.Line != 1 {
		t.Errorf("expected line 1, got %d", result.Line)
	}
	// Empty claudeDir → no session match.
	if result.Match != nil {
		t.Error("expected no match with empty claudeDir")
	}
}

func TestLookupLine_InvalidFile(t *testing.T) {
	_, err := lookupLine("/nonexistent/path/file.go", 1, t.TempDir())
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// --- lookupFile ---

func TestLookupFile_RealFile(t *testing.T) {
	absFile := repoFile(t, "augur.go")
	result, err := lookupFile(absFile, t.TempDir())
	if err != nil {
		t.Fatalf("lookupFile failed: %v", err)
	}
	if len(result.Regions) == 0 {
		t.Error("expected at least one region")
	}
	for _, reg := range result.Regions {
		if reg.Blame == nil {
			t.Error("each region should have blame info")
		}
	}
}

func TestLookupFile_InvalidFile(t *testing.T) {
	_, err := lookupFile("/nonexistent/path/file.go", t.TempDir())
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLookupFile_WithSessionMatch(t *testing.T) {
	// Write a session that edited augur.go so the match cache path is exercised.
	absFile := repoFile(t, "augur.go")

	// Use the real blame to get the commit hash for line 1.
	blame, err := blameLine(absFile, 1)
	if err != nil {
		t.Fatalf("blameLine: %v", err)
	}

	sessionContent := `{"type":"user","sessionId":"integ1","timestamp":"2020-01-01T00:00:00.000Z","cwd":"` +
		filepath.Dir(absFile) + `","gitBranch":"main","message":{"role":"user","content":[{"type":"text","text":"init commit"}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","id":"e1","input":{"path":"` +
		absFile + `","old_string":"x","new_string":"y"}}]}}
`
	dir := t.TempDir()
	projDir := filepath.Join(dir, "proj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "s.jsonl"), []byte(sessionContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// The session timestamp is in 2020 — far before any commit in this repo — so no match
	// is expected, but the match-cache path through lookupFile is exercised.
	result, err := lookupFile(absFile, dir)
	if err != nil {
		t.Fatalf("lookupFile failed: %v", err)
	}
	if len(result.Regions) == 0 {
		t.Error("expected at least one region")
	}
	_ = blame
}
