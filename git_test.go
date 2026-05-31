package augur

import (
	"testing"
	"time"
)

func TestIsBlameHashLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"abc1234567890abcdef1234567890abcdef12345 1 1 1", true},
		{"^abc1234567890abcdef1234567890abcdef12345 1 1 1", true},
		{"author Jane Doe", false},
		{"summary Add feature", false},
		{"\tline content", false},
		{"short 1 1", false},
		{"abc1234567890abcdef1234567890abcdef1234X 1 1 1", false}, // non-hex char
	}
	for _, c := range cases {
		got := isBlameHashLine(c.line)
		if got != c.want {
			t.Errorf("isBlameHashLine(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestIsZeroHash(t *testing.T) {
	if !isZeroHash("0000000000000000000000000000000000000000") {
		t.Error("expected true for zero hash")
	}
	if isZeroHash("abc1234567890abcdef1234567890abcdef123456") {
		t.Error("expected false for real hash")
	}
	if isZeroHash("") {
		t.Error("expected false for empty string")
	}
}

func TestParseBlameOutput(t *testing.T) {
	const sample = `abc1234567890abcdef1234567890abcdef12345 1 1 2
author Alice
author-mail <alice@example.com>
author-time 1748000000
author-tz +0000
committer Alice
committer-mail <alice@example.com>
committer-time 1748000000
committer-tz +0000
summary Add auth handler
filename src/auth.go
	func handleAuth() {
abc1234567890abcdef1234567890abcdef12345 2 2
filename src/auth.go
	}
`

	m, err := parseBlameOutput(sample)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}

	b1, ok := m[1]
	if !ok {
		t.Fatal("missing entry for line 1")
	}
	if b1.CommitHash != "abc1234567890abcdef1234567890abcdef12345" {
		t.Errorf("wrong hash: %s", b1.CommitHash)
	}
	if b1.Author != "Alice" {
		t.Errorf("wrong author: %s", b1.Author)
	}
	if b1.AuthorTime != time.Unix(1748000000, 0).UTC() {
		t.Errorf("wrong author time: %v", b1.AuthorTime)
	}
	if b1.Summary != "Add auth handler" {
		t.Errorf("wrong summary: %s", b1.Summary)
	}
	if b1.LineContent != "func handleAuth() {" {
		t.Errorf("wrong line content: %s", b1.LineContent)
	}

	// Line 2 reuses the cached commit metadata.
	b2, ok := m[2]
	if !ok {
		t.Fatal("missing entry for line 2")
	}
	if b2.Author != "Alice" {
		t.Errorf("line 2 should inherit author from cache, got %q", b2.Author)
	}
	if b2.LineContent != "}" {
		t.Errorf("wrong line 2 content: %q", b2.LineContent)
	}
}

func TestParseBlameOutput_Empty(t *testing.T) {
	m, err := parseBlameOutput("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(m))
	}
}
