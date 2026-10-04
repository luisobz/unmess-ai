package agents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func jsonLine(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b) + "\n"
}

func TestTranscriptSkipsInjectedAndMissingTimestampPrompts(t *testing.T) {
	raw := `{"timestamp":"2026-10-04T12:00:00Z","type":"session_meta","payload":{"cwd":"/home/test/work"}}` + "\n" +
		`{"timestamp":"2026-10-04T12:00:01Z","type":"response_item","payload":{"role":"user","content":[{"text":"# AGENTS.md instructions for /home/test"}]}}` + "\n" +
		`{"type":"user","message":{"content":"message without timestamp"}}` + "\n" +
		`{"timestamp":"2026-10-04T12:02:00Z","type":"event_msg","payload":{"type":"user_message","message":"fix the login"}}` + "\n"
	got := ParseTranscript([]byte(raw))
	if len(got.Prompts) != 1 || got.Prompts[0].Text != "fix the login" {
		t.Fatalf("prompts: %+v", got.Prompts)
	}
	if got.CWD != "/home/test/work" || got.Start.IsZero() || got.End.Sub(got.Start) != 2*time.Minute {
		t.Fatalf("metadata: %+v", got)
	}
}

func TestTranscriptContinuesAfterOversizedCorruptRecords(t *testing.T) {
	raw := strings.Repeat("x", 2*1024*1024+1) + "\n{broken\n" +
		`{"timestamp":"2026-10-04T12:02:00Z","type":"user","message":{"content":"real request"}}` + "\n"
	got := ParseTranscript([]byte(raw))
	if got.InvalidLines != 2 || len(got.Prompts) != 1 {
		t.Fatalf("parsed: %+v", got)
	}
}

func TestClaudeWritesRequireSuccessfulMatchingResult(t *testing.T) {
	call := func(id, path string) string {
		return jsonLine(t, map[string]any{"type": "assistant", "timestamp": "2026-10-04T12:00:00Z", "cwd": "/home/test/work", "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": id, "name": "Edit", "input": map[string]any{"file_path": path}}}}})
	}
	result := func(id string, failed bool) string {
		return jsonLine(t, map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": failed, "content": "result"}}}})
	}
	got := ParseTranscript([]byte(call("good", "a.go") + call("failed", "b.go") + call("pending", "c.go") + result("good", false) + result("failed", true)))
	if len(got.Writes) != 1 || got.Writes[0].Path != "a.go" || got.Writes[0].CWD != "/home/test/work" {
		t.Fatalf("writes: %+v", got.Writes)
	}
}

func TestCodexPatchWritesAndMoveRequireSuccess(t *testing.T) {
	call := jsonLine(t, map[string]any{"type": "response_item", "timestamp": "2026-10-04T12:00:00Z", "payload": map[string]any{"type": "custom_tool_call", "name": "apply_patch", "call_id": "a", "input": "*** Begin Patch\n*** Update File: src/a.go\n*** Move to: src/b.go\n*** End Patch"}})
	output := func(value string) string {
		return jsonLine(t, map[string]any{"type": "response_item", "payload": map[string]any{"type": "custom_tool_call_output", "call_id": "a", "output": value}})
	}
	if got := ParseTranscript([]byte(call + output("patch failed"))); len(got.Writes) != 0 {
		t.Fatal("failed patch attributed")
	}
	got := ParseTranscript([]byte(call + output("Success. Updated the following files:\nM src/a.go")))
	if len(got.Writes) != 2 {
		t.Fatalf("writes: %+v", got.Writes)
	}
}
