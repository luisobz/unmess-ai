package api

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luisobz/unmess-ai/internal/agents"
)

func TestSessionUsesRecordedWorkspaceAndSuccessfulWrites(t *testing.T) {
	srv, st, base := newTestServer(t)
	t0 := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	cwd := filepath.Join(base, "projects", "target")
	cwdJSON, _ := json.Marshal(cwd)
	transcript := ".codex/sessions/2026/10/04/rollout-test.jsonl"
	raw := `{"timestamp":"2026-10-04T10:00:00Z","type":"session_meta","payload":{"cwd":` + string(cwdJSON) + `}}` + "\n" +
		`{"timestamp":"2026-10-04T10:05:00Z","type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch","call_id":"p","input":"*** Begin Patch\n*** Update File: a.go\n*** Add File: ignored.go\n*** End Patch"}}` + "\n" +
		`{"timestamp":"2026-10-04T10:06:00Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"p","output":"Success. Updated the following files:\nM a.go\nA ignored.go"}}` + "\n"
	writeVersion(t, st, base, transcript, raw, t0.Add(8*time.Hour))
	writeVersion(t, st, base, "projects/target/a.go", "before", t0.Add(-time.Hour))
	writeVersion(t, st, base, "projects/target/a.go", "agent", t0.Add(6*time.Minute))
	writeVersion(t, st, base, "projects/target/manual.go", "user", t0.Add(6*time.Minute))
	// Concurrent project dominates by file count, but cannot steal this session.
	for _, p := range []string{"a", "b", "c", "d"} {
		writeVersion(t, st, base, "other/"+p, "other", t0.Add(6*time.Minute))
	}
	files, err := st.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	spans := srv.buildSessions("codex", files)
	if len(spans) != 1 || spans[0].info.Workspace != "projects/target" || spans[0].info.TimeSource != "transcript" || !spans[0].start.Equal(t0) {
		t.Fatalf("session: %+v", spans)
	}
	_, sf := srv.sessionTrace(spans[0], files)
	byPath := map[string]sessionFile{}
	for _, f := range sf {
		byPath[f.Path] = f
	}
	if len(byPath) != 3 || byPath["projects/target/a.go"].Attribution != "tool" || byPath["projects/target/manual.go"].Attribution != "temporal" || byPath["projects/target/ignored.go"].BackedUp {
		t.Fatalf("files: %+v", sf)
	}
	body, _ := json.Marshal(map[string]any{"agent": "codex", "session": transcript, "start": spans[0].info.Start, "end": spans[0].info.End, "dry_run": true})
	rec := do(t, srv, "POST", "/api/agent/revert", reqOpts{token: srv.Token(), body: string(body)})
	var plan revertResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(plan.Items) != 1 || plan.Items[0].Path != "projects/target/a.go" {
		t.Fatalf("revert: %d %s", rec.Code, rec.Body.String())
	}
	// Detail remains tied to the saved snapshot even if live transcript changes.
	if err := os.WriteFile(st.OriginalPath(transcript), []byte("unbacked changes"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec = do(t, srv, "GET", "/api/agent/session?agent=codex&id="+url.QueryEscape(transcript), reqOpts{token: srv.Token()})
	if rec.Code != 200 {
		t.Fatalf("detail: %s", rec.Body.String())
	}
}

func TestRevertPreservesLaterAndUnbackedWork(t *testing.T) {
	srv, st, base := newTestServer(t)
	t0 := time.Now().Add(-time.Hour).Truncate(time.Second)
	for _, name := range []string{"later.txt", "pending.txt"} {
		writeVersion(t, st, base, name, "before", t0.Add(-time.Hour))
		writeVersion(t, st, base, name, "session", t0.Add(time.Minute))
	}
	writeVersion(t, st, base, "later.txt", "later work", t0.Add(20*time.Minute))
	if err := os.WriteFile(filepath.Join(base, "pending.txt"), []byte("pending work"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"start": t0.Format(time.RFC3339Nano), "end": t0.Add(10 * time.Minute).Format(time.RFC3339Nano), "dry_run": false})
	rec := do(t, srv, "POST", "/api/agent/revert", reqOpts{token: srv.Token(), body: string(body)})
	var plan revertResponse
	json.Unmarshal(rec.Body.Bytes(), &plan)
	if rec.Code != 200 || plan.SkippedConflict != 2 || plan.Reverted != 0 {
		t.Fatalf("revert: %d %s", rec.Code, rec.Body.String())
	}
	for name, want := range map[string]string{"later.txt": "later work", "pending.txt": "pending work"} {
		got, _ := os.ReadFile(filepath.Join(base, name))
		if string(got) != want {
			t.Fatal("later work overwritten")
		}
	}
}

func TestUnknownPromptTimestampCannotBeAssigned(t *testing.T) {
	ev := []sessionEvent{{TS: "2026-10-04T12:00:00Z"}}
	data := agents.ParseTranscript([]byte(`{"type":"user","message":{"content":"unknown time"}}`))
	assignPrompts(sessionSpan{prompts: data.Prompts}, ev, nil)
	if ev[0].Prompt != "" {
		t.Fatal("unknown time attributed")
	}
}

func TestConcurrentWritesSurfaceBothAgentSessions(t *testing.T) {
	srv, st, base := newTestServer(t)
	t0 := time.Now().Add(-time.Hour).In(time.Local).Truncate(time.Second)
	path := filepath.Join(base, "work", "shared.txt")
	abs, _ := json.Marshal(path)
	for _, session := range []string{"one", "two"} {
		raw := `{"type":"assistant","timestamp":"` + t0.Format(time.RFC3339Nano) + `","message":{"content":[{"type":"tool_use","id":"edit","name":"Edit","input":{"file_path":` + string(abs) + `}}]}}` + "\n" +
			`{"type":"user","timestamp":"` + t0.Add(time.Minute).Format(time.RFC3339Nano) + `","message":{"content":[{"type":"tool_result","tool_use_id":"edit","content":"updated"}]}}` + "\n"
		writeVersion(t, st, base, ".claude/projects/work/"+session+".jsonl", raw, t0.Add(2*time.Minute))
	}
	writeVersion(t, st, base, "work/shared.txt", "before", t0.Add(-time.Hour))
	writeVersion(t, st, base, "work/shared.txt", "shared", t0.Add(time.Minute))
	rec := do(t, srv, "GET", "/api/agent/session?agent=claude&id="+url.QueryEscape(".claude/projects/work/one.jsonl"), reqOpts{token: srv.Token()})
	var detail struct {
		Files []sessionFile `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(detail.Files) != 1 || len(detail.Files[0].SharedWith) != 1 || detail.Files[0].SharedWith[0].ID != ".claude/projects/work/two.jsonl" {
		t.Fatalf("shared attribution missing: %s", rec.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"agent": "claude", "session": ".claude/projects/work/one.jsonl", "start": t0.Format(time.RFC3339Nano), "end": t0.Add(5 * time.Minute).Format(time.RFC3339Nano), "dry_run": false})
	rec = do(t, srv, "POST", "/api/agent/revert", reqOpts{token: srv.Token(), body: string(body)})
	var result revertResponse
	json.Unmarshal(rec.Body.Bytes(), &result)
	if rec.Code != 200 || result.SkippedConflict != 1 || result.Reverted != 0 {
		t.Fatalf("shared work overwritten: %s", rec.Body.String())
	}
}

func TestTranscriptExportUsesStoredSnapshotAndAllowsBinary(t *testing.T) {
	srv, st, base := newTestServer(t)
	rel := ".gemini/antigravity/conversations/test.pb"
	writeVersion(t, st, base, rel, "\x00saved binary", time.Now())
	if err := os.WriteFile(st.OriginalPath(rel), []byte("live changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := do(t, srv, "GET", "/api/agent/transcript?agent=gemini&id="+url.QueryEscape(rel), reqOpts{token: srv.Token()})
	if rec.Code != 200 || rec.Body.String() != "\x00saved binary" {
		t.Fatalf("export: %d %q", rec.Code, rec.Body.String())
	}
}
