package augur

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestShort8(t *testing.T) {
	if short8("abcdefghij") != "abcdefgh" {
		t.Error("should truncate to 8 chars")
	}
	if short8("abc") != "abc" {
		t.Error("should not truncate short strings")
	}
	if short8("") != "" {
		t.Error("should handle empty string")
	}
	if short8("12345678") != "12345678" {
		t.Error("exactly 8 chars should be unchanged")
	}
}

func TestHumanTime_Ranges(t *testing.T) {
	now := time.Now()
	cases := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5 minutes ago"},
		{now.Add(-3 * time.Hour), "3 hours ago"},
		{now.Add(-5 * 24 * time.Hour), "5 days ago"},
		{now.Add(-3 * 7 * 24 * time.Hour), "3 weeks ago"},
		{now.Add(-3 * 30 * 24 * time.Hour), "3 months ago"},
	}
	for _, c := range cases {
		got := humanTime(c.t)
		if got != c.want {
			t.Errorf("humanTime(%v ago) = %q, want %q", time.Since(c.t).Round(time.Second), got, c.want)
		}
	}
}

func TestRelPath_UnderCWD(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Skip("can't get working dir")
	}
	got := relPath(wd + "/augur.go")
	if got != "augur.go" {
		t.Errorf("expected augur.go, got %q", got)
	}
}

func TestRelPath_Outside(t *testing.T) {
	// A path relative to CWD should still return something (../../... form)
	got := relPath("/tmp/some/other/file.go")
	if got == "" {
		t.Error("expected non-empty path")
	}
}

func TestPrintLineResultText_WithMatch(t *testing.T) {
	r := &LineResult{
		File: "/repo/src/auth.go",
		Line: 42,
		Blame: &BlameInfo{
			CommitHash: "abc1234567890abcdef1234567890abcdef12345",
			Author:     "Alice",
			AuthorTime: time.Now().Add(-2 * time.Hour),
			Summary:    "Add auth handler",
		},
		Match: &MatchResult{
			Session: SessionMeta{
				ID:        "sess1234567890abcdef1234567890abcdef12345",
				Timestamp: time.Now(),
				Branch:    "main",
			},
			Prompt:     "implement auth",
			TurnIndex:  2,
			TotalTurns: 5,
		},
	}
	out := captureStdout(t, func() { printLineResultText(r, false) })

	for _, want := range []string{"abc12345", "Alice", "Add auth handler", "sess1234", "main", "implement auth", "2 of 5"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintLineResultText_NoMatch(t *testing.T) {
	r := &LineResult{
		File:  "/repo/src/auth.go",
		Line:  10,
		Blame: &BlameInfo{CommitHash: "abc1234567890abcdef1234567890abcdef12345"},
	}
	out := captureStdout(t, func() { printLineResultText(r, false) })
	if !strings.Contains(out, "no session match") {
		t.Errorf("expected 'no session match' in output:\n%s", out)
	}
}

func TestPrintLineResultText_NilBlame(t *testing.T) {
	r := &LineResult{File: "/repo/main.go", Line: 1}
	out := captureStdout(t, func() { printLineResultText(r, false) })
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestPrintLineResultText_NoSessionBranch(t *testing.T) {
	r := &LineResult{
		File: "/repo/main.go",
		Line: 1,
		Blame: &BlameInfo{
			CommitHash: "abc1234567890abcdef1234567890abcdef12345",
			AuthorTime: time.Now().Add(-time.Hour),
		},
		Match: &MatchResult{
			Session:    SessionMeta{ID: "sess1234567890abcdef", Timestamp: time.Now()},
			Prompt:     "do thing",
			TurnIndex:  1,
			TotalTurns: 1,
		},
	}
	out := captureStdout(t, func() { printLineResultText(r, false) })
	// Branch field is empty — branch line should be absent
	if strings.Contains(out, "branch") {
		t.Errorf("expected no branch line when Branch is empty:\n%s", out)
	}
}

func TestPrintFileResultText_WithMatch(t *testing.T) {
	r := &FileResult{
		File: "/repo/main.go",
		Regions: []Region{
			{
				StartLine: 1,
				EndLine:   5,
				Blame: &BlameInfo{
					CommitHash: "abc1234567890abcdef1234567890abcdef12345",
					Author:     "Bob",
					AuthorTime: time.Now().Add(-24 * time.Hour),
				},
				Match: &MatchResult{
					Session: SessionMeta{ID: "s1", Timestamp: time.Now()},
					Prompt:  "fix the handler",
				},
			},
		},
	}
	out := captureStdout(t, func() { printFileResultText(r, false) })
	if !strings.Contains(out, "fix the handler") {
		t.Errorf("expected prompt in output:\n%s", out)
	}
	if !strings.Contains(out, "1-5") {
		t.Errorf("expected line range in output:\n%s", out)
	}
}

func TestPrintFileResultText_NoMatch_WithAuthor(t *testing.T) {
	r := &FileResult{
		File: "/repo/main.go",
		Regions: []Region{
			{
				StartLine: 1,
				EndLine:   1,
				Blame: &BlameInfo{
					CommitHash: "abc1234567890abcdef1234567890abcdef12345",
					Author:     "Carol",
					AuthorTime: time.Now().Add(-time.Hour),
				},
			},
		},
	}
	out := captureStdout(t, func() { printFileResultText(r, false) })
	if !strings.Contains(out, "no match (Carol)") {
		t.Errorf("expected 'no match (Carol)' in output:\n%s", out)
	}
}

func TestPrintFileResultText_NoMatch_NoAuthor(t *testing.T) {
	r := &FileResult{
		File: "/repo/main.go",
		Regions: []Region{
			{
				StartLine: 3,
				EndLine:   7,
				Blame:     &BlameInfo{CommitHash: "abc1234567890abcdef1234567890abcdef12345"},
			},
		},
	}
	out := captureStdout(t, func() { printFileResultText(r, false) })
	if !strings.Contains(out, "no match") {
		t.Errorf("expected 'no match' in output:\n%s", out)
	}
	// Should not print "()" empty parens
	if strings.Contains(out, "()") {
		t.Errorf("unexpected empty parens in output:\n%s", out)
	}
}

func TestPrintFileResultText_NilBlame(t *testing.T) {
	r := &FileResult{
		File: "/repo/main.go",
		Regions: []Region{
			{StartLine: 1, EndLine: 1},
		},
	}
	out := captureStdout(t, func() { printFileResultText(r, false) })
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestPrintFileResultText_SingleLineRange(t *testing.T) {
	r := &FileResult{
		File: "/repo/main.go",
		Regions: []Region{
			{
				StartLine: 5,
				EndLine:   5,
				Blame:     &BlameInfo{CommitHash: "abc1234567890abcdef1234567890abcdef12345"},
			},
		},
	}
	out := captureStdout(t, func() { printFileResultText(r, false) })
	// Single-line region should show "5" not "5-5"
	if strings.Contains(out, "5-5") {
		t.Errorf("single-line region should not show range:\n%s", out)
	}
}

func TestPrintLineResult_JSON(t *testing.T) {
	r := &LineResult{
		File: "/repo/main.go",
		Line: 1,
		Blame: &BlameInfo{
			CommitHash: "abc1234567890abcdef1234567890abcdef12345",
			Author:     "Dev",
			AuthorTime: time.Unix(1748000000, 0).UTC(),
			Summary:    "init",
		},
	}
	out := captureStdout(t, func() { printLineResult(r, true, false) })
	var got LineResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if got.Line != 1 {
		t.Errorf("wrong line: %d", got.Line)
	}
}

func TestPrintLineResult_Text(t *testing.T) {
	r := &LineResult{
		File:  "/repo/main.go",
		Line:  1,
		Blame: &BlameInfo{CommitHash: "abc1234567890abcdef1234567890abcdef12345"},
	}
	out := captureStdout(t, func() { printLineResult(r, false, false) })
	if out == "" {
		t.Error("expected non-empty text output")
	}
}

func TestPrintFileResult_JSON(t *testing.T) {
	r := &FileResult{
		File: "/repo/main.go",
		Regions: []Region{
			{StartLine: 1, EndLine: 3, Blame: &BlameInfo{CommitHash: "abc1234567890abcdef1234567890abcdef12345"}},
		},
	}
	out := captureStdout(t, func() { printFileResult(r, true, false) })
	var got FileResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(got.Regions) != 1 {
		t.Errorf("expected 1 region, got %d", len(got.Regions))
	}
}

func TestPrintFileResult_Text(t *testing.T) {
	r := &FileResult{
		File: "/repo/main.go",
		Regions: []Region{
			{StartLine: 1, EndLine: 3, Blame: &BlameInfo{CommitHash: "abc1234567890abcdef1234567890abcdef12345"}},
		},
	}
	out := captureStdout(t, func() { printFileResult(r, false, false) })
	if out == "" {
		t.Error("expected non-empty text output")
	}
}

func TestPrintLineResultText_VerboseShowsFullPrompt(t *testing.T) {
	longPrompt := strings.Repeat("a", 200)
	r := &LineResult{
		File:  "/repo/main.go",
		Line:  1,
		Blame: &BlameInfo{CommitHash: "abc1234567890abcdef1234567890abcdef12345"},
		Match: &MatchResult{
			Session:    SessionMeta{ID: "s1", Timestamp: time.Now()},
			Prompt:     longPrompt,
			TurnIndex:  1,
			TotalTurns: 1,
		},
	}

	outShort := captureStdout(t, func() { printLineResultText(r, false) })
	if strings.Contains(outShort, longPrompt) {
		t.Error("non-verbose should truncate long prompt")
	}

	outFull := captureStdout(t, func() { printLineResultText(r, true) })
	if !strings.Contains(outFull, longPrompt) {
		t.Error("verbose should show full prompt without truncation")
	}
}

func TestPrintFileResultText_VerboseShowsFullPrompt(t *testing.T) {
	longPrompt := strings.Repeat("b", 200)
	r := &FileResult{
		File: "/repo/main.go",
		Regions: []Region{
			{
				StartLine: 1,
				EndLine:   10,
				Blame: &BlameInfo{
					CommitHash: "abc1234567890abcdef1234567890abcdef12345",
					AuthorTime: time.Now().Add(-time.Hour),
				},
				Match: &MatchResult{
					Session: SessionMeta{ID: "s1", Timestamp: time.Now()},
					Prompt:  longPrompt,
				},
			},
		},
	}

	outShort := captureStdout(t, func() { printFileResultText(r, false) })
	if strings.Contains(outShort, longPrompt) {
		t.Error("non-verbose should truncate long prompt in file mode")
	}

	outFull := captureStdout(t, func() { printFileResultText(r, true) })
	if !strings.Contains(outFull, longPrompt) {
		t.Error("verbose should show full prompt in file mode")
	}
}
