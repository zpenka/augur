package augur

import (
	"encoding/json"
	"testing"
)

func TestExtractPromptFromMessage_Empty(t *testing.T) {
	if extractPromptFromMessage(nil) != "" {
		t.Error("nil message should return empty")
	}
	if extractPromptFromMessage(json.RawMessage("{}")) != "" {
		t.Error("empty content should return empty")
	}
}

func TestExtractPromptFromMessage_StringContent(t *testing.T) {
	raw := json.RawMessage(`{"content":"hello world"}`)
	got := extractPromptFromMessage(raw)
	if got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestExtractPromptFromMessage_OnlyToolResults(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}`)
	got := extractPromptFromMessage(raw)
	if got != "" {
		t.Errorf("tool-result-only message should return empty, got %q", got)
	}
}

func TestExtractPromptFromMessage_MixedBlocks(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"},{"type":"text","text":"follow-up question"}]}`)
	got := extractPromptFromMessage(raw)
	if got != "follow-up question" {
		t.Errorf("got %q", got)
	}
}

func TestExtractEditsFromMessage_Write(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"tool_use","name":"Write","id":"w1","input":{"path":"/proj/new.go","content":"package main"}}]}`)
	edits := extractEditsFromMessage(raw)
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].Tool != "Write" {
		t.Errorf("wrong tool: %s", edits[0].Tool)
	}
	if edits[0].Path != "/proj/new.go" {
		t.Errorf("wrong path: %s", edits[0].Path)
	}
}

func TestExtractEditsFromMessage_NoPath(t *testing.T) {
	// Tool use with empty path should be skipped.
	raw := json.RawMessage(`{"content":[{"type":"tool_use","name":"Edit","id":"e1","input":{"path":""}}]}`)
	edits := extractEditsFromMessage(raw)
	if len(edits) != 0 {
		t.Errorf("expected 0 edits for empty path, got %d", len(edits))
	}
}

func TestExtractEditsFromMessage_NonEditTool(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"tool_use","name":"Bash","id":"b1","input":{"command":"ls"}}]}`)
	edits := extractEditsFromMessage(raw)
	if len(edits) != 0 {
		t.Errorf("Bash tool should not produce edits, got %d", len(edits))
	}
}

func TestExtractEditsFromMessage_Empty(t *testing.T) {
	if edits := extractEditsFromMessage(nil); len(edits) != 0 {
		t.Errorf("nil should produce no edits")
	}
}

func TestExtractEditsFromMessage_BadContentJSON(t *testing.T) {
	// content field is not an array — should return nil gracefully
	raw := json.RawMessage(`{"content":"not-an-array"}`)
	if edits := extractEditsFromMessage(raw); len(edits) != 0 {
		t.Errorf("bad content JSON should produce no edits, got %d", len(edits))
	}
}

func TestExtractPromptFromMessage_NullContent(t *testing.T) {
	raw := json.RawMessage(`{"content":null}`)
	got := extractPromptFromMessage(raw)
	if got != "" {
		t.Errorf("null content should return empty, got %q", got)
	}
}

func TestExtractPromptFromMessage_BadOuterJSON(t *testing.T) {
	// Completely invalid JSON — should return ""
	got := extractPromptFromMessage(json.RawMessage(`{bad json`))
	if got != "" {
		t.Errorf("bad JSON should return empty, got %q", got)
	}
}
