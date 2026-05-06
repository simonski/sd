package main

import (
	"bytes"
	"encoding/base64"
	"io"
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

func TestCompactSmallSessionFilesCollectsWithoutDeletingOriginals(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), ".respec")
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	smallA := []byte(`[{"dt":"2026-05-03T06:00:00Z","role":"user","text":"hello"}]`)
	smallB := []byte(`[]`)
	large := bytes.Repeat([]byte("x"), 2048)
	if err := os.WriteFile(filepath.Join(sessionsDir, "a.conversation.json"), smallA, 0o644); err != nil {
		t.Fatalf("write small a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "b.conversation.json"), smallB, 0o644); err != nil {
		t.Fatalf("write small b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "large.conversation.json"), large, 0o644); err != nil {
		t.Fatalf("write large: %v", err)
	}
	if err := writeInteractionTimeline(filepath.Join(stateDir, "interactions.ndjson"), []interaction{
		{SessionID: "a", Timestamp: "2026-05-03T06:00:00Z", Command: "copilot", ConversationLog: ".respec/sessions/a.conversation.json"},
		{SessionID: "b", Timestamp: "2026-05-03T06:00:01Z", Command: "copilot", ConversationLog: ".respec/sessions/b.conversation.json"},
		{SessionID: "c", Timestamp: "2026-05-03T06:00:02Z", Command: "copilot", ConversationLog: ".respec/sessions/large.conversation.json"},
	}); err != nil {
		t.Fatalf("write interactions: %v", err)
	}

	if err := compactSmallSessionFiles(stateDir, 1024); err != nil {
		t.Fatalf("compactSmallSessionFiles error: %v", err)
	}

	bundle, err := readCompactBundleFromDB(stateDir)
	if err != nil {
		t.Fatalf("read compact bundle from db: %v", err)
	}
	if len(bundle.Files) != 2 {
		t.Fatalf("expected 2 compacted entries, got %d", len(bundle.Files))
	}

	gotByPath := map[string][]byte{}
	for _, file := range bundle.Files {
		decoded, decodeErr := base64.StdEncoding.DecodeString(file.ContentB64)
		if decodeErr != nil {
			t.Fatalf("decode compacted content for %s: %v", file.Path, decodeErr)
		}
		gotByPath[file.Path] = decoded
	}
	if got := gotByPath[".respec/sessions/a.conversation.json"]; string(got) != string(smallA) {
		t.Fatalf("unexpected compacted content for a: %q", string(got))
	}
	if got := gotByPath[".respec/sessions/b.conversation.json"]; string(got) != string(smallB) {
		t.Fatalf("unexpected compacted content for b: %q", string(got))
	}
	if _, ok := gotByPath[".respec/sessions/large.conversation.json"]; ok {
		t.Fatalf("did not expect large file to be compacted")
	}
	if _, err := os.Stat(filepath.Join(sessionsDir, "a.conversation.json")); err != nil {
		t.Fatalf("expected original file to remain: %v", err)
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
	if !strings.Contains(rendered, "respec doctor\n") {
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
		".respec/interactions.ndjson",
		".respec/sessions/20260430T000000Z-copilot.stdin.log",
		".respec/sessions/20260430T000000Z-copilot.stdout.log",
		"SPEC.md",
		"cmd/respec/main.go",
	}
	got := filterIncrementalFiles(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 filtered files, got %d (%v)", len(got), got)
	}
	if got[0] != "SPEC.md" || got[1] != "cmd/respec/main.go" {
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

func TestPanelAwareWriterReplaysBufferedOutputAfterDismiss(t *testing.T) {
	var out bytes.Buffer
	panel := &specPanelController{panelOpen: true, native: true}
	writer := &panelAwareWriter{dst: &out, panel: panel}

	if _, err := writer.Write([]byte("suppressed")); err != nil {
		t.Fatalf("write while visible returned error: %v", err)
	}
	if got := out.String(); got != "" {
		t.Fatalf("expected suppressed output hidden while panel visible, got %q", got)
	}

	panel.panelOpen = false
	panel.native = false
	if _, err := writer.Write([]byte(" live")); err != nil {
		t.Fatalf("write after dismiss returned error: %v", err)
	}
	if got := out.String(); got != "suppressed live" {
		t.Fatalf("expected buffered output replayed then live output, got %q", got)
	}
}

func TestPanelAwareWriterFlushBufferedOutputWithoutNewWrite(t *testing.T) {
	var out bytes.Buffer
	panel := &specPanelController{panelOpen: true, native: true}
	writer := &panelAwareWriter{dst: &out, panel: panel}

	if _, err := writer.Write([]byte("suppressed")); err != nil {
		t.Fatalf("write while visible returned error: %v", err)
	}
	if got := out.String(); got != "" {
		t.Fatalf("expected suppressed output hidden while panel visible, got %q", got)
	}

	panel.panelOpen = false
	panel.native = false
	if err := writer.flushBuffered(); err != nil {
		t.Fatalf("flushBuffered returned error: %v", err)
	}
	if got := out.String(); got != "suppressed" {
		t.Fatalf("expected buffered output replayed after explicit flush, got %q", got)
	}
}

func TestConsumeEscapeSequence(t *testing.T) {
	tests := []struct {
		name         string
		in           []byte
		wantConsumed int
		wantNeedMore bool
	}{
		{
			name:         "csi complete",
			in:           []byte("\x1b[A"),
			wantConsumed: 3,
			wantNeedMore: false,
		},
		{
			name:         "ss3 complete",
			in:           []byte("\x1bOP"),
			wantConsumed: 3,
			wantNeedMore: false,
		},
		{
			name:         "csi partial",
			in:           []byte("\x1b[12"),
			wantConsumed: 0,
			wantNeedMore: true,
		},
		{
			name:         "single esc",
			in:           []byte{0x1b},
			wantConsumed: 0,
			wantNeedMore: false,
		},
		{
			name:         "not escape sequence",
			in:           []byte("\x1bx"),
			wantConsumed: 0,
			wantNeedMore: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotConsumed, gotNeedMore := consumeEscapeSequence(tc.in)
			if gotConsumed != tc.wantConsumed || gotNeedMore != tc.wantNeedMore {
				t.Fatalf("consumeEscapeSequence(%q) = (%d,%t), want (%d,%t)", string(tc.in), gotConsumed, gotNeedMore, tc.wantConsumed, tc.wantNeedMore)
			}
		})
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

func TestCopyInputWithShortcutsCallsSubmitOnEnter(t *testing.T) {
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	defer stdinR.Close()
	defer stdinW.Close()

	ptmxR, ptmxW, err := os.Pipe()
	if err != nil {
		t.Fatalf("ptmx pipe: %v", err)
	}
	defer ptmxR.Close()
	defer ptmxW.Close()

	var stdinLog bytes.Buffer
	submitted := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		copyInputWithShortcuts(stdinR, ptmxW, &stdinLog, &specPanelController{}, func(line string) {
			submitted <- line
		})
		close(done)
	}()

	if _, err := stdinW.Write([]byte("hello there\r")); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = stdinW.Close()
	<-done
	_ = ptmxW.Close()

	select {
	case got := <-submitted:
		if got != "hello there" {
			t.Fatalf("unexpected submitted line: %q", got)
		}
	default:
		t.Fatalf("expected submitted line callback on enter")
	}

	forwarded, err := io.ReadAll(ptmxR)
	if err != nil {
		t.Fatalf("read ptmx: %v", err)
	}
	if string(forwarded) != "hello there\r" {
		t.Fatalf("unexpected forwarded input: %q", string(forwarded))
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

func TestAssistantMessageFromOutputLine(t *testing.T) {
	if msg, ok := assistantMessageFromOutputLine("● Hello there"); !ok || msg != "Hello there" {
		t.Fatalf("expected assistant message, got ok=%t msg=%q", ok, msg)
	}
	if _, ok := assistantMessageFromOutputLine("● Loading environment: 3 skills"); ok {
		t.Fatalf("expected loading line to be filtered")
	}
	if _, ok := assistantMessageFromOutputLine("plain output"); ok {
		t.Fatalf("expected non-assistant line to be ignored")
	}
}

func TestLineSubmitWriterEmitsOnlyOnNewline(t *testing.T) {
	var dst bytes.Buffer
	var lines []string
	w := &lineSubmitWriter{
		dst: &dst,
		onLine: func(line string) {
			lines = append(lines, line)
		},
	}
	if _, err := w.Write([]byte("● Hel")); err != nil {
		t.Fatalf("write chunk 1: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected no emitted lines before newline, got %v", lines)
	}
	if _, err := w.Write([]byte("lo\n● World\r\n")); err != nil {
		t.Fatalf("write chunk 2: %v", err)
	}
	if got := dst.String(); got != "● Hello\n● World\r\n" {
		t.Fatalf("unexpected dst content: %q", got)
	}
	if len(lines) != 2 || lines[0] != "● Hello" || lines[1] != "● World" {
		t.Fatalf("unexpected emitted lines: %v", lines)
	}
}

func TestRunLsNoArgsPrintsSelectionHintAndExitsZero(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".respec"), 0o755); err != nil {
		t.Fatalf("mkdir .respec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".respec", "interactions.ndjson"), []byte(`{"session_id":"s1","timestamp":"2026-04-30T00:00:00Z","command":"copilot","args":[],"exit_code":0}`+"\n"), 0o644); err != nil {
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
	if !strings.Contains(rendered, "To inspect one session, run `respec ls N`") {
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
		{Timestamp: "t1", Command: "copilot", StdinLog: ".respec/sessions/a.stdin.log"},
		{Timestamp: "t2", Command: "copilot", StdinLog: ".respec/sessions/a.stdin.log"},
	}
	got := summarizeSessions(in, map[string]struct{}{}, map[string]int{"legacy:.respec/sessions/a.stdin.log": 0})
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
	stateDir := filepath.Join(repo, ".respec")
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	conversationRel := ".respec/sessions/s1.conversation.json"
	if err := writeConversationLog(filepath.Join(repo, filepath.FromSlash(conversationRel)), []conversationMessage{
		{Dt: "2026-04-30T10:00:00Z", Role: "user", Text: "in"},
		{Dt: "2026-04-30T10:00:01Z", Role: "assistant", Text: "out"},
	}); err != nil {
		t.Fatalf("write conversation: %v", err)
	}
	timeline := []interaction{
		{SessionID: "s1", Timestamp: "2026-04-30T10:00:00Z", Command: "copilot", EventType: eventTypeFinal, ConversationLog: conversationRel},
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
	if removedInteractions != 1 || removedFiles != 1 {
		t.Fatalf("unexpected removal counts: interactions=%d files=%d", removedInteractions, removedFiles)
	}

	got, err := readInteractionTimeline(interactionsPath)
	if err != nil {
		t.Fatalf("read timeline: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != "s2" {
		t.Fatalf("expected only s2 interaction remaining, got %+v", got)
	}
	gotMessages, err := readConversationLog(filepath.Join(repo, filepath.FromSlash(conversationRel)))
	if err != nil {
		t.Fatalf("readConversationLog: %v", err)
	}
	if len(gotMessages) != 0 {
		t.Fatalf("expected conversation log cleared, got %+v", gotMessages)
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

	firstDate := entries[0].Timestamp.Local().Format("2006-01-02")
	thirdDate := entries[2].Timestamp.Local().Format("2006-01-02")
	if !strings.Contains(rendered, historyDayColorStart+firstDate+historyColorReset) || !strings.Contains(rendered, historyDayColorStart+thirdDate+historyColorReset) {
		t.Fatalf("expected date group headers, got %q", rendered)
	}

	firstTime := entries[0].Timestamp.Local().Format("15:04:05")
	secondTime := entries[1].Timestamp.Local().Format("15:04:05")
	thirdTime := entries[2].Timestamp.Local().Format("15:04:05")
	if !strings.Contains(rendered, firstTime+" | first input line") || !strings.Contains(rendered, secondTime+" | second") || !strings.Contains(rendered, thirdTime+" | third") {
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
	sessionsDir := filepath.Join(repoRoot, ".respec", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	conversationRel := ".respec/sessions/s1.conversation.json"
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

	got := collectInputHistoryEntries(repoRoot, interactions, map[string]struct{}{}, false, false)
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

func TestCollectInputHistoryEntriesIncludesAssistantOutputWhenEnabled(t *testing.T) {
	repoRoot := t.TempDir()
	sessionsDir := filepath.Join(repoRoot, ".respec", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	conversationRel := ".respec/sessions/s1.conversation.json"
	conversationPath := filepath.Join(repoRoot, filepath.FromSlash(conversationRel))
	conversation := []byte(`[
  {"dt":"2026-05-01T10:00:01Z","role":"user","text":"first prompt"},
  {"dt":"2026-05-01T10:00:02Z","role":"assistant","text":"first response"},
  {"dt":"2026-05-01T10:00:03Z","role":"user","text":"second prompt"}
]`)
	if err := os.WriteFile(conversationPath, conversation, 0o644); err != nil {
		t.Fatalf("write conversation: %v", err)
	}

	interactions := []interaction{
		{
			EventType:       eventTypeFinal,
			SessionID:       "s1",
			Timestamp:       "2026-05-01T10:00:04Z",
			Command:         "copilot",
			ConversationLog: conversationRel,
		},
	}

	got := collectInputHistoryEntries(repoRoot, interactions, map[string]struct{}{}, false, true)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d (%+v)", len(got), got)
	}
	if got[1].Text != "< first response" || !got[1].IsOutput {
		t.Fatalf("expected assistant response entry, got %+v", got[1])
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

	interactionsPath := filepath.Join(repo, ".respec", "interactions.ndjson")
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
	if err := os.MkdirAll(filepath.Join(repo, ".respec", "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	stdinRel := ".respec/sessions/s1.stdin.log"
	if err := os.WriteFile(filepath.Join(repo, stdinRel), []byte("this is an input\r"), 0o644); err != nil {
		t.Fatalf("write stdin log: %v", err)
	}
	raw := `{"session_id":"s1","timestamp":"2026-04-30T10:00:00Z","command":"copilot","args":[],"event_type":"final","exit_code":0,"stdin_log":".respec/sessions/s1.stdin.log","stdout_log":".respec/sessions/s1.stdout.log"}` + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".respec", "interactions.ndjson"), []byte(raw), 0o644); err != nil {
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

func TestRunHistoryIncludesOutputWithFlag(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".respec", "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	conversationRel := ".respec/sessions/s1.conversation.json"
	conversation := []byte(`[
  {"dt":"2026-05-01T10:00:01Z","role":"user","text":"first prompt"},
  {"dt":"2026-05-01T10:00:02Z","role":"assistant","text":"first response"}
]`)
	if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(conversationRel)), conversation, 0o644); err != nil {
		t.Fatalf("write conversation: %v", err)
	}
	raw := `{"session_id":"s1","timestamp":"2026-05-01T10:00:03Z","command":"copilot","args":[],"event_type":"final","exit_code":0,"conversation_log":".respec/sessions/s1.conversation.json"}` + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".respec", "interactions.ndjson"), []byte(raw), 0o644); err != nil {
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
	code, err := run([]string{"history", "-o"}, &out, &out)
	if err != nil {
		t.Fatalf("run history error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	rendered := out.String()

	userTS, err := time.Parse(time.RFC3339, "2026-05-01T10:00:01Z")
	if err != nil {
		t.Fatalf("parse user dt: %v", err)
	}
	assistantTS, err := time.Parse(time.RFC3339, "2026-05-01T10:00:02Z")
	if err != nil {
		t.Fatalf("parse assistant dt: %v", err)
	}
	userTime := userTS.Local().Format("15:04:05")
	assistantTime := assistantTS.Local().Format("15:04:05")
	if !strings.Contains(rendered, "  "+userTime+" | first prompt") {
		t.Fatalf("expected user prompt in output, got %q", rendered)
	}
	if !strings.Contains(rendered, "   "+assistantTime+" | < first response") {
		t.Fatalf("expected indented assistant output row, got %q", rendered)
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
	if err := os.MkdirAll(filepath.Join(repo, ".respec", "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	stdinRel := ".respec/sessions/s1.stdin.log"
	if err := os.WriteFile(filepath.Join(repo, stdinRel), []byte("alpha\rbravo\r"), 0o644); err != nil {
		t.Fatalf("write stdin log: %v", err)
	}
	raw := `{"session_id":"s1","timestamp":"2026-04-30T10:00:00Z","command":"copilot","args":[],"event_type":"final","exit_code":0,"stdin_log":".respec/sessions/s1.stdin.log","stdout_log":".respec/sessions/s1.stdout.log"}` + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".respec", "interactions.ndjson"), []byte(raw), 0o644); err != nil {
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

func TestAppendConversationUserMessageCreatesConversationLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.conversation.json")
	if err := appendConversationUserMessage(path, "hello there"); err != nil {
		t.Fatalf("appendConversationUserMessage: %v", err)
	}
	got, err := readConversationLog(path)
	if err != nil {
		t.Fatalf("readConversationLog: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d (%+v)", len(got), got)
	}
	if got[0].Role != "user" || got[0].Text != "hello there" {
		t.Fatalf("unexpected message: %+v", got[0])
	}
	if strings.TrimSpace(got[0].Dt) == "" {
		t.Fatalf("expected dt on appended message, got %+v", got[0])
	}
}

func TestAppendConversationUserMessageAppendsToExistingLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.conversation.json")
	seed := []conversationMessage{
		{Dt: "2026-05-03T08:00:00Z", Role: "assistant", Text: "hello"},
	}
	if err := writeConversationLog(path, seed); err != nil {
		t.Fatalf("writeConversationLog seed: %v", err)
	}
	if err := appendConversationUserMessage(path, "new prompt"); err != nil {
		t.Fatalf("appendConversationUserMessage: %v", err)
	}
	got, err := readConversationLog(path)
	if err != nil {
		t.Fatalf("readConversationLog: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d (%+v)", len(got), got)
	}
	if got[0].Role != "assistant" || got[0].Text != "hello" {
		t.Fatalf("unexpected first message: %+v", got[0])
	}
	if got[1].Role != "user" || got[1].Text != "new prompt" {
		t.Fatalf("unexpected second message: %+v", got[1])
	}
}

func TestMigrateConversationLogsCreatesSingleJson(t *testing.T) {
	repo := t.TempDir()
	stateDir := filepath.Join(repo, ".respec")
	if err := os.MkdirAll(filepath.Join(stateDir, "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	stdinRel := ".respec/sessions/s1.stdin.log"
	stdoutRel := ".respec/sessions/s1.stdout.log"
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
