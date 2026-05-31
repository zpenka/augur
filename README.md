# augur

**augur** traces code back to the Claude session that wrote it — like `git blame`, but for AI.

Given a file (or a specific line), augur walks your Claude Code session history to find the prompt that produced the code, which session it came from, and how many rounds of back-and-forth it took.

## Usage

```bash
# Trace a specific line
augur src/auth.go:42

# Trace a whole file (shows all AI-attributed regions)
augur src/auth.go

# JSON output (for piping)
augur --json src/auth.go:42
```

### Example output

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

If no session matches (code predates AI usage or session was deleted):

```
src/auth.go:42

  commit   abc1234f  3 months ago
  author   Jane Doe
  message  Initial commit

  no session match
```

## How it works

1. Runs `git blame` on the target file/line to get the commit hash and timestamp.
2. Scans `~/.claude/projects/` for sessions whose working directory is within the same git repo.
3. Filters to sessions that started within 7 days before the commit.
4. Parses each candidate session's JSONL transcript, looking for `Edit` or `Write` tool calls on the target file.
5. Returns the user prompt that preceded the matching edit, plus session metadata.

## Install

```bash
go install github.com/zpenka/augur/cmd/augur@latest
```

## Configuration

| Flag | Env var | Default |
|------|---------|---------|
| `--dir` | `AUGUR_PROJECTS_DIR` | `~/.claude/projects` |

## Companion tool

augur is a companion to [lore](https://github.com/zpenka/lore), a TUI for browsing Claude Code session history. The session ID shown in augur's output can be used to find the full session in lore.
