package augur

import (
	"testing"
	"time"
)

func TestParseTarget_WholeFile(t *testing.T) {
	file, line, err := parseTarget("src/auth.go")
	if err != nil {
		t.Fatal(err)
	}
	if file != "src/auth.go" || line != 0 {
		t.Fatalf("got (%q, %d), want (src/auth.go, 0)", file, line)
	}
}

func TestParseTarget_WithLine(t *testing.T) {
	file, line, err := parseTarget("src/auth.go:42")
	if err != nil {
		t.Fatal(err)
	}
	if file != "src/auth.go" || line != 42 {
		t.Fatalf("got (%q, %d), want (src/auth.go, 42)", file, line)
	}
}

func TestParseTarget_LineZeroError(t *testing.T) {
	_, _, err := parseTarget("src/auth.go:0")
	if err == nil {
		t.Fatal("expected error for line 0")
	}
}

func TestParseTarget_NonNumericSuffix(t *testing.T) {
	// Colon followed by non-number → treat as filename.
	file, line, err := parseTarget("C:/some/path.go")
	if err != nil {
		t.Fatal(err)
	}
	if file != "C:/some/path.go" || line != 0 {
		t.Fatalf("got (%q, %d)", file, line)
	}
}

func TestParseTarget_EmptySuffix(t *testing.T) {
	file, line, err := parseTarget("src/auth.go:")
	if err != nil {
		t.Fatal(err)
	}
	if file != "src/auth.go:" || line != 0 {
		t.Fatalf("got (%q, %d)", file, line)
	}
}

func TestBuildRegions_Empty(t *testing.T) {
	regions := buildRegions(map[int]*BlameInfo{})
	if len(regions) != 0 {
		t.Fatalf("expected 0 regions, got %d", len(regions))
	}
}

func TestBuildRegions_SingleBlock(t *testing.T) {
	m := map[int]*BlameInfo{
		1: {CommitHash: "aaa"},
		2: {CommitHash: "aaa"},
		3: {CommitHash: "aaa"},
	}
	regions := buildRegions(m)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0].StartLine != 1 || regions[0].EndLine != 3 {
		t.Fatalf("got [%d-%d]", regions[0].StartLine, regions[0].EndLine)
	}
}

func TestBuildRegions_TwoCommits(t *testing.T) {
	m := map[int]*BlameInfo{
		1: {CommitHash: "aaa"},
		2: {CommitHash: "aaa"},
		3: {CommitHash: "bbb"},
		4: {CommitHash: "bbb"},
	}
	regions := buildRegions(m)
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}
	if regions[0].StartLine != 1 || regions[0].EndLine != 2 {
		t.Fatalf("region 0: got [%d-%d]", regions[0].StartLine, regions[0].EndLine)
	}
	if regions[1].StartLine != 3 || regions[1].EndLine != 4 {
		t.Fatalf("region 1: got [%d-%d]", regions[1].StartLine, regions[1].EndLine)
	}
}

func TestBuildRegions_NonConsecutiveSameHash(t *testing.T) {
	// Same commit hash on non-consecutive lines → two separate regions.
	m := map[int]*BlameInfo{
		1: {CommitHash: "aaa"},
		3: {CommitHash: "aaa"},
	}
	regions := buildRegions(m)
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}
}

func TestHumanTime_NonEmpty(t *testing.T) {
	s := humanTime(time.Now().Add(-25 * time.Hour))
	if s == "" {
		t.Fatal("empty human time")
	}
}

func TestHumanTime_Zero(t *testing.T) {
	s := humanTime(time.Time{})
	if s != "unknown" {
		t.Fatalf("want 'unknown', got %q", s)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello w…"},
		{"line1\nline2", 20, "line1 line2"},
		{"", 10, ""},
	}
	for _, c := range cases {
		got := truncate(c.in, c.max)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}
