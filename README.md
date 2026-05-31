# augur

**augur** traces code back to the Claude session that wrote it — like `git blame`, but for AI.

Given a file (or a specific line), augur walks your Claude Code session history to find the prompt that produced the code, which session it came from, and how many rounds of back-and-forth it took.

## Install

```bash
go install github.com/zpenka/augur/cmd/augur@latest
```

## Usage

```
augur [--json] [--verbose] [--dir <projects-dir>] <file>[:<line>]
```

```bash
# Trace a specific line
augur src/auth.go:42

# Trace a whole file (shows all AI-attributed regions)
augur src/auth.go

# JSON output (for piping / tooling)
augur --json src/auth.go:42

# Show the full prompt without truncation
augur --verbose src/auth.go:42

# Use a custom Claude projects directory
augur --dir /path/to/projects src/auth.go

# Print version
augur --version
```

## Output

### Single-line lookup

```
src/auth.go:42

  commit   abc1234f  3 days ago
  author   Jane Doe
  message  Add error handling to auth flow

  session  c95ad6fe  2026-05-28
  branch   main
  prompt   "add error handling to the auth flow"
  turn     2 of 5
```

If no session matches (code predates AI usage or the session was deleted):

```
src/auth.go:42

  commit   abc1234f  3 months ago
  author   Jane Doe
  message  Initial commit

  no session match
```

### Whole-file lookup

```
src/auth.go — 3 region(s), 2 AI-attributed

  lines 1-18      abc1234f  3 days ago    "add error handling to the auth flow"
  lines 19-40     def5678a  2 weeks ago   "refactor session middleware"
  lines 41-55     abc1234f  3 days ago    no match (Jane Doe)
```

## How it works

1. Runs `git blame` on the target file/line to get the commit hash and timestamp.
2. Scans `~/.claude/projects/` for sessions whose working directory is within the same git repo.
3. Filters to sessions that started within 7 days before the commit (with a 1-hour clock-skew buffer).
4. Parses each candidate session's JSONL transcript, looking for `Edit`, `Write`, and `Bash` tool calls on the target file.
5. Returns the user prompt that preceded the matching edit, plus session metadata.

## Configuration

| Flag | Env var | Default |
|------|---------|---------|
| `--dir` | `AUGUR_PROJECTS_DIR` | `~/.claude/projects` |
| `--json` | — | text output |
| `--verbose` | — | truncated prompts |

## Companion tool

augur is a companion to [lore](https://github.com/zpenka/lore), a TUI for browsing Claude Code session history. The session ID shown in augur's output can be used to find the full session in lore.
