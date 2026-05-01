package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSemanticVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "0.0.1", want: "0.0.1"},
		{in: "v1.2.3", want: "1.2.3"},
		{in: "v1.2.3-4-gabc123", want: "1.2.3"},
		{in: "1.2.3+build.7", want: "1.2.3"},
		{in: "dev", want: "0.0.0"},
	}

	for _, tc := range tests {
		if got := semanticVersion(tc.in); got != tc.want {
			t.Fatalf("semanticVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRunVersionPrintsSemanticOnly(t *testing.T) {
	original := version
	version = "v1.2.3-4-gabc123"
	t.Cleanup(func() { version = original })

	var out bytes.Buffer
	code, err := run([]string{"version"}, &out, &out)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if got := out.String(); got != "1.2.3\n" {
		t.Fatalf("expected semantic version only, got %q", got)
	}
}

func TestDetectTerminalDoctorInfoMacOSTerminal(t *testing.T) {
	env := map[string]string{
		"TERM_PROGRAM": "Apple_Terminal",
		"TERM":         "xterm-256color",
	}
	info := detectTerminalDoctorInfo(func(key string) string { return env[key] })
	if info.TerminalName != "macOS Terminal" {
		t.Fatalf("expected macOS Terminal, got %q", info.TerminalName)
	}
	if info.InTmux {
		t.Fatalf("expected not in tmux")
	}
	if !info.OverlaySupport {
		t.Fatalf("expected overlay support true for macOS Terminal")
	}
	if !strings.Contains(info.OverlayStatus, "native macOS Terminal overlay") {
		t.Fatalf("unexpected overlay status: %q", info.OverlayStatus)
	}
}

func TestDetectTerminalDoctorInfoTmux(t *testing.T) {
	env := map[string]string{
		"TERM_PROGRAM": "Apple_Terminal",
		"TERM":         "screen-256color",
		"TMUX":         "/tmp/tmux-501/default,123,0",
		"TMUX_PANE":    "%5",
	}
	info := detectTerminalDoctorInfo(func(key string) string { return env[key] })
	if info.TerminalName != "tmux" {
		t.Fatalf("expected tmux, got %q", info.TerminalName)
	}
	if !info.InTmux {
		t.Fatalf("expected in tmux")
	}
	if !info.OverlaySupport {
		t.Fatalf("expected overlay support true in tmux")
	}
}

func TestRunDoctorPrintsDiagnostics(t *testing.T) {
	var out bytes.Buffer
	code, err := run([]string{"doctor"}, &out, &out)
	if err != nil {
		t.Fatalf("run doctor error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "sd doctor\n") {
		t.Fatalf("expected doctor header, got %q", rendered)
	}
	if !strings.Contains(rendered, "Terminal: ") {
		t.Fatalf("expected terminal line, got %q", rendered)
	}
	if !strings.Contains(rendered, "Panel overlay support: ") {
		t.Fatalf("expected overlay support line, got %q", rendered)
	}
}

func TestFindRepoRootFromPrefersNearestWorkspace(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	work := filepath.Join(project, "work")

	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "SPEC.md"), []byte("# spec"), 0o644); err != nil {
		t.Fatalf("write SPEC.md: %v", err)
	}

	got, err := findRepoRootFrom(work)
	if err != nil {
		t.Fatalf("findRepoRootFrom error: %v", err)
	}
	if got != project {
		t.Fatalf("expected %q, got %q", project, got)
	}
}

func TestFindRepoRootFromFallsBackToGitRoot(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "nested", "work")

	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}

	got, err := findRepoRootFrom(work)
	if err != nil {
		t.Fatalf("findRepoRootFrom error: %v", err)
	}
	if got != root {
		t.Fatalf("expected %q, got %q", root, got)
	}
}

func TestChangedFilesBetween(t *testing.T) {
	before := map[string]string{
		"a.txt": " M",
		"b.txt": "??",
	}
	after := map[string]string{
		"a.txt": "MM", // status changed
		"b.txt": "??", // unchanged
		"c.txt": " M", // new in snapshot
	}

	got := changedFilesBetween(before, after)
	if len(got) != 2 {
		t.Fatalf("expected 2 changed files, got %d (%v)", len(got), got)
	}
	if got[0] != "a.txt" || got[1] != "c.txt" {
		t.Fatalf("unexpected changed files: %v", got)
	}
}

func TestFilterIncrementalFilesSkipsSessionArtifacts(t *testing.T) {
	in := []string{
		".sd/interactions.ndjson",
		".sd/sessions/20260430T000000Z-copilot.stdin.log",
		".sd/sessions/20260430T000000Z-copilot.stdout.log",
		"SPEC.md",
		"cmd/sd/main.go",
	}
	got := filterIncrementalFiles(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 filtered files, got %d (%v)", len(got), got)
	}
	if got[0] != "SPEC.md" || got[1] != "cmd/sd/main.go" {
		t.Fatalf("unexpected filtered files: %v", got)
	}
}

func TestSanitizeInputLogStripsControlCodes(t *testing.T) {
	raw := []byte("\x1b]11;rgb:ffff/ffff/ffff\x07hello\x7f!\r\x03\n")
	got := sanitizeInputLog(raw)
	if !strings.Contains(got, "hell!") {
		t.Fatalf("expected cleaned input text, got %q", got)
	}
	if !strings.Contains(got, "<CTRL-C>") {
		t.Fatalf("expected ctrl-c marker, got %q", got)
	}
	if strings.Contains(got, "\x1b") {
		t.Fatalf("expected escape sequences removed, got %q", got)
	}
}

func TestSanitizeOutputLogStripsAnsi(t *testing.T) {
	raw := []byte("\x1b[31mRED\x1b[39m\x1b]2;title\x07\r\nok\n")
	got := sanitizeOutputLog(raw)
	if strings.Contains(got, "\x1b") {
		t.Fatalf("expected escape sequences removed, got %q", got)
	}
	if !strings.Contains(got, "RED") || !strings.Contains(got, "ok") {
		t.Fatalf("expected printable content retained, got %q", got)
	}
}

func TestIsPanelToggleShortcutKey(t *testing.T) {
	tests := []struct {
		key  byte
		want bool
	}{
		{key: 0x1b, want: true},
		{key: '`', want: true},
		{key: '~', want: true},
		{key: 'a', want: false},
		{key: ' ', want: false},
	}
	for _, tc := range tests {
		if got := isPanelToggleShortcutKey(tc.key); got != tc.want {
			t.Fatalf("isPanelToggleShortcutKey(%q) = %t, want %t", tc.key, got, tc.want)
		}
	}
}

func TestConsumeDoubleShiftSequence(t *testing.T) {
	tests := []struct {
		name         string
		in           []byte
		wantConsumed int
		wantMatch    bool
	}{
		{
			name:         "kitty-left-shift",
			in:           []byte("\x1b[57441;2u"),
			wantConsumed: len("\x1b[57441;2u"),
			wantMatch:    true,
		},
		{
			name:         "kitty-right-shift-with-extra-modifier",
			in:           []byte("\x1b[57447;3u"),
			wantConsumed: len("\x1b[57447;3u"),
			wantMatch:    true,
		},
		{
			name:         "not-shift",
			in:           []byte("\x1b[65;2u"),
			wantConsumed: 0,
			wantMatch:    false,
		},
		{
			name:         "non-u-final",
			in:           []byte("\x1b[57441;2A"),
			wantConsumed: 0,
			wantMatch:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotConsumed, gotMatch := consumeDoubleShiftSequence(tc.in)
			if gotConsumed != tc.wantConsumed || gotMatch != tc.wantMatch {
				t.Fatalf("consumeDoubleShiftSequence(%q) = (%d,%t), want (%d,%t)", string(tc.in), gotConsumed, gotMatch, tc.wantConsumed, tc.wantMatch)
			}
		})
	}
}

func TestPanelAwareWriterSuppressesDisplayWhenPanelVisible(t *testing.T) {
	var out bytes.Buffer
	panel := &specPanelController{panelOpen: true, native: true}
	writer := &panelAwareWriter{dst: &out, panel: panel}

	n, err := writer.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write returned error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected write count 5, got %d", n)
	}
	if got := out.String(); got != "" {
		t.Fatalf("expected output to be suppressed, got %q", got)
	}
}

func TestPanelAwareWriterForwardsWhenPanelHidden(t *testing.T) {
	var out bytes.Buffer
	panel := &specPanelController{}
	writer := &panelAwareWriter{dst: &out, panel: panel}

	n, err := writer.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write returned error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected write count 5, got %d", n)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("expected output to pass through, got %q", got)
	}
}

func TestNativeOverlayFrameColumnsSlidesFromLeft(t *testing.T) {
	cols := nativeOverlayFrameColumns(120, 80)
	if len(cols) < 2 {
		t.Fatalf("expected animation columns, got %v", cols)
	}
	if cols[0] != 1 {
		t.Fatalf("expected first column 1, got %d", cols[0])
	}
	if cols[len(cols)-1] != 41 {
		t.Fatalf("expected final column 41, got %d", cols[len(cols)-1])
	}
	for i := 1; i < len(cols); i++ {
		if cols[i] < cols[i-1] {
			t.Fatalf("expected non-decreasing columns, got %v", cols)
		}
	}
}

func TestNativeOverlayFrameColumnsNoAnimationWhenFullWidth(t *testing.T) {
	cols := nativeOverlayFrameColumns(80, 80)
	if len(cols) != 1 || cols[0] != 1 {
		t.Fatalf("expected single final column, got %v", cols)
	}
}

func TestExtractDialog(t *testing.T) {
	input := "hello?\n<CTRL-C>\n"
	output := "● Loading environment: 3 skills\n● Hey! I’m here and ready to help.\n"
	got := extractDialog(input, output)
	if len(got) != 2 {
		t.Fatalf("expected 2 dialog lines, got %d (%v)", len(got), got)
	}
	if got[0] != "User: hello?" {
		t.Fatalf("unexpected user line: %q", got[0])
	}
	if !strings.Contains(got[1], "Assistant: Hey!") {
		t.Fatalf("unexpected assistant line: %q", got[1])
	}
}

func TestRunLsNoArgsPrintsSelectionHintAndExitsZero(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".sd"), 0o755); err != nil {
		t.Fatalf("mkdir .sd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sd", "interactions.ndjson"), []byte(`{"session_id":"s1","timestamp":"2026-04-30T00:00:00Z","command":"copilot","args":[],"exit_code":0}`+"\n"), 0o644); err != nil {
		t.Fatalf("write interactions: %v", err)
	}

	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(prev)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var out bytes.Buffer
	code, err := runLs(nil, &out)
	if err != nil {
		t.Fatalf("runLs error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	rendered := out.String()
	if !strings.Contains(rendered, "To inspect one session, run `sd ls N`") {
		t.Fatalf("expected selection hint, got %q", rendered)
	}
	if strings.Contains(rendered, "Select session number to view") {
		t.Fatalf("did not expect interactive prompt, got %q", rendered)
	}
}

func TestReadInteractionsCollapsesSessionToLatestRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "interactions.ndjson")
	raw := strings.Join([]string{
		`{"timestamp":"2026-04-30T10:00:00Z","command":"copilot","args":[],"exit_code":0,"stdin_log":"a","stdout_log":"b"}`,
		`{"session_id":"s1","timestamp":"2026-04-30T10:01:00Z","command":"copilot","args":[],"exit_code":-1,"in_progress":true,"stdin_log":"a","stdout_log":"b","modified_files":["x.txt"]}`,
		`{"session_id":"s1","timestamp":"2026-04-30T10:02:00Z","command":"copilot","args":[],"exit_code":0,"stdin_log":"a","stdout_log":"b","modified_files":["x.txt","y.txt"]}`,
		`{"session_id":"s2","timestamp":"2026-04-30T10:03:00Z","command":"copilot","args":[],"exit_code":-1,"in_progress":true,"stdin_log":"a","stdout_log":"b","modified_files":["z.txt"]}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write interactions: %v", err)
	}

	got, err := readInteractions(path)
	if err != nil {
		t.Fatalf("readInteractions error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 records (legacy + s1 + s2), got %d (%v)", len(got), got)
	}

	if got[1].SessionID != "s1" || got[1].InProgress || got[1].ExitCode != 0 {
		t.Fatalf("expected s1 to be latest completed record, got %+v", got[1])
	}
	if got[2].SessionID != "s2" || !got[2].InProgress || got[2].ExitCode != -1 {
		t.Fatalf("expected s2 to remain in-progress record, got %+v", got[2])
	}
}

func TestReadInteractionTimelineKeepsAllEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "interactions.ndjson")
	raw := strings.Join([]string{
		`{"session_id":"s1","timestamp":"2026-04-30T10:01:00Z","command":"copilot","args":[],"exit_code":-1,"in_progress":true,"stdin_log":"a","stdout_log":"b","modified_files":["x.txt"]}`,
		`{"session_id":"s1","timestamp":"2026-04-30T10:02:00Z","command":"copilot","args":[],"exit_code":-1,"in_progress":true,"stdin_log":"a","stdout_log":"b","modified_files":["x.txt","y.txt"]}`,
		`{"session_id":"s1","timestamp":"2026-04-30T10:03:00Z","command":"copilot","args":[],"exit_code":0,"stdin_log":"a","stdout_log":"b","modified_files":["x.txt","y.txt"]}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write interactions: %v", err)
	}

	got, err := readInteractionTimeline(path)
	if err != nil {
		t.Fatalf("readInteractionTimeline error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected all 3 events, got %d (%v)", len(got), got)
	}
	if !got[0].InProgress || !got[1].InProgress || got[2].InProgress {
		t.Fatalf("expected in-progress/in-progress/final timeline, got %+v", got)
	}
}

func TestOrderedInputRecordsReturnsFirstPerSessionInOrder(t *testing.T) {
	in := []interaction{
		{SessionID: "s1", Timestamp: "t1", StdinLog: "a"},
		{SessionID: "s1", Timestamp: "t2", StdinLog: "a"},
		{SessionID: "s2", Timestamp: "t3", StdinLog: "b"},
		{SessionID: "", Timestamp: "t4", StdinLog: "legacy-1"},
		{SessionID: "", Timestamp: "t5", StdinLog: "legacy-1"},
	}
	got := orderedInputRecords(in)
	if len(got) != 3 {
		t.Fatalf("expected 3 ordered input records, got %d (%v)", len(got), got)
	}
	if got[0].Timestamp != "t1" || got[1].Timestamp != "t3" || got[2].Timestamp != "t4" {
		t.Fatalf("unexpected record order: %+v", got)
	}
}

func TestSummarizeSessionsCountsInteractionsAndRespectsHidden(t *testing.T) {
	in := []interaction{
		{SessionID: "s1", Timestamp: "t1", Command: "copilot", ExitCode: -1, InProgress: true},
		{SessionID: "s1", Timestamp: "t2", Command: "copilot", ExitCode: 0},
		{SessionID: "s2", Timestamp: "t3", Command: "codex", ExitCode: 0},
	}
	hidden := map[string]struct{}{"s2": {}}
	got := filterSessionSummaries(summarizeSessions(in, hidden, map[string]int{"s1": 0, "s2": 1}), lsOptions{})
	if len(got) != 1 {
		t.Fatalf("expected only visible session summary, got %d (%v)", len(got), got)
	}
	if got[0].SessionID != "s1" || got[0].InteractionCount != 2 || got[0].LastTimestamp != "t2" || got[0].Latest.ExitCode != 0 {
		t.Fatalf("unexpected session summary: %+v", got[0])
	}
}

func TestWriteAndReadHiddenSessionIDsRoundTrip(t *testing.T) {
	stateDir := t.TempDir()
	hidden := map[string]struct{}{
		"s2": {},
		"s1": {},
	}
	if err := writeHiddenSessionIDs(stateDir, hidden); err != nil {
		t.Fatalf("writeHiddenSessionIDs error: %v", err)
	}

	got, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		t.Fatalf("readHiddenSessionIDs error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 hidden sessions, got %d (%v)", len(got), got)
	}
	if _, ok := got["s1"]; !ok {
		t.Fatalf("missing hidden s1")
	}
	if _, ok := got["s2"]; !ok {
		t.Fatalf("missing hidden s2")
	}
}

func TestSummarizeSessionsIncludesLegacyByStdinLog(t *testing.T) {
	in := []interaction{
		{Timestamp: "t1", Command: "copilot", StdinLog: ".sd/sessions/a.stdin.log"},
		{Timestamp: "t2", Command: "copilot", StdinLog: ".sd/sessions/a.stdin.log"},
	}
	got := summarizeSessions(in, map[string]struct{}{}, map[string]int{"legacy:.sd/sessions/a.stdin.log": 0})
	if len(got) != 1 {
		t.Fatalf("expected one legacy summary, got %d (%v)", len(got), got)
	}
	if got[0].SessionID != "(legacy)" || got[0].InteractionCount != 2 {
		t.Fatalf("unexpected legacy summary: %+v", got[0])
	}
}

func TestParseLsOptions(t *testing.T) {
	opts, err := parseLsOptions([]string{"-a", "--active", "--agent", "copilot", "--since", "2026-04-30", "--verbose", "2"})
	if err != nil {
		t.Fatalf("parseLsOptions error: %v", err)
	}
	if !opts.All || !opts.Active || opts.Agent != "copilot" || !opts.HasSince || !opts.Verbose || !opts.HasIndex || opts.Index != 2 {
		t.Fatalf("unexpected parsed options: %+v", opts)
	}
}

func TestFilterSessionSummariesSupportsHiddenAndAgent(t *testing.T) {
	sessions := []sessionSummary{
		{SessionID: "s1", Hidden: false, Command: "copilot", Latest: interaction{InProgress: true}, LastTimestamp: "2026-04-30T10:00:00Z"},
		{SessionID: "s2", Hidden: true, Command: "codex", Latest: interaction{InProgress: false}, LastTimestamp: "2026-04-30T10:01:00Z"},
	}
	got := filterSessionSummaries(sessions, lsOptions{Hidden: true, Agent: "codex"})
	if len(got) != 1 || got[0].SessionID != "s2" {
		t.Fatalf("expected hidden codex session, got %+v", got)
	}
}

func TestHardDeleteSessionRemovesEventsAndLogs(t *testing.T) {
	repo := t.TempDir()
	stateDir := filepath.Join(repo, ".sd")
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	stdinRel := ".sd/sessions/s1.stdin.log"
	stdoutRel := ".sd/sessions/s1.stdout.log"
	if err := os.WriteFile(filepath.Join(repo, stdinRel), []byte("in"), 0o644); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, stdoutRel), []byte("out"), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	timeline := []interaction{
		{SessionID: "s1", Timestamp: "2026-04-30T10:00:00Z", Command: "copilot", EventType: eventTypeFinal, StdinLog: stdinRel, StdoutLog: stdoutRel},
		{SessionID: "s2", Timestamp: "2026-04-30T10:01:00Z", Command: "copilot", EventType: eventTypeFinal},
	}
	if err := writeInteractionTimeline(interactionsPath, timeline); err != nil {
		t.Fatalf("write timeline: %v", err)
	}
	hidden := map[string]struct{}{"s1": {}}
	if err := writeHiddenSessionIDs(stateDir, hidden); err != nil {
		t.Fatalf("write hidden: %v", err)
	}

	removedInteractions, removedFiles, err := hardDeleteSession(repo, stateDir, "s1", timeline, hidden)
	if err != nil {
		t.Fatalf("hardDeleteSession error: %v", err)
	}
	if removedInteractions != 1 || removedFiles != 2 {
		t.Fatalf("unexpected removal counts: interactions=%d files=%d", removedInteractions, removedFiles)
	}

	got, err := readInteractionTimeline(interactionsPath)
	if err != nil {
		t.Fatalf("read timeline: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != "s2" {
		t.Fatalf("expected only s2 interaction remaining, got %+v", got)
	}
	if _, err := os.Stat(filepath.Join(repo, stdinRel)); !os.IsNotExist(err) {
		t.Fatalf("expected stdin log removed, stat err=%v", err)
	}
}

func TestExtractInputSequence(t *testing.T) {
	raw := []byte("hello\rworld\r\x03\r\n")
	got := extractInputSequence(raw)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 input lines, got %v", got)
	}
	if got[0] != "hello" || got[1] != "world" {
		t.Fatalf("unexpected input sequence: %v", got)
	}
}

func TestStyleSessionListLineDimsHiddenInAllMode(t *testing.T) {
	got := styleSessionListLine("x", true, lsOptions{All: true})
	if !strings.Contains(got, "\x1b[90m") || !strings.Contains(got, "\x1b[0m") {
		t.Fatalf("expected dim color wrapper, got %q", got)
	}
}

func TestPrintInputHistoryGroupsByDateAndTime(t *testing.T) {
	entries := []inputHistoryEntry{
		{Timestamp: time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC), Text: "first input line"},
		{Timestamp: time.Date(2026, 4, 30, 11, 0, 0, 0, time.UTC), Text: "second"},
		{Timestamp: time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC), Text: "third"},
	}
	var out bytes.Buffer
	printInputHistory(&out, entries)
	rendered := out.String()
	if !strings.Contains(rendered, historyDayColorStart+"2026-04-30"+historyColorReset) || !strings.Contains(rendered, historyDayColorStart+"2026-05-01"+historyColorReset) {
		t.Fatalf("expected date group headers, got %q", rendered)
	}
	if !strings.Contains(rendered, "10:00:00 | first input line") || !strings.Contains(rendered, "11:00:00 | second") || !strings.Contains(rendered, "09:30:00 | third") {
		t.Fatalf("expected time+text rows, got %q", rendered)
	}
	if !strings.Contains(rendered, historyContinuationBlank) {
		t.Fatalf("expected blank continuation row between inputs, got %q", rendered)
	}
	if !strings.Contains(rendered, historyDayDivider) {
		t.Fatalf("expected day divider between dates, got %q", rendered)
	}
}

func TestCollectInputHistoryEntriesIncludesCommandAndConversationUsers(t *testing.T) {
	repoRoot := t.TempDir()
	sessionsDir := filepath.Join(repoRoot, ".sd", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	conversationRel := ".sd/sessions/s1.conversation.json"
	conversationPath := filepath.Join(repoRoot, filepath.FromSlash(conversationRel))
	conversation := []byte(`[
  {"dt":"2026-05-01T10:00:01Z","role":"user","text":"first prompt"},
  {"dt":"2026-05-01T10:00:02Z","role":"assistant","text":"response"},
  {"dt":"2026-05-01T10:00:03Z","role":"user","text":"second prompt"}
]`)
	if err := os.WriteFile(conversationPath, conversation, 0o644); err != nil {
		t.Fatalf("write conversation: %v", err)
	}

	interactions := []interaction{
		{
			EventType:       eventTypeStart,
			SessionID:       "s1",
			Timestamp:       "2026-05-01T10:00:00Z",
			Command:         "copilot",
			Args:            []string{"--resume=Plan Next Steps"},
			ConversationLog: conversationRel,
		},
		{
			EventType:       eventTypeFinal,
			SessionID:       "s1",
			Timestamp:       "2026-05-01T10:00:04Z",
			Command:         "copilot",
			Args:            []string{"--resume=Plan Next Steps"},
			ConversationLog: conversationRel,
		},
	}

	got := collectInputHistoryEntries(repoRoot, interactions, map[string]struct{}{}, false)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d (%+v)", len(got), got)
	}
	if got[0].Text != "$ copilot --resume=Plan Next Steps" {
		t.Fatalf("unexpected first entry: %+v", got[0])
	}
	if got[1].Text != "first prompt" || got[2].Text != "second prompt" {
		t.Fatalf("unexpected conversation entries: %+v", got)
	}
}

func TestWrapWordsNoSplit(t *testing.T) {
	in := "alpha beta gamma delta epsilon zeta eta theta"
	lines := wrapWordsNoSplit(in, 14)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines, got %v", lines)
	}
	for _, line := range lines {
		if strings.Contains(line, "  ") {
			t.Fatalf("expected normalized spacing, got %q", line)
		}
	}
}

func TestRunAgentWritesLifecycleEvents(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "SPEC.md"), []byte("# spec"), 0o644); err != nil {
		t.Fatalf("write SPEC.md: %v", err)
	}

	prevPoll, prevDebounce, prevMin := incrementalPollInterval, incrementalDebounce, incrementalMinInterval
	incrementalPollInterval = 50 * time.Millisecond
	incrementalDebounce = 100 * time.Millisecond
	incrementalMinInterval = 100 * time.Millisecond
	defer func() {
		incrementalPollInterval = prevPoll
		incrementalDebounce = prevDebounce
		incrementalMinInterval = prevMin
	}()

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(prevWD)
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var out bytes.Buffer
	code, err := run([]string{"sh", "-c", "echo first > x.txt; sleep 0.25; echo second >> x.txt; sleep 0.25"}, &out, &out)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	interactionsPath := filepath.Join(repo, ".sd", "interactions.ndjson")
	events, err := readInteractionTimeline(interactionsPath)
	if err != nil {
		t.Fatalf("readInteractionTimeline: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d (%v)", len(events), events)
	}
	var hasStart, hasFinal bool
	for _, event := range events {
		if event.EventType == eventTypeStart {
			hasStart = true
		}
		if event.EventType == eventTypeFinal {
			hasFinal = true
		}
		if event.SchemaVersion == 0 {
			t.Fatalf("expected schema version set, got %+v", event)
		}
	}
	if !hasStart || !hasFinal {
		t.Fatalf("expected start and final events, got %+v", events)
	}
}

func TestRunHistoryAliasesInputs(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".sd", "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	stdinRel := ".sd/sessions/s1.stdin.log"
	if err := os.WriteFile(filepath.Join(repo, stdinRel), []byte("this is an input\r"), 0o644); err != nil {
		t.Fatalf("write stdin log: %v", err)
	}
	raw := `{"session_id":"s1","timestamp":"2026-04-30T10:00:00Z","command":"copilot","args":[],"event_type":"final","exit_code":0,"stdin_log":".sd/sessions/s1.stdin.log","stdout_log":".sd/sessions/s1.stdout.log"}` + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".sd", "interactions.ndjson"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write interactions: %v", err)
	}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(prevWD)
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var out bytes.Buffer
	code, err := run([]string{"history"}, &out, &out)
	if err != nil {
		t.Fatalf("run history error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out.String(), "this is an input") {
		t.Fatalf("expected history output to include input sequence, got %q", out.String())
	}
}

func TestRetrofitSessionIndexAssignsStableNumbers(t *testing.T) {
	stateDir := t.TempDir()
	interactions := []interaction{
		{SessionID: "s1", Timestamp: "2026-04-30T10:00:00Z"},
		{SessionID: "s2", Timestamp: "2026-04-30T10:01:00Z"},
	}
	got, err := retrofitSessionIndex(stateDir, interactions)
	if err != nil {
		t.Fatalf("retrofitSessionIndex error: %v", err)
	}
	if got["s1"] != 0 || got["s2"] != 1 {
		t.Fatalf("unexpected session numbers: %v", got)
	}
	// Re-running should preserve numbers.
	got2, err := retrofitSessionIndex(stateDir, interactions)
	if err != nil {
		t.Fatalf("retrofitSessionIndex rerun error: %v", err)
	}
	if got2["s1"] != 0 || got2["s2"] != 1 {
		t.Fatalf("expected stable numbers, got %v", got2)
	}
}

func TestRunGetReturnsSessionInputByStableNumber(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".sd", "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	stdinRel := ".sd/sessions/s1.stdin.log"
	if err := os.WriteFile(filepath.Join(repo, stdinRel), []byte("alpha\rbravo\r"), 0o644); err != nil {
		t.Fatalf("write stdin log: %v", err)
	}
	raw := `{"session_id":"s1","timestamp":"2026-04-30T10:00:00Z","command":"copilot","args":[],"event_type":"final","exit_code":0,"stdin_log":".sd/sessions/s1.stdin.log","stdout_log":".sd/sessions/s1.stdout.log"}` + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".sd", "interactions.ndjson"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write interactions: %v", err)
	}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(prevWD)
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var out bytes.Buffer
	code, err := run([]string{"get", "0"}, &out, &out)
	if err != nil {
		t.Fatalf("run get error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out.String(), "alpha") || !strings.Contains(out.String(), "bravo") {
		t.Fatalf("expected cleaned input output, got %q", out.String())
	}
}

func TestBuildConversationMessages(t *testing.T) {
	in := []byte("hello there\r")
	out := []byte("● Hi back\n")
	got := buildConversationMessages(in, out)
	if len(got) < 2 {
		t.Fatalf("expected user/assistant messages, got %+v", got)
	}
	if got[0].Role != "user" || !strings.Contains(got[0].Text, "hello") {
		t.Fatalf("unexpected first message: %+v", got[0])
	}
	if got[1].Role != "assistant" || !strings.Contains(strings.ToLower(got[1].Text), "hi") {
		t.Fatalf("unexpected second message: %+v", got[1])
	}
	if strings.TrimSpace(got[0].Dt) == "" || strings.TrimSpace(got[1].Dt) == "" {
		t.Fatalf("expected dt on all messages, got %+v", got)
	}
}

func TestMigrateConversationLogsCreatesSingleJson(t *testing.T) {
	repo := t.TempDir()
	stateDir := filepath.Join(repo, ".sd")
	if err := os.MkdirAll(filepath.Join(stateDir, "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	stdinRel := ".sd/sessions/s1.stdin.log"
	stdoutRel := ".sd/sessions/s1.stdout.log"
	if err := os.WriteFile(filepath.Join(repo, stdinRel), []byte("hello\r"), 0o644); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, stdoutRel), []byte("● world\n"), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	interactions := []interaction{
		{SessionID: "s1", Timestamp: "2026-04-30T10:00:00Z", Command: "copilot", StdinLog: stdinRel, StdoutLog: stdoutRel},
	}
	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	if err := writeInteractionTimeline(interactionsPath, interactions); err != nil {
		t.Fatalf("write interactions: %v", err)
	}
	if err := migrateConversationLogs(repo, interactionsPath, interactions); err != nil {
		t.Fatalf("migrateConversationLogs: %v", err)
	}
	got, err := readInteractionTimeline(interactionsPath)
	if err != nil {
		t.Fatalf("readInteractionTimeline: %v", err)
	}
	if len(got) != 1 || strings.TrimSpace(got[0].ConversationLog) == "" {
		t.Fatalf("expected conversation log path set, got %+v", got)
	}
	messages, err := readConversationLog(filepath.Join(repo, filepath.FromSlash(got[0].ConversationLog)))
	if err != nil {
		t.Fatalf("readConversationLog: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("expected migrated conversation messages, got none")
	}
	for _, msg := range messages {
		if strings.TrimSpace(msg.Dt) == "" {
			t.Fatalf("expected dt on migrated message, got %+v", msg)
		}
	}
}
