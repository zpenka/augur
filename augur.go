package augur

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const Version = "0.1.0"

// LineResult is the result of a single-line lookup.
type LineResult struct {
	File  string       `json:"file"`
	Line  int          `json:"line"`
	Blame *BlameInfo   `json:"commit"`
	Match *MatchResult `json:"session"` // nil if no session found
}

// Region is a consecutive range of lines sharing the same git commit.
type Region struct {
	StartLine int          `json:"start_line"`
	EndLine   int          `json:"end_line"`
	Blame     *BlameInfo   `json:"commit"`
	Match     *MatchResult `json:"session"` // nil if no session found
}

// FileResult is the result of a whole-file lookup.
type FileResult struct {
	File    string   `json:"file"`
	Regions []Region `json:"regions"`
}

func Run() int {
	var jsonFlag bool
	var verboseFlag bool
	var claudeDir string
	var versionFlag bool

	flag.BoolVar(&jsonFlag, "json", false, "output as JSON")
	flag.BoolVar(&verboseFlag, "verbose", false, "show full prompt text without truncation")
	flag.StringVar(&claudeDir, "dir", "", "claude projects directory")
	flag.BoolVar(&versionFlag, "version", false, "print version and exit")
	flag.BoolVar(&versionFlag, "v", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: augur [--json] [--verbose] [--dir <projects-dir>] <file>[:<line>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Traces a file (or line) back to the Claude session that wrote it.")
		fmt.Fprintln(os.Stderr, "")
		flag.PrintDefaults()
	}
	flag.Parse()

	if versionFlag {
		fmt.Printf("augur %s\n", Version)
		return 0
	}

	if flag.NArg() == 0 {
		flag.Usage()
		return 1
	}

	if claudeDir == "" {
		claudeDir = os.Getenv("AUGUR_PROJECTS_DIR")
	}
	if claudeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot find home dir: %v\n", err)
			return 1
		}
		claudeDir = filepath.Join(home, ".claude", "projects")
	}

	target := flag.Arg(0)
	file, line, err := parseTarget(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	absFile, err := filepath.Abs(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if line > 0 {
		result, err := lookupLine(absFile, line, claudeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		printLineResult(result, jsonFlag, verboseFlag)
	} else {
		result, err := lookupFile(absFile, claudeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		printFileResult(result, jsonFlag, verboseFlag)
	}

	return 0
}

// parseTarget splits "file" or "file:line" into (file, line, err).
// Returns line=0 for whole-file queries.
func parseTarget(s string) (string, int, error) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, 0, nil
	}
	after := s[i+1:]
	n, err := strconv.Atoi(after)
	if err != nil {
		// Not a number — treat the whole string as a filename.
		return s, 0, nil
	}
	if n < 1 {
		return "", 0, fmt.Errorf("line number must be >= 1, got %d", n)
	}
	return s[:i], n, nil
}

func lookupLine(absFile string, line int, claudeDir string) (*LineResult, error) {
	root, _ := gitRoot(absFile)

	blame, err := blameLine(absFile, line)
	if err != nil {
		return nil, err
	}

	result := &LineResult{File: absFile, Line: line, Blame: blame}

	if root != "" && !isZeroHash(blame.CommitHash) {
		sessions, err := scanSessionMeta(claudeDir)
		if err == nil {
			result.Match, _ = findMatch(sessions, root, absFile, blame.AuthorTime)
		}
	}

	return result, nil
}

func lookupFile(absFile string, claudeDir string) (*FileResult, error) {
	root, _ := gitRoot(absFile)

	blameMap, err := blameFile(absFile)
	if err != nil {
		return nil, err
	}

	regions := buildRegions(blameMap)

	var sessions []SessionMeta
	if root != "" {
		sessions, _ = scanSessionMeta(claudeDir)
	}

	// Cache matches by commit hash to avoid rescanning sessions for repeated commits.
	type cacheEntry struct {
		m   *MatchResult
		err error
	}
	cache := make(map[string]cacheEntry)

	for i := range regions {
		hash := regions[i].Blame.CommitHash
		if isZeroHash(hash) || root == "" {
			continue
		}
		if ce, ok := cache[hash]; ok {
			regions[i].Match = ce.m
			continue
		}
		m, merr := findMatch(sessions, root, absFile, regions[i].Blame.AuthorTime)
		cache[hash] = cacheEntry{m, merr}
		regions[i].Match = m
	}

	return &FileResult{File: absFile, Regions: regions}, nil
}

// buildRegions groups consecutive lines sharing the same commit hash into regions.
func buildRegions(blameMap map[int]*BlameInfo) []Region {
	lines := make([]int, 0, len(blameMap))
	for ln := range blameMap {
		lines = append(lines, ln)
	}
	sort.Ints(lines)

	if len(lines) == 0 {
		return nil
	}

	var regions []Region
	cur := Region{
		StartLine: lines[0],
		EndLine:   lines[0],
		Blame:     blameMap[lines[0]],
	}

	for _, ln := range lines[1:] {
		b := blameMap[ln]
		if ln == cur.EndLine+1 && b.CommitHash == cur.Blame.CommitHash {
			cur.EndLine = ln
		} else {
			regions = append(regions, cur)
			cur = Region{StartLine: ln, EndLine: ln, Blame: b}
		}
	}
	regions = append(regions, cur)

	return regions
}

func printLineResult(r *LineResult, asJSON, verbose bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r) //nolint:errcheck
		return
	}
	printLineResultText(r, verbose)
}

func printFileResult(r *FileResult, asJSON, verbose bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r) //nolint:errcheck
		return
	}
	printFileResultText(r, verbose)
}
