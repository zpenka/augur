# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./cmd/augur

# Test (all, with race detector)
go test -race ./...

# Test (single test)
go test -run TestParseTarget_WholeFile ./...

# Vet + format check
go vet ./...
gofmt -l .   # should return nothing

# Run
go run ./cmd/augur src/auth.go          # whole file
go run ./cmd/augur src/auth.go:42       # specific line
go run ./cmd/augur --json src/auth.go   # JSON output
```

## Architecture

Augur is a CLI tool that traces code back to the Claude Code session that wrote it — like `git blame` but for AI-generated code. Given a file or line, it finds the commit, the Claude Code session, and the user prompt that produced the code.

**Package layout (all in the root package `augur`):**

- `cmd/augur/main.go` — thin entry point; calls `augur.Run()` and exits
- `augur.go` — `Run()`, flag parsing, `lookupLine()` / `lookupFile()`, `buildRegions()`
- `git.go` — `blameLine()` / `blameFile()` via `git blame -p`; `parseBlameOutput()` → `BlameInfo` map
- `session.go` — walks `~/.claude/projects/` JSONL files; `scanSessionMeta()`, `parseTurns()` → `ParsedTurn` / `EditRef`
- `match.go` — `findMatch()`: filters sessions by repo CWD + 7-day commit window (1-hour clock skew buffer); `buildPromptEdits()` pairs user prompts with subsequent Edit/Write calls
- `output.go` — text and JSON rendering; `printLineResultText()`, `printFileResultText()`

**Data flow:** `lookupLine/File` → `blameFile/Line` → `findMatch` (scans sessions) → `printXxxResult`

**Key matching heuristics in `match.go`:**
- Sessions filtered to same git repo (CWD comparison)
- Session must start within 7 days before commit timestamp
- File paths normalized with `filepath.Clean()` before comparison
- In `lookupFile()`, matches are cached by commit hash to avoid re-scanning for repeated commits

**No external dependencies** — stdlib only.
