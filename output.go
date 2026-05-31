package augur

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func printLineResultText(r *LineResult) {
	rel := relPath(r.File)
	fmt.Printf("%s:%d\n\n", rel, r.Line)

	if r.Blame != nil {
		fmt.Printf("  commit   %s  %s\n", short8(r.Blame.CommitHash), humanTime(r.Blame.AuthorTime))
		if r.Blame.Author != "" {
			fmt.Printf("  author   %s\n", r.Blame.Author)
		}
		if r.Blame.Summary != "" {
			fmt.Printf("  message  %s\n", r.Blame.Summary)
		}
	}

	fmt.Println()

	if r.Match != nil {
		fmt.Printf("  session  %s  %s\n", short8(r.Match.Session.ID), r.Match.Session.Timestamp.Format("2006-01-02"))
		if r.Match.Session.Branch != "" {
			fmt.Printf("  branch   %s\n", r.Match.Session.Branch)
		}
		fmt.Printf("  prompt   %q\n", truncate(r.Match.Prompt, 100))
		fmt.Printf("  turn     %d of %d\n", r.Match.TurnIndex, r.Match.TotalTurns)
	} else {
		fmt.Printf("  no session match\n")
	}

	fmt.Println()
}

func printFileResultText(r *FileResult) {
	rel := relPath(r.File)
	matched := 0
	for _, reg := range r.Regions {
		if reg.Match != nil {
			matched++
		}
	}
	fmt.Printf("%s — %d region(s), %d AI-attributed\n\n", rel, len(r.Regions), matched)

	for _, reg := range r.Regions {
		lineRange := fmt.Sprintf("%d-%d", reg.StartLine, reg.EndLine)
		if reg.StartLine == reg.EndLine {
			lineRange = fmt.Sprintf("%d", reg.StartLine)
		}
		commitShort := "       "
		age := ""
		if reg.Blame != nil {
			commitShort = short8(reg.Blame.CommitHash)
			age = humanTime(reg.Blame.AuthorTime)
		}

		if reg.Match != nil {
			prompt := truncate(reg.Match.Prompt, 60)
			fmt.Printf("  lines %-8s  %s  %-12s  %q\n", lineRange, commitShort, age, prompt)
		} else {
			author := ""
			if reg.Blame != nil && reg.Blame.Author != "" {
				author = reg.Blame.Author + ", "
			}
			fmt.Printf("  lines %-8s  %s  %-12s  no match (%s%s)\n", lineRange, commitShort, age, author, age)
		}
	}

	fmt.Println()
}

func relPath(absPath string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, absPath); err == nil {
			return rel
		}
	}
	return absPath
}

func short8(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func humanTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < 2*time.Minute:
		return "just now"
	case d < 2*time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d < 8*7*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(7*24)))
	default:
		return fmt.Sprintf("%d months ago", int(d.Hours()/(30*24)))
	}
}

func truncate(s string, max int) string {
	r := []rune(s)
	// Collapse newlines to spaces for single-line display.
	for i, c := range r {
		if c == '\n' || c == '\r' || c == '\t' {
			r[i] = ' '
		}
	}
	s = strings.TrimSpace(string(r))
	r = []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
