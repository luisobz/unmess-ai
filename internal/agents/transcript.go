package agents

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Prompt is a genuine user message with a valid timestamp. It is context,
// never proof that a particular filesystem change was made by the agent.
type Prompt struct {
	Time time.Time
	Text string
}

// TranscriptData separates observed metadata from inferred file correlations.
type TranscriptData struct {
	Start, End   time.Time
	CWD, Title   string
	Prompts      []Prompt
	InvalidLines int
	Writes       []WriteEvidence
}

// ParseTranscript tolerates damaged and oversized JSONL records and continues
// parsing later messages. Callers bound input size before calling this function.
func ParseTranscript(data []byte) TranscriptData {
	var out TranscriptData
	seen := make(map[string]bool)
	pending := make(map[string][]WriteEvidence)
	currentCWD := ""
	for len(data) > 0 {
		line, rest, _ := bytes.Cut(data, []byte("\n"))
		data = rest
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if len(line) > 2*1024*1024 {
			out.InvalidLines++
			continue
		}
		var obj map[string]any
		if json.Unmarshal(line, &obj) != nil {
			out.InvalidLines++
			continue
		}
		typ, _ := obj["type"].(string)
		stamp, _ := obj["timestamp"].(string)
		if stamp == "" {
			stamp, _ = obj["ts"].(string)
		}
		ts, _ := time.Parse(time.RFC3339Nano, stamp)
		if !ts.IsZero() {
			if out.Start.IsZero() || ts.Before(out.Start) {
				out.Start = ts
			}
			if out.End.IsZero() || ts.After(out.End) {
				out.End = ts
			}
		}
		cwd, _ := obj["cwd"].(string)
		payload, _ := obj["payload"].(map[string]any)
		if (typ == "session_meta" || typ == "turn_context") && payload != nil {
			cwd, _ = payload["cwd"].(string)
		}
		if cwd != "" {
			currentCWD = cwd
		}
		if cwd == "" {
			cwd = currentCWD
		}
		if out.CWD == "" && cwd != "" {
			out.CWD = cwd
		}
		collectWrites(obj, typ, payload, ts, cwd, pending, &out)
		var text string
		if typ == "user" && obj["isMeta"] != true {
			text = UserText(obj["message"])
		}
		if typ == "response_item" && payload != nil && payload["role"] == "user" {
			text = UserText(payload)
		}
		if typ == "event_msg" && payload != nil && payload["type"] == "user_message" {
			raw, _ := payload["message"].(string)
			text = usefulText(raw)
		}
		if text == "" {
			continue
		}
		if out.Title == "" {
			out.Title = clipTitle(text)
		}
		// Unknown timestamps must never be treated as the beginning of time.
		if ts.IsZero() {
			continue
		}
		key := ts.Format(time.RFC3339Nano) + "\x00" + text
		if seen[key] {
			continue
		}
		seen[key] = true
		out.Prompts = append(out.Prompts, Prompt{Time: ts, Text: text})
	}
	sort.SliceStable(out.Prompts, func(i, j int) bool { return out.Prompts[i].Time.Before(out.Prompts[j].Time) })
	return out
}

func isInjectedText(s string) bool {
	for _, prefix := range []string{"# AGENTS.md instructions", "<environment_context>", "<permissions instructions>", "<turn_aborted>", "[Request interrupted", "Caveat:"} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// WriteEvidence links an explicit write tool invocation to its successful result.
// Shell commands and narrative assistant messages are deliberately not inferred.
type WriteEvidence struct {
	Path string    `json:"path"`
	Tool string    `json:"tool"`
	Time time.Time `json:"time"`
	CWD  string    `json:"-"`
}

func collectWrites(obj map[string]any, typ string, payload map[string]any, ts time.Time, cwd string, pending map[string][]WriteEvidence, out *TranscriptData) {
	if cwd == "" {
		cwd = out.CWD
	}
	// Claude/CommandCode tool calls and results.
	message, _ := obj["message"].(map[string]any)
	if message != nil {
		blocks, _ := message["content"].([]any)
		for _, raw := range blocks {
			b, _ := raw.(map[string]any)
			if b == nil {
				continue
			}
			id, _ := b["id"].(string)
			if typ == "assistant" && b["type"] == "tool_use" && id != "" {
				tool, _ := b["name"].(string)
				input, _ := b["input"].(map[string]any)
				if tool == "Write" || tool == "Edit" || tool == "MultiEdit" {
					path, _ := input["file_path"].(string)
					if path != "" {
						pending[id] = []WriteEvidence{{Path: path, Tool: tool, Time: ts, CWD: cwd}}
					}
				}
			}
			if typ == "user" && b["type"] == "tool_result" {
				callID, _ := b["tool_use_id"].(string)
				if writes, ok := pending[callID]; ok {
					if b["is_error"] != true && b["content"] != nil {
						out.Writes = append(out.Writes, writes...)
					}
					delete(pending, callID)
				}
			}
		}
	}
	if typ != "response_item" || payload == nil {
		return
	}
	callID, _ := payload["call_id"].(string)
	if callID == "" {
		return
	}
	pt, _ := payload["type"].(string)
	if pt == "custom_tool_call" || pt == "function_call" {
		tool, _ := payload["name"].(string)
		if tool != "apply_patch" && tool != "functions.apply_patch" {
			return
		}
		input, _ := payload["input"].(string)
		if input == "" {
			args, _ := payload["arguments"].(string)
			var a map[string]any
			if json.Unmarshal([]byte(args), &a) == nil {
				input, _ = a["patch"].(string)
				if input == "" {
					input, _ = a["input"].(string)
				}
			}
		}
		for _, path := range patchPaths(input) {
			pending[callID] = append(pending[callID], WriteEvidence{Path: path, Tool: tool, Time: ts, CWD: cwd})
		}
	}
	if pt == "custom_tool_call_output" || pt == "function_call_output" {
		output, _ := payload["output"].(string)
		// apply_patch's canonical success response. A failed or missing output
		// never confirms ownership, even when the attempted patch mentions a path.
		if strings.Contains(output, "Success. Updated the following files:") {
			out.Writes = append(out.Writes, pending[callID]...)
		}
		delete(pending, callID)
	}
}

func patchPaths(input string) []string {
	var paths []string
	for _, line := range strings.Split(input, "\n") {
		for _, prefix := range []string{"*** Update File: ", "*** Add File: ", "*** Delete File: ", "*** Move to: "} {
			if path, ok := strings.CutPrefix(line, prefix); ok && strings.TrimSpace(path) != "" {
				paths = append(paths, strings.TrimSpace(path))
			}
		}
	}
	return paths
}
