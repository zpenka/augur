package augur

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const sessionWindow = 7 * 24 * time.Hour

// MatchResult holds the session and prompt that produced a piece of code.
type MatchResult struct {
	Session    SessionMeta `json:"session"`
	Prompt     string      `json:"prompt"`
	TurnIndex  int         `json:"turn_index"`  // which user turn (1-indexed)
	TotalTurns int         `json:"total_turns"` // total user turns in session
}

// promptEdits pairs a user prompt with the edits that followed it.
type promptEdits struct {
	Prompt string
	Edits  []EditRef
}

// findMatch finds the best session that contains an edit to absFile near commitTime.
// Returns nil, nil when no session matches.
func findMatch(sessions []SessionMeta, gitRoot, absFile string, commitTime time.Time) (*MatchResult, error) {
	var candidates []SessionMeta
	for _, s := range sessions {
		if !isRepoSession(s.CWD, gitRoot) {
			continue
		}
		// Session must start before commit time (+ 1h buffer for clock skew).
		if s.Timestamp.After(commitTime.Add(time.Hour)) {
			continue
		}
		// Don't look further back than the window.
		if s.Timestamp.Before(commitTime.Add(-sessionWindow)) {
			continue
		}
		candidates = append(candidates, s)
	}

	// Most recent session first — prefer the session closest to the commit.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Timestamp.After(candidates[j].Timestamp)
	})

	for _, s := range candidates {
		turns, err := parseTurns(s.Path)
		if err != nil {
			continue
		}

		pairs := buildPromptEdits(turns)
		for i, pe := range pairs {
			for _, edit := range pe.Edits {
				editPath := edit.Path
				if !filepath.IsAbs(editPath) && s.CWD != "" {
					editPath = filepath.Join(s.CWD, editPath)
				}
				if sameFile(editPath, absFile) {
					return &MatchResult{
						Session:    s,
						Prompt:     pe.Prompt,
						TurnIndex:  i + 1,
						TotalTurns: len(pairs),
					}, nil
				}
			}
		}
	}

	return nil, nil
}

// buildPromptEdits groups turns into (prompt, edits) pairs: each user prompt paired
// with all Edit/Write calls that occurred before the next user prompt.
func buildPromptEdits(turns []ParsedTurn) []promptEdits {
	var result []promptEdits
	var cur *promptEdits

	for _, t := range turns {
		if t.IsUser && t.Prompt != "" {
			if cur != nil && len(cur.Edits) > 0 {
				result = append(result, *cur)
			}
			cur = &promptEdits{Prompt: t.Prompt}
		} else if !t.IsUser && cur != nil {
			cur.Edits = append(cur.Edits, t.Edits...)
		}
	}

	if cur != nil && len(cur.Edits) > 0 {
		result = append(result, *cur)
	}

	return result
}

// isRepoSession returns true if sessionCWD is the git root or a subdirectory of it.
func isRepoSession(sessionCWD, gitRoot string) bool {
	if sessionCWD == "" || gitRoot == "" {
		return false
	}
	sessionCWD = resolveSymlink(filepath.Clean(sessionCWD))
	gitRoot = resolveSymlink(filepath.Clean(gitRoot))
	return sessionCWD == gitRoot ||
		strings.HasPrefix(sessionCWD, gitRoot+string(os.PathSeparator))
}

// sameFile returns true if two paths refer to the same file after cleaning and symlink resolution.
func sameFile(a, b string) bool {
	return resolveSymlink(filepath.Clean(a)) == resolveSymlink(filepath.Clean(b))
}

// resolveSymlink resolves symlinks in path; returns the cleaned path if resolution fails.
func resolveSymlink(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
