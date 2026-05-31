package augur

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// BlameInfo holds git blame metadata for a single line.
type BlameInfo struct {
	CommitHash  string    `json:"hash"`
	Author      string    `json:"author"`
	AuthorTime  time.Time `json:"author_time"`
	Summary     string    `json:"summary"`
	LineContent string    `json:"-"`
}

// gitRoot returns the git repository root for the directory containing path.
func gitRoot(path string) (string, error) {
	cmd := exec.Command("git", "-C", filepath.Dir(path), "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// blameLine returns blame info for a single line in file.
func blameLine(file string, line int) (*BlameInfo, error) {
	cmd := exec.Command("git", "blame", "-p", "-L", fmt.Sprintf("%d,%d", line, line), "--", file)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame: %w", err)
	}
	m, err := parseBlameOutput(string(out))
	if err != nil {
		return nil, err
	}
	b, ok := m[line]
	if !ok {
		return nil, fmt.Errorf("no blame info for line %d", line)
	}
	return b, nil
}

// blameFile returns blame info for every line in file.
func blameFile(file string) (map[int]*BlameInfo, error) {
	cmd := exec.Command("git", "blame", "-p", "--", file)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame: %w", err)
	}
	return parseBlameOutput(string(out))
}

// parseBlameOutput parses git blame --porcelain output into a map of line→BlameInfo.
func parseBlameOutput(output string) (map[int]*BlameInfo, error) {
	type commitMeta struct {
		Author     string
		AuthorTime time.Time
		Summary    string
	}

	results := make(map[int]*BlameInfo)
	cache := make(map[string]*commitMeta)

	scanner := bufio.NewScanner(strings.NewReader(output))
	var curHash string
	var curLine int

	for scanner.Scan() {
		text := scanner.Text()

		switch {
		case isBlameHashLine(text):
			parts := strings.Fields(text)
			if len(parts) < 3 {
				continue
			}
			curHash = strings.TrimPrefix(parts[0], "^")
			n, _ := strconv.Atoi(parts[2])
			curLine = n
			if _, ok := cache[curHash]; !ok {
				cache[curHash] = &commitMeta{}
			}
			results[curLine] = &BlameInfo{CommitHash: curHash}

		case strings.HasPrefix(text, "author "):
			if c := cache[curHash]; c != nil {
				c.Author = strings.TrimPrefix(text, "author ")
			}

		case strings.HasPrefix(text, "author-time "):
			if c := cache[curHash]; c != nil {
				ts, _ := strconv.ParseInt(strings.TrimPrefix(text, "author-time "), 10, 64)
				c.AuthorTime = time.Unix(ts, 0).UTC()
			}

		case strings.HasPrefix(text, "summary "):
			if c := cache[curHash]; c != nil {
				c.Summary = strings.TrimPrefix(text, "summary ")
			}

		case strings.HasPrefix(text, "\t"):
			// Tab-prefixed line comes last in each block — copy cached metadata into result.
			if b, ok := results[curLine]; ok {
				b.LineContent = strings.TrimPrefix(text, "\t")
				if c := cache[curHash]; c != nil {
					b.Author = c.Author
					b.AuthorTime = c.AuthorTime
					b.Summary = c.Summary
				}
			}
		}
	}

	return results, scanner.Err()
}

// isBlameHashLine returns true if the line looks like the first line of a blame block:
// a 40-hex-char commit hash (optionally prefixed with ^) followed by a space.
func isBlameHashLine(line string) bool {
	s := strings.TrimPrefix(line, "^")
	if len(s) < 41 {
		return false
	}
	for _, c := range s[:40] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return s[40] == ' '
}

// isZeroHash returns true for the all-zeros hash git uses for uncommitted lines.
func isZeroHash(hash string) bool {
	for _, c := range hash {
		if c != '0' {
			return false
		}
	}
	return len(hash) > 0
}
