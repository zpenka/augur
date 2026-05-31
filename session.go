package augur

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SessionMeta holds the identifying metadata read from a session's first user event.
type SessionMeta struct {
	ID        string
	Path      string
	CWD       string
	Branch    string
	Timestamp time.Time
}

// ParsedTurn is a simplified turn used for prompt-attribution matching.
type ParsedTurn struct {
	IsUser bool
	Prompt string    // non-empty for user turns that contain text (not just tool results)
	Edits  []EditRef // non-empty for assistant turns that contain Edit or Write tool calls
}

// EditRef records a file path touched by an Edit or Write tool call.
type EditRef struct {
	Path string
	Tool string // "Edit" or "Write"
}

// scanSessionMeta walks projectsDir and returns metadata for every session found.
// Files that cannot be read are silently skipped.
func scanSessionMeta(projectsDir string) ([]SessionMeta, error) {
	var sessions []SessionMeta
	err := filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"subagents"+string(filepath.Separator)) {
			return nil
		}
		meta, err := readSessionMeta(path)
		if err != nil || meta == nil {
			return nil
		}
		sessions = append(sessions, *meta)
		return nil
	})
	return sessions, err
}

type rawMetaEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"`
	CWD       string `json:"cwd"`
	GitBranch string `json:"gitBranch"`
}

// readSessionMeta reads only the first user event from a JSONL file.
func readSessionMeta(path string) (*SessionMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)

	for sc.Scan() {
		var ev rawMetaEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Type != "user" || ev.SessionID == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
		if err != nil {
			return nil, err
		}
		return &SessionMeta{
			ID:        ev.SessionID,
			Path:      path,
			CWD:       ev.CWD,
			Branch:    ev.GitBranch,
			Timestamp: ts,
		}, nil
	}
	return nil, sc.Err()
}

// parseTurns reads a session JSONL and returns a flat slice of ParsedTurns.
func parseTurns(path string) ([]ParsedTurn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)

	type rawEvent struct {
		Type    string          `json:"type"`
		Message json.RawMessage `json:"message"`
	}

	var turns []ParsedTurn

	for sc.Scan() {
		var ev rawEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "user":
			prompt := extractPromptFromMessage(ev.Message)
			if prompt != "" {
				turns = append(turns, ParsedTurn{IsUser: true, Prompt: prompt})
			}
		case "assistant":
			edits := extractEditsFromMessage(ev.Message)
			if len(edits) > 0 {
				turns = append(turns, ParsedTurn{IsUser: false, Edits: edits})
			}
		}
	}

	return turns, sc.Err()
}

// extractPromptFromMessage returns the user's text prompt from a raw message JSON blob.
// Returns "" for tool-result-only messages.
func extractPromptFromMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || len(msg.Content) == 0 {
		return ""
	}

	// Content may be a plain string.
	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil {
		return strings.TrimSpace(s)
	}

	// Content is an array of blocks.
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return ""
	}

	// Skip messages that are purely tool results.
	allToolResults := len(blocks) > 0
	var parts []string
	for _, b := range blocks {
		if b.Type != "tool_result" {
			allToolResults = false
		}
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, strings.TrimSpace(b.Text))
		}
	}
	if allToolResults {
		return ""
	}
	return strings.Join(parts, " ")
}

var (
	reBashRedirect   = regexp.MustCompile(`>{1,2}\s*(/\S+)`)
	reBashTee        = regexp.MustCompile(`\btee\s+(?:-a\s+)?(/\S+)`)
	reBashSedInPlace = regexp.MustCompile(`\bsed\b[^|&;]*?-i\S*[^|&;]*?\s(/\S+)`)
)

// extractBashPaths returns absolute file paths that a bash command writes to.
// Recognises output redirects (> / >>), tee, and sed -i.
func extractBashPaths(command string) []string {
	seen := make(map[string]bool)
	var paths []string
	add := func(p string) {
		p = strings.TrimRight(p, ";|&)")
		if p != "" && !seen[p] && strings.HasPrefix(p, "/") {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	for _, m := range reBashRedirect.FindAllStringSubmatch(command, -1) {
		add(m[1])
	}
	for _, m := range reBashTee.FindAllStringSubmatch(command, -1) {
		add(m[1])
	}
	for _, m := range reBashSedInPlace.FindAllStringSubmatch(command, -1) {
		add(m[1])
	}
	return paths
}

// extractEditsFromMessage returns Edit/Write tool calls from an assistant message.
func extractEditsFromMessage(raw json.RawMessage) []EditRef {
	if len(raw) == 0 {
		return nil
	}

	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || len(msg.Content) == 0 {
		return nil
	}

	var blocks []struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Input struct {
			Path    string `json:"path"`
			Command string `json:"command"`
		} `json:"input"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil
	}

	var edits []EditRef
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		switch b.Name {
		case "Edit", "Write":
			if b.Input.Path != "" {
				edits = append(edits, EditRef{Path: b.Input.Path, Tool: b.Name})
			}
		case "Bash":
			for _, p := range extractBashPaths(b.Input.Command) {
				edits = append(edits, EditRef{Path: p, Tool: "Bash"})
			}
		}
	}
	return edits
}
