package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/simonski/sd/internal/bootstrap"
	"golang.org/x/term"
)

var version = "dev"

const stateDirName = ".sd"

var (
	incrementalPollInterval = 2 * time.Second
	incrementalDebounce     = 4 * time.Second
	incrementalMinInterval  = 15 * time.Second
)

const (
	interactionSchemaVersion = 2

	eventTypeStart     = "start"
	eventTypeUpdate    = "update"
	eventTypeFinal     = "final"
	eventTypeHide      = "hide"
	eventTypeUnhide    = "unhide"
	eventTypeRemove    = "remove"
	eventTypeTerminate = "terminate"
)

const (
	historyDayColorStart     = "\x1b[38;2;110;160;245m"
	historyColorReset        = "\x1b[0m"
	historyContinuation      = "           | "
	historyContinuationBlank = "           |"
	historyDayDivider        = "-----------+------------------------------------------------------------------------"
	historyTextWrapWidth     = 72
)

type config struct {
	Version      int      `json:"version"`
	SpecPointers []string `json:"spec_pointers"`
}

type interaction struct {
	SchemaVersion   int      `json:"schema_version,omitempty"`
	EventType       string   `json:"event_type,omitempty"`
	SessionID       string   `json:"session_id"`
	Timestamp       string   `json:"timestamp"`
	Command         string   `json:"command"`
	Args            []string `json:"args"`
	ExitCode        int      `json:"exit_code"`
	InProgress      bool     `json:"in_progress,omitempty"`
	InputPreview    string   `json:"input_preview,omitempty"`
	ConversationLog string   `json:"conversation_log,omitempty"`
	StdinLog        string   `json:"stdin_log"`
	StdoutLog       string   `json:"stdout_log"`
	ModifiedFiles   []string `json:"modified_files,omitempty"`
}

type conversationMessage struct {
	Dt   string `json:"dt"`
	Role string `json:"role"`
	Text string `json:"text"`
}

type hiddenSessions struct {
	HiddenSessionIDs []string `json:"hidden_session_ids"`
}

type sessionIndex struct {
	NextID  int            `json:"next_id"`
	Entries map[string]int `json:"entries"`
}

type sessionSummary struct {
	Number           int
	MatchKey         string
	SessionID        string
	Hidden           bool
	FirstTimestamp   string
	LastTimestamp    string
	Command          string
	Args             []string
	InteractionCount int
	Latest           interaction
}

type inputHistoryEntry struct {
	Timestamp time.Time
	SessionID string
	Text      string
}

func main() {
	code, err := run(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sd: %v\n", err)
	}
	os.Exit(code)
}

func run(args []string, out io.Writer, errOut io.Writer) (int, error) {
	if len(args) == 0 {
		printUsage(out)
		return 0, nil
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage(out)
		return 0, nil
	case "version", "--version", "-v":
		fmt.Fprintf(out, "sd %s\n", version)
		return 0, nil
	case "init":
		return runInit(out)
	case "spec":
		return runSpec(out)
	case "ls":
		return runLs(args[1:], out)
	case "cat":
		return runCat(args[1:], out)
	case "hide":
		return runHide(args[1:], out)
	case "unhide":
		return runUnhide(args[1:], out)
	case "rm":
		return runRm(args[1:], out)
	case "prune":
		return runPrune(out)
	case "inputs":
		return runInputs(args[1:], out)
	case "history":
		return runInputs(args[1:], out)
	case "get":
		return runGet(args[1:], out)
	default:
		return runAgent(args[0], args[1:], errOut)
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  sd <command>")
	fmt.Fprintln(out, "  sd <agent-binary> [args...]")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Hardcoded commands:")
	fmt.Fprintln(out, "  help      Show this usage output")
	fmt.Fprintln(out, "  version   Show sd version")
	fmt.Fprintln(out, "  init      Create/update .sd workspace in the current repo")
	fmt.Fprintln(out, "  ls        List sessions, or show abbreviated interactions for one session")
	fmt.Fprintln(out, "  cat       Show full logs/details for one session")
	fmt.Fprintln(out, "  hide      Soft-delete a session from ls/cat")
	fmt.Fprintln(out, "  unhide    Restore a hidden session (`sd unhide N` from `sd ls --hidden`)")
	fmt.Fprintln(out, "  rm        Hard-delete one session (`sd rm N`)")
	fmt.Fprintln(out, "  prune     Remove hidden sessions and orphan artifacts")
	fmt.Fprintln(out, "  inputs    Print user input sequence across sessions")
	fmt.Fprintln(out, "  history   Alias for `sd inputs`")
	fmt.Fprintln(out, "  get       Show cleaned input for one session (`sd get N`)")
	fmt.Fprintln(out, "  spec      Assemble .sd/spec.generated.md from state")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Agent wrapper examples:")
	fmt.Fprintln(out, "  sd copilot")
	fmt.Fprintln(out, "  sd codex")
	fmt.Fprintln(out, "  sd claude")
}

func runInit(out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}

	cfg, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}

	fmt.Fprintf(out, "Initialized %s\n", stateDir)
	fmt.Fprintf(out, "Spec pointers: %s\n", strings.Join(cfg.SpecPointers, ", "))
	return 0, nil
}

func runSpec(out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}

	cfg, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}

	interactions, err := readInteractionTimeline(filepath.Join(stateDir, "interactions.ndjson"))
	if err != nil {
		return 1, err
	}

	var b strings.Builder
	b.WriteString("# Generated Spec View (`sd spec`)\n\n")
	b.WriteString(fmt.Sprintf("- Source repo: `%s`\n", filepath.Base(repoRoot)))
	b.WriteString(fmt.Sprintf("- Interaction schema: v%d\n", interactionSchemaVersion))
	b.WriteString(fmt.Sprintf("- Interaction records: %d\n\n", len(interactions)))
	b.WriteString("## Baseline Specs\n\n")

	for _, relPath := range cfg.SpecPointers {
		specPath := filepath.Join(repoRoot, relPath)
		content, readErr := os.ReadFile(specPath)
		b.WriteString(fmt.Sprintf("### `%s`\n\n", relPath))
		if readErr != nil {
			b.WriteString(fmt.Sprintf("_Unreadable: %v_\n\n", readErr))
			continue
		}
		b.WriteString("```markdown\n")
		b.Write(content)
		b.WriteString("\n```\n\n")
	}

	hiddenSet, hiddenErr := readHiddenSessionIDs(stateDir)
	if hiddenErr != nil {
		return 1, hiddenErr
	}
	numbers, numErr := retrofitSessionIndex(stateDir, interactions)
	if numErr != nil {
		return 1, numErr
	}
	curated := filterSessionSummaries(summarizeSessions(interactions, hiddenSet, numbers), lsOptions{})

	b.WriteString("## Curated Session Summary (latest per visible session)\n\n")
	if len(curated) == 0 {
		b.WriteString("- No captured sessions yet.\n\n")
	} else {
		for _, session := range curated {
			state := fmt.Sprintf("exit=%d", session.Latest.ExitCode)
			if session.Latest.InProgress {
				state = "in-progress"
			}
			b.WriteString(fmt.Sprintf("- %s — `%s %s` (%s, interactions=%d)\n",
				session.LastTimestamp,
				session.Command,
				strings.Join(session.Args, " "),
				state,
				session.InteractionCount,
			))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Session Inputs (in order)\n\n")
	inputs := orderedInputRecords(interactions)
	if len(inputs) == 0 {
		b.WriteString("- No captured inputs yet.\n\n")
	} else {
		for _, item := range inputs {
			cmdLine := strings.TrimSpace(strings.Join(append([]string{item.Command}, item.Args...), " "))
			b.WriteString(fmt.Sprintf("### %s — `%s`\n\n", item.Timestamp, cmdLine))
			if strings.TrimSpace(item.ConversationLog) == "" {
				b.WriteString("[no conversation log path recorded]\n\n")
				continue
			}
			messages, readErr := readConversationLog(filepath.Join(repoRoot, filepath.FromSlash(item.ConversationLog)))
			if readErr != nil {
				b.WriteString(fmt.Sprintf("_Unreadable conversation log (%s): %v_\n\n", item.ConversationLog, readErr))
				continue
			}
			var userLines []string
			for _, msg := range messages {
				if msg.Role != "user" {
					continue
				}
				line := strings.TrimSpace(msg.Text)
				if line != "" {
					userLines = append(userLines, line)
				}
			}
			cleanedInput := strings.Join(userLines, "\n")
			if strings.TrimSpace(cleanedInput) == "" {
				b.WriteString("[no printable input captured]\n\n")
				continue
			}
			b.WriteString("```text\n")
			b.WriteString(cleanedInput)
			if cleanedInput[len(cleanedInput)-1] != '\n' {
				b.WriteString("\n")
			}
			b.WriteString("```\n\n")
		}
	}

	b.WriteString("## Notable Change Sequences (Heuristic)\n\n")
	for _, line := range recentCommitLines(repoRoot, 20) {
		b.WriteString(fmt.Sprintf("- %s\n", line))
	}
	b.WriteString("\n")

	b.WriteString("## Raw Timeline (append-only)\n\n")
	if len(interactions) == 0 {
		b.WriteString("- No timeline events yet.\n")
	} else {
		for _, item := range interactions {
			cmdLine := strings.TrimSpace(strings.Join(append([]string{item.Command}, item.Args...), " "))
			state := fmt.Sprintf("exit=%d", item.ExitCode)
			if item.InProgress {
				state = "in-progress"
			}
			b.WriteString(fmt.Sprintf("- %s — `%s` (%s, event=%s, files=%d)\n",
				item.Timestamp,
				cmdLine,
				state,
				nonEmpty(item.EventType, "final"),
				len(item.ModifiedFiles),
			))
		}
	}

	generatedPath := filepath.Join(stateDir, "spec.generated.md")
	generated := b.String()
	if err := os.WriteFile(generatedPath, []byte(generated), 0o644); err != nil {
		return 1, err
	}

	fmt.Fprintln(out, generated)
	fmt.Fprintf(out, "Wrote %s\n", generatedPath)
	return 0, nil
}

type lsOptions struct {
	Index    int
	HasIndex bool
	Timeline bool
	All      bool
	Active   bool
	Hidden   bool
	Agent    string
	Since    time.Time
	HasSince bool
	Verbose  bool
}

func runLs(args []string, out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}

	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}

	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	interactions, err := readInteractionTimeline(interactionsPath)
	if err != nil {
		return 1, err
	}

	options, err := parseLsOptions(args)
	if err != nil {
		return 1, err
	}

	hiddenSet, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		return 1, err
	}
	numbers, err := retrofitSessionIndex(stateDir, interactions)
	if err != nil {
		return 1, err
	}
	sessions := filterSessionSummaries(summarizeSessions(interactions, hiddenSet, numbers), options)

	if options.Timeline && !options.HasIndex {
		printTimelineEvents(out, interactions, hiddenSet, options)
		return 0, nil
	}
	if len(sessions) == 0 {
		fmt.Fprintln(out, "No captured sessions found.")
		return 0, nil
	}

	for _, session := range sessions {
		cmdLine := strings.TrimSpace(strings.Join(append([]string{session.Command}, session.Args...), " "))
		state := fmt.Sprintf("exit=%d", session.Latest.ExitCode)
		if session.Latest.InProgress {
			state = "in-progress"
		}
		if options.Verbose {
			line := fmt.Sprintf("%d) %s..%s | %s | %s | interactions=%d | files=%d | hidden=%t",
				session.Number,
				session.FirstTimestamp,
				session.LastTimestamp,
				cmdLine,
				state,
				session.InteractionCount,
				len(session.Latest.ModifiedFiles),
				session.Hidden,
			)
			fmt.Fprintln(out, styleSessionListLine(line, session.Hidden, options))
		} else {
			line := fmt.Sprintf("%d) %s | %s | %s | interactions=%d | files=%d",
				session.Number,
				session.LastTimestamp,
				cmdLine,
				state,
				session.InteractionCount,
				len(session.Latest.ModifiedFiles),
			)
			fmt.Fprintln(out, styleSessionListLine(line, session.Hidden, options))
		}
	}

	if !options.HasIndex {
		fmt.Fprintln(out, "To inspect one session, run `sd ls N` (for example: `sd ls 0`).")
		fmt.Fprintln(out, "For full session output, run `sd cat N`.")
		fmt.Fprintln(out, "To soft-delete one session, run `sd hide N`.")
		fmt.Fprintln(out, "Use `sd ls --hidden` to list hidden sessions.")
		return 0, nil
	}
	selected, ok := findSessionByNumber(sessions, options.Index)
	if !ok {
		return 1, fmt.Errorf("invalid session number %d", options.Index)
	}
	sessionEvents := interactionsForSession(interactions, selected.MatchKey)

	return 0, printSessionAbbreviated(out, repoRoot, selected, sessionEvents)
}

func runCat(args []string, out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}
	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}

	showHidden := false
	if len(args) > 0 && args[0] == "--hidden" {
		showHidden = true
		args = args[1:]
	}
	if len(args) == 0 {
		return 1, fmt.Errorf("missing session number; use `sd cat N`")
	}
	n, parseErr := strconv.Atoi(args[0])
	if parseErr != nil {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}

	interactions, err := readInteractionTimeline(filepath.Join(stateDir, "interactions.ndjson"))
	if err != nil {
		return 1, err
	}
	hiddenSet, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		return 1, err
	}
	numbers, err := retrofitSessionIndex(stateDir, interactions)
	if err != nil {
		return 1, err
	}
	sessions := summarizeSessions(interactions, hiddenSet, numbers)
	if showHidden {
		sessions = filterSessionSummaries(sessions, lsOptions{Hidden: true})
	} else {
		sessions = filterSessionSummaries(sessions, lsOptions{})
	}
	selected, ok := findSessionByNumber(sessions, n)
	if !ok {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}
	return 0, printSessionDetail(out, repoRoot, selected.Latest, selected.Number)
}

func runHide(args []string, out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}
	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}

	if len(args) == 0 {
		return 1, fmt.Errorf("missing session number; use `sd hide N`")
	}
	n, parseErr := strconv.Atoi(args[0])
	if parseErr != nil {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}

	interactions, err := readInteractionTimeline(filepath.Join(stateDir, "interactions.ndjson"))
	if err != nil {
		return 1, err
	}
	hiddenSet, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		return 1, err
	}
	numbers, err := retrofitSessionIndex(stateDir, interactions)
	if err != nil {
		return 1, err
	}
	sessions := filterSessionSummaries(summarizeSessions(interactions, hiddenSet, numbers), lsOptions{})
	target, ok := findSessionByNumber(sessions, n)
	if !ok {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}
	hiddenSet[target.MatchKey] = struct{}{}
	if err := writeHiddenSessionIDs(stateDir, hiddenSet); err != nil {
		return 1, err
	}
	_ = appendInteraction(filepath.Join(stateDir, "interactions.ndjson"), interaction{
		SchemaVersion: interactionSchemaVersion,
		EventType:     eventTypeHide,
		SessionID:     target.MatchKey,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Command:       "sd",
		Args:          []string{"hide", strconv.Itoa(n)},
		ExitCode:      0,
	})
	fmt.Fprintf(out, "Hidden session %d (%s)\n", n, target.MatchKey)
	return 0, nil
}

func runUnhide(args []string, out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}
	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}
	if len(args) == 0 {
		return 1, fmt.Errorf("missing session number; use `sd unhide N` from `sd ls --hidden`")
	}
	n, parseErr := strconv.Atoi(args[0])
	if parseErr != nil {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}

	interactions, err := readInteractionTimeline(filepath.Join(stateDir, "interactions.ndjson"))
	if err != nil {
		return 1, err
	}
	hiddenSet, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		return 1, err
	}
	numbers, err := retrofitSessionIndex(stateDir, interactions)
	if err != nil {
		return 1, err
	}
	sessions := filterSessionSummaries(summarizeSessions(interactions, hiddenSet, numbers), lsOptions{Hidden: true})
	target, ok := findSessionByNumber(sessions, n)
	if !ok {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}
	delete(hiddenSet, target.MatchKey)
	if err := writeHiddenSessionIDs(stateDir, hiddenSet); err != nil {
		return 1, err
	}
	_ = appendInteraction(filepath.Join(stateDir, "interactions.ndjson"), interaction{
		SchemaVersion: interactionSchemaVersion,
		EventType:     eventTypeUnhide,
		SessionID:     target.MatchKey,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Command:       "sd",
		Args:          []string{"unhide", strconv.Itoa(n)},
		ExitCode:      0,
	})
	fmt.Fprintf(out, "Unhid session %d (%s)\n", n, target.MatchKey)
	return 0, nil
}

func runRm(args []string, out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}
	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}

	includeHidden := false
	if len(args) > 0 && args[0] == "--hidden" {
		includeHidden = true
		args = args[1:]
	}
	if len(args) == 0 {
		return 1, fmt.Errorf("missing session number; use `sd rm N`")
	}
	n, parseErr := strconv.Atoi(args[0])
	if parseErr != nil {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}

	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	interactions, err := readInteractionTimeline(interactionsPath)
	if err != nil {
		return 1, err
	}
	hiddenSet, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		return 1, err
	}
	numbers, err := retrofitSessionIndex(stateDir, interactions)
	if err != nil {
		return 1, err
	}
	sessions := summarizeSessions(interactions, hiddenSet, numbers)
	if includeHidden {
		sessions = filterSessionSummaries(sessions, lsOptions{Hidden: true})
	} else {
		sessions = filterSessionSummaries(sessions, lsOptions{})
	}
	target, ok := findSessionByNumber(sessions, n)
	if !ok {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}
	removedInteractions, removedFiles, err := hardDeleteSession(repoRoot, stateDir, target.MatchKey, interactions, hiddenSet)
	if err != nil {
		return 1, err
	}
	_ = appendInteraction(interactionsPath, interaction{
		SchemaVersion: interactionSchemaVersion,
		EventType:     eventTypeRemove,
		SessionID:     target.MatchKey,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Command:       "sd",
		Args:          []string{"rm", strconv.Itoa(n)},
		ExitCode:      0,
	})
	fmt.Fprintf(out, "Removed session %d (%s): interactions=%d logs=%d\n", n, target.MatchKey, removedInteractions, removedFiles)
	return 0, nil
}

func runPrune(out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}
	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}
	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	interactions, err := readInteractionTimeline(interactionsPath)
	if err != nil {
		return 1, err
	}
	hiddenSet, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		return 1, err
	}

	totalInteractions := 0
	totalFiles := 0
	for sessionKey := range hiddenSet {
		removedInteractions, removedFiles, deleteErr := hardDeleteSession(repoRoot, stateDir, sessionKey, interactions, hiddenSet)
		if deleteErr != nil {
			return 1, deleteErr
		}
		totalInteractions += removedInteractions
		totalFiles += removedFiles
		interactions, err = readInteractionTimeline(interactionsPath)
		if err != nil {
			return 1, err
		}
	}

	removedOrphans, err := removeOrphanSessionLogs(repoRoot, stateDir, interactions)
	if err != nil {
		return 1, err
	}
	totalFiles += removedOrphans
	fmt.Fprintf(out, "Pruned hidden sessions: interactions=%d logs=%d\n", totalInteractions, totalFiles)
	return 0, nil
}

func runInputs(args []string, out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}
	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}

	showAll := false
	if len(args) > 0 {
		switch strings.TrimSpace(args[0]) {
		case "--all", "-a":
			showAll = true
		default:
			return 1, fmt.Errorf("unknown inputs argument %q", args[0])
		}
	}

	interactions, err := readInteractionTimeline(filepath.Join(stateDir, "interactions.ndjson"))
	if err != nil {
		return 1, err
	}
	hiddenSet, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		return 1, err
	}

	entries := collectInputHistoryEntries(repoRoot, interactions, hiddenSet, showAll)
	if len(entries) == 0 {
		fmt.Fprintln(out, "No captured inputs found.")
		return 0, nil
	}
	printInputHistory(out, entries)
	return 0, nil
}

func runGet(args []string, out io.Writer) (int, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, err
	}
	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}
	if len(args) == 0 {
		return 1, fmt.Errorf("missing session number; use `sd get N`")
	}
	n, parseErr := strconv.Atoi(args[0])
	if parseErr != nil {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}

	interactions, err := readInteractionTimeline(filepath.Join(stateDir, "interactions.ndjson"))
	if err != nil {
		return 1, err
	}
	hiddenSet, err := readHiddenSessionIDs(stateDir)
	if err != nil {
		return 1, err
	}
	numbers, err := retrofitSessionIndex(stateDir, interactions)
	if err != nil {
		return 1, err
	}
	sessions := filterSessionSummaries(summarizeSessions(interactions, hiddenSet, numbers), lsOptions{})
	session, ok := findSessionByNumber(sessions, n)
	if !ok {
		return 1, fmt.Errorf("invalid session number %q", args[0])
	}
	messages, readErr := readConversationLog(filepath.Join(repoRoot, filepath.FromSlash(session.Latest.ConversationLog)))
	if readErr != nil {
		return 1, readErr
	}
	var lines []string
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		line := strings.TrimSpace(msg.Text)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	cleaned := strings.Join(lines, "\n")
	if strings.TrimSpace(cleaned) == "" {
		fmt.Fprintln(out, "[no printable input captured]")
		return 0, nil
	}
	fmt.Fprint(out, cleaned)
	if cleaned[len(cleaned)-1] != '\n' {
		fmt.Fprintln(out)
	}
	return 0, nil
}

func runAgent(command string, args []string, errOut io.Writer) (int, error) {
	resolvedCommand, err := exec.LookPath(command)
	if err != nil {
		return 1, fmt.Errorf("cannot find agent binary %q in PATH", command)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return 1, fmt.Errorf("wrapped sessions require a git workspace: %w", err)
	}

	_, stateDir, err := ensureState(repoRoot)
	if err != nil {
		return 1, err
	}

	sessionID := fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102T150405Z"), sanitizeFileSegment(command))
	if _, err := ensureSessionNumber(stateDir, sessionID); err != nil {
		return 1, err
	}
	sessionDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return 1, err
	}
	conversationRel := filepath.ToSlash(filepath.Join(stateDirName, "sessions", sessionID+".conversation.json"))
	conversationPath := filepath.Join(repoRoot, filepath.FromSlash(conversationRel))
	stdinCapture := &lockedBuffer{}
	stdoutCapture := &lockedBuffer{}

	cmd := exec.Command(resolvedCommand, args...)
	cmd.Env = os.Environ()

	before, beforeErr := snapshotRepoStatus(repoRoot)
	if beforeErr != nil {
		fmt.Fprintf(errOut, "sd: wrapper warning: failed to snapshot pre-session file status: %v\n", beforeErr)
	}

	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	appendEvent := func(eventType string, exitCode int, inProgress bool, modified []string, inputPreview string) {
		_ = appendInteraction(interactionsPath, interaction{
			SchemaVersion:   interactionSchemaVersion,
			EventType:       eventType,
			SessionID:       sessionID,
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
			Command:         command,
			Args:            args,
			ExitCode:        exitCode,
			InProgress:      inProgress,
			InputPreview:    inputPreview,
			ConversationLog: conversationRel,
			ModifiedFiles:   modified,
		})
	}

	appendEvent(eventTypeStart, -1, true, nil, "")

	var finalizeMu sync.Mutex
	finalized := false
	finalizeTerminated := func(exitCode int) {
		finalizeMu.Lock()
		defer finalizeMu.Unlock()
		if finalized {
			return
		}
		finalized = true
		appendEvent(eventTypeTerminate, exitCode, false, nil, buildInputPreviewFromRaw(stdinCapture.Bytes()))
	}

	sigDone := make(chan struct{})
	stopSig := make(chan struct{})
	termSignals := make(chan os.Signal, 1)
	signal.Notify(termSignals, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		defer close(sigDone)
		select {
		case sig := <-termSignals:
			exitCode := 143
			if sig == syscall.SIGHUP {
				exitCode = 129
			}
			finalizeTerminated(exitCode)
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		case <-stopSig:
			return
		}
	}()

	var watcherStop chan struct{}
	var watcherDone chan struct{}
	if beforeErr == nil {
		watcherStop = make(chan struct{})
		watcherDone = make(chan struct{})
		go func() {
			defer close(watcherDone)
			ticker := time.NewTicker(incrementalPollInterval)
			defer ticker.Stop()

			lastRecorded := ""
			lastRecordedAt := time.Time{}
			pendingKey := ""
			pendingSince := time.Time{}
			pendingFiles := []string{}
			for {
				select {
				case <-watcherStop:
					return
				case <-ticker.C:
					after, err := snapshotRepoStatus(repoRoot)
					if err != nil {
						continue
					}
					modified := filterIncrementalFiles(changedFilesBetween(before, after))
					if len(modified) == 0 {
						pendingKey = ""
						pendingSince = time.Time{}
						pendingFiles = nil
						continue
					}

					key := strings.Join(modified, "\n")
					now := time.Now()
					if key != pendingKey {
						pendingKey = key
						pendingSince = now
						pendingFiles = modified
						continue
					}
					if now.Sub(pendingSince) < incrementalDebounce {
						continue
					}
					if !lastRecordedAt.IsZero() && now.Sub(lastRecordedAt) < incrementalMinInterval {
						continue
					}
					if key == lastRecorded {
						continue
					}

					lastRecorded = key
					lastRecordedAt = now
					inputPreview := buildInputPreviewFromRaw(stdinCapture.Bytes())
					appendEvent(eventTypeUpdate, -1, true, pendingFiles, inputPreview)
				}
			}
		}()
	}

	exitCode, execErr := runInteractive(cmd, stdinCapture, stdoutCapture)
	close(stopSig)
	signal.Stop(termSignals)
	<-sigDone
	if execErr != nil && !errors.Is(execErr, io.EOF) {
		fmt.Fprintf(errOut, "sd: wrapper warning: %v\n", execErr)
	}
	if watcherStop != nil {
		close(watcherStop)
		<-watcherDone
	}

	after, afterErr := snapshotRepoStatus(repoRoot)
	if afterErr != nil {
		fmt.Fprintf(errOut, "sd: wrapper warning: failed to snapshot post-session file status: %v\n", afterErr)
	}

	modified := []string{}
	if beforeErr == nil && afterErr == nil {
		modified = filterIncrementalFiles(changedFilesBetween(before, after))
	}

	conversationMessages := buildConversationMessages(stdinCapture.Bytes(), stdoutCapture.Bytes())
	if writeErr := writeConversationLog(conversationPath, conversationMessages); writeErr != nil {
		fmt.Fprintf(errOut, "sd: wrapper warning: failed to write conversation log: %v\n", writeErr)
	}
	finalPreview := buildInputPreviewFromConversation(conversationMessages)
	appendEvent(eventTypeFinal, exitCode, false, modified, finalPreview)

	return exitCode, nil
}

func runInteractive(cmd *exec.Cmd, stdinLog io.Writer, stdoutLog io.Writer) (int, error) {
	stdin := os.Stdin
	stdout := os.Stdout
	stderr := os.Stderr
	displayWriter := newOSCTerminalFilterWriter(stdout)

	if stdinLog == nil {
		stdinLog = io.Discard
	}
	if stdoutLog == nil {
		stdoutLog = io.Discard
	}

	if term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd())) {
		size, _ := pty.GetsizeFull(stdin)
		ptmx, err := pty.StartWithSize(cmd, size)
		if err != nil {
			return 1, err
		}
		defer ptmx.Close()

		_ = pty.InheritSize(stdin, ptmx)
		winch := make(chan os.Signal, 1)
		resizeDone := make(chan struct{})
		signal.Notify(winch, syscall.SIGWINCH)
		defer func() {
			signal.Stop(winch)
			close(resizeDone)
		}()
		go func() {
			for {
				select {
				case <-resizeDone:
					return
				case <-winch:
					_ = pty.InheritSize(stdin, ptmx)
				}
			}
		}()

		oldState, rawErr := term.MakeRaw(int(stdin.Fd()))
		if rawErr == nil {
			defer term.Restore(int(stdin.Fd()), oldState)
		}

		outDone := make(chan error, 1)
		go func() {
			_, copyErr := io.Copy(io.MultiWriter(displayWriter, stdoutLog), ptmx)
			outDone <- copyErr
		}()

		go func() {
			_, _ = io.Copy(ptmx, io.TeeReader(stdin, stdinLog))
		}()

		waitErr := cmd.Wait()
		_ = ptmx.Close()
		<-outDone
		return exitCodeFromWait(waitErr), nil
	}

	cmd.Stdin = io.TeeReader(stdin, stdinLog)
	cmd.Stdout = io.MultiWriter(displayWriter, stdoutLog)
	cmd.Stderr = io.MultiWriter(newOSCTerminalFilterWriter(stderr), stdoutLog)
	waitErr := cmd.Run()
	return exitCodeFromWait(waitErr), nil
}

func exitCodeFromWait(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func ensureState(repoRoot string) (config, string, error) {
	stateDir := filepath.Join(repoRoot, stateDirName)
	sessionDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return config{}, "", err
	}

	cfgPath := filepath.Join(stateDir, "config.json")
	cfg := config{
		Version:      1,
		SpecPointers: collectSpecPointers(repoRoot),
	}

	if existingCfgBytes, err := os.ReadFile(cfgPath); err == nil {
		var existing config
		if json.Unmarshal(existingCfgBytes, &existing) == nil {
			if existing.Version > 0 {
				cfg.Version = existing.Version
			}
			if len(existing.SpecPointers) > 0 {
				cfg.SpecPointers = dedupeSorted(existing.SpecPointers)
			}
		}
	}

	if len(cfg.SpecPointers) == 0 {
		cfg.SpecPointers = []string{"SPEC.md"}
	}

	cfgBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return config{}, "", err
	}
	if err := os.WriteFile(cfgPath, cfgBytes, 0o644); err != nil {
		return config{}, "", err
	}

	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	if _, err := os.Stat(interactionsPath); errors.Is(err, fs.ErrNotExist) {
		if err := os.WriteFile(interactionsPath, []byte(""), 0o644); err != nil {
			return config{}, "", err
		}
	}
	interactions, err := readInteractionTimeline(interactionsPath)
	if err != nil {
		return config{}, "", err
	}
	if _, err := retrofitSessionIndex(stateDir, interactions); err != nil {
		return config{}, "", err
	}
	if err := migrateConversationLogs(repoRoot, interactionsPath, interactions); err != nil {
		return config{}, "", err
	}

	if err := bootstrap.Extract(stateDir); err != nil {
		return config{}, "", err
	}

	return cfg, stateDir, nil
}

func collectSpecPointers(repoRoot string) []string {
	var pointers []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == stateDirName || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "SPEC.md" {
			rel, relErr := filepath.Rel(repoRoot, path)
			if relErr == nil {
				pointers = append(pointers, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		return []string{"SPEC.md"}
	}
	return dedupeSorted(pointers)
}

func dedupeSorted(items []string) []string {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for item := range set {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func appendInteraction(path string, item interaction) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return err
	}
	return nil
}

func readInteractionTimeline(path string) ([]interaction, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var out []interaction
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item interaction
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		if item.SchemaVersion == 0 {
			item.SchemaVersion = 1
		}
		if strings.TrimSpace(item.EventType) == "" {
			if item.InProgress || item.ExitCode < 0 {
				item.EventType = eventTypeUpdate
			} else {
				item.EventType = eventTypeFinal
			}
		}
		out = append(out, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func readInteractions(path string) ([]interaction, error) {
	timeline, err := readInteractionTimeline(path)
	if err != nil {
		return nil, err
	}
	summaries := summarizeSessions(timeline, map[string]struct{}{}, map[string]int{})
	out := make([]interaction, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, summary.Latest)
	}
	return out, nil
}

func orderedInputRecords(interactions []interaction) []interaction {
	seen := map[string]struct{}{}
	out := make([]interaction, 0, len(interactions))
	for _, item := range interactions {
		if isSessionMetaEvent(item.EventType) {
			continue
		}
		key := strings.TrimSpace(item.SessionID)
		if key == "" {
			key = strings.TrimSpace(item.ConversationLog)
		}
		if key == "" {
			key = strings.TrimSpace(item.StdinLog)
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func interactionsForSession(interactions []interaction, sessionKey string) []interaction {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	out := make([]interaction, 0, len(interactions))
	for _, item := range interactions {
		if isSessionMetaEvent(item.EventType) {
			continue
		}
		if item.EventType == eventTypeStart {
			continue
		}
		if interactionSessionKey(item) == sessionKey {
			out = append(out, item)
		}
	}
	return out
}

func summarizeSessions(interactions []interaction, hidden map[string]struct{}, numbers map[string]int) []sessionSummary {
	indexBySessionID := map[string]int{}
	summaries := make([]sessionSummary, 0)
	for _, item := range interactions {
		if isSessionMetaEvent(item.EventType) {
			continue
		}
		sessionKey := interactionSessionKey(item)
		if sessionKey == "" {
			continue
		}
		_, isHidden := hidden[sessionKey]
		if idx, exists := indexBySessionID[sessionKey]; exists {
			summaries[idx].InteractionCount++
			summaries[idx].LastTimestamp = item.Timestamp
			summaries[idx].Latest = item
			continue
		}
		displayID := strings.TrimSpace(item.SessionID)
		if displayID == "" {
			displayID = "(legacy)"
		}
		indexBySessionID[sessionKey] = len(summaries)
		summaries = append(summaries, sessionSummary{
			Number:           sessionNumberForKey(numbers, sessionKey),
			MatchKey:         sessionKey,
			SessionID:        displayID,
			Hidden:           isHidden,
			FirstTimestamp:   item.Timestamp,
			LastTimestamp:    item.Timestamp,
			Command:          item.Command,
			Args:             append([]string(nil), item.Args...),
			InteractionCount: 1,
			Latest:           item,
		})
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].Number != summaries[j].Number {
			return summaries[i].Number < summaries[j].Number
		}
		return summaries[i].FirstTimestamp < summaries[j].FirstTimestamp
	})
	return summaries
}

func sessionNumberForKey(numbers map[string]int, key string) int {
	if n, ok := numbers[key]; ok {
		return n
	}
	return -1
}

func findSessionByNumber(sessions []sessionSummary, number int) (sessionSummary, bool) {
	for _, session := range sessions {
		if session.Number == number {
			return session, true
		}
	}
	return sessionSummary{}, false
}

func interactionSessionKey(item interaction) string {
	sessionID := strings.TrimSpace(item.SessionID)
	if sessionID != "" {
		return sessionID
	}
	if convo := strings.TrimSpace(item.ConversationLog); convo != "" {
		return "conversation:" + convo
	}
	if stdin := strings.TrimSpace(item.StdinLog); stdin != "" {
		return "legacy:" + stdin
	}
	return strings.TrimSpace(item.Timestamp + "|" + item.Command)
}

func isSessionMetaEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case eventTypeHide, eventTypeUnhide, eventTypeRemove:
		return true
	default:
		return false
	}
}

func parseLsOptions(args []string) (lsOptions, error) {
	var options lsOptions
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		switch {
		case arg == "--all" || arg == "-a":
			options.All = true
		case arg == "--timeline" || arg == "-t":
			options.Timeline = true
		case arg == "--active":
			options.Active = true
		case arg == "--hidden":
			options.Hidden = true
		case arg == "--verbose" || arg == "-v":
			options.Verbose = true
		case arg == "--compact":
			options.Verbose = false
		case arg == "--agent":
			if i+1 >= len(args) {
				return options, errors.New("missing value for --agent")
			}
			i++
			options.Agent = strings.TrimSpace(args[i])
		case strings.HasPrefix(arg, "--agent="):
			options.Agent = strings.TrimSpace(strings.TrimPrefix(arg, "--agent="))
		case arg == "--since":
			if i+1 >= len(args) {
				return options, errors.New("missing value for --since")
			}
			i++
			parsed, err := parseSince(args[i])
			if err != nil {
				return options, err
			}
			options.Since = parsed
			options.HasSince = true
		case strings.HasPrefix(arg, "--since="):
			parsed, err := parseSince(strings.TrimPrefix(arg, "--since="))
			if err != nil {
				return options, err
			}
			options.Since = parsed
			options.HasSince = true
		default:
			n, err := strconv.Atoi(arg)
			if err == nil {
				if options.HasIndex {
					return options, fmt.Errorf("multiple session indexes provided")
				}
				options.Index = n
				options.HasIndex = true
				continue
			}
			return options, fmt.Errorf("unknown ls argument %q", arg)
		}
	}
	return options, nil
}

func parseSince(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("empty --since value")
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since value %q (use RFC3339 or YYYY-MM-DD)", raw)
}

func filterSessionSummaries(sessions []sessionSummary, options lsOptions) []sessionSummary {
	out := make([]sessionSummary, 0, len(sessions))
	for _, session := range sessions {
		if !options.All {
			if options.Hidden {
				if !session.Hidden {
					continue
				}
			} else {
				if session.Hidden {
					continue
				}
			}
		}
		if options.Active && !session.Latest.InProgress {
			continue
		}
		if options.Agent != "" && session.Command != options.Agent {
			continue
		}
		if options.HasSince {
			last, err := time.Parse(time.RFC3339, session.LastTimestamp)
			if err != nil || last.Before(options.Since) {
				continue
			}
		}
		out = append(out, session)
	}
	return out
}

func printTimelineEvents(out io.Writer, interactions []interaction, hidden map[string]struct{}, options lsOptions) {
	index := 1
	for _, item := range interactions {
		if isSessionMetaEvent(item.EventType) {
			continue
		}
		key := interactionSessionKey(item)
		_, isHidden := hidden[key]
		if !options.All {
			if options.Hidden {
				if !isHidden {
					continue
				}
			} else if isHidden {
				continue
			}
		}
		if options.Active && !item.InProgress {
			continue
		}
		if options.Agent != "" && item.Command != options.Agent {
			continue
		}
		if options.HasSince {
			eventTime, err := time.Parse(time.RFC3339, item.Timestamp)
			if err != nil || eventTime.Before(options.Since) {
				continue
			}
		}
		preview := abbreviatePreview(item.InputPreview, 100)
		if preview == "" {
			preview = "[no input preview]"
		}
		state := fmt.Sprintf("exit=%d", item.ExitCode)
		if item.InProgress {
			state = "in-progress"
		}
		fmt.Fprintf(out, "%d) %s | %s %s | %s | %s\n", index, item.Timestamp, item.Command, strings.Join(item.Args, " "), state, preview)
		index++
	}
	if index == 1 {
		fmt.Fprintln(out, "No timeline events found.")
	}
}

func styleSessionListLine(line string, hidden bool, options lsOptions) string {
	if hidden && options.All {
		return "\x1b[90m" + line + "\x1b[0m"
	}
	return line
}

func hiddenSessionsPath(stateDir string) string {
	return filepath.Join(stateDir, "sessions.hidden.json")
}

func sessionIndexPath(stateDir string) string {
	return filepath.Join(stateDir, "session_index.json")
}

func readSessionIndex(stateDir string) (sessionIndex, error) {
	path := sessionIndexPath(stateDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return sessionIndex{Entries: map[string]int{}}, nil
		}
		return sessionIndex{}, err
	}
	var idx sessionIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return sessionIndex{}, err
	}
	if idx.Entries == nil {
		idx.Entries = map[string]int{}
	}
	max := -1
	for _, n := range idx.Entries {
		if n > max {
			max = n
		}
	}
	if idx.NextID <= max {
		idx.NextID = max + 1
	}
	if idx.NextID < 0 {
		idx.NextID = 0
	}
	return idx, nil
}

func writeSessionIndex(stateDir string, idx sessionIndex) error {
	if idx.Entries == nil {
		idx.Entries = map[string]int{}
	}
	raw, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionIndexPath(stateDir), raw, 0o644)
}

func retrofitSessionIndex(stateDir string, interactions []interaction) (map[string]int, error) {
	idx, err := readSessionIndex(stateDir)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, item := range interactions {
		key := interactionSessionKey(item)
		if strings.TrimSpace(key) == "" {
			continue
		}
		if _, exists := idx.Entries[key]; exists {
			continue
		}
		idx.Entries[key] = idx.NextID
		idx.NextID++
		changed = true
	}
	if changed {
		if err := writeSessionIndex(stateDir, idx); err != nil {
			return nil, err
		}
	}
	return idx.Entries, nil
}

func ensureSessionNumber(stateDir, sessionKey string) (int, error) {
	idx, err := readSessionIndex(stateDir)
	if err != nil {
		return -1, err
	}
	if n, exists := idx.Entries[sessionKey]; exists {
		return n, nil
	}
	n := idx.NextID
	idx.Entries[sessionKey] = n
	idx.NextID++
	if err := writeSessionIndex(stateDir, idx); err != nil {
		return -1, err
	}
	return n, nil
}

func readHiddenSessionIDs(stateDir string) (map[string]struct{}, error) {
	path := hiddenSessionsPath(stateDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}
	var payload hiddenSessions
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(payload.HiddenSessionIDs))
	for _, sessionID := range payload.HiddenSessionIDs {
		sessionID = strings.TrimSpace(sessionID)
		if sessionID == "" {
			continue
		}
		set[sessionID] = struct{}{}
	}
	return set, nil
}

func writeHiddenSessionIDs(stateDir string, hidden map[string]struct{}) error {
	ids := make([]string, 0, len(hidden))
	for sessionID := range hidden {
		sessionID = strings.TrimSpace(sessionID)
		if sessionID == "" {
			continue
		}
		ids = append(ids, sessionID)
	}
	sort.Strings(ids)
	payload := hiddenSessions{HiddenSessionIDs: ids}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(hiddenSessionsPath(stateDir), raw, 0o644)
}

func conversationLogRelForInteraction(item interaction) string {
	if strings.TrimSpace(item.ConversationLog) != "" {
		return filepath.ToSlash(strings.TrimSpace(item.ConversationLog))
	}
	if sid := strings.TrimSpace(item.SessionID); sid != "" {
		return filepath.ToSlash(filepath.Join(stateDirName, "sessions", sanitizeFileSegment(sid)+".conversation.json"))
	}
	if stdin := strings.TrimSpace(item.StdinLog); stdin != "" {
		base := filepath.Base(stdin)
		base = strings.TrimSuffix(base, ".stdin.log")
		if base == "" {
			base = sanitizeFileSegment(stdin)
		}
		return filepath.ToSlash(filepath.Join(stateDirName, "sessions", base+".conversation.json"))
	}
	fallback := sanitizeFileSegment(item.Timestamp + "-" + item.Command)
	return filepath.ToSlash(filepath.Join(stateDirName, "sessions", fallback+".conversation.json"))
}

func migrateConversationLogs(repoRoot, interactionsPath string, interactions []interaction) error {
	if len(interactions) == 0 {
		return nil
	}
	changed := false
	created := map[string]struct{}{}
	for idx := range interactions {
		if isSessionMetaEvent(interactions[idx].EventType) {
			continue
		}
		rel := conversationLogRelForInteraction(interactions[idx])
		interactions[idx].ConversationLog = rel
		oldStdin := strings.TrimSpace(interactions[idx].StdinLog)
		oldStdout := strings.TrimSpace(interactions[idx].StdoutLog)
		interactions[idx].StdinLog = ""
		interactions[idx].StdoutLog = ""
		abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
		if _, ok := created[rel]; ok {
			changed = true
			if oldStdin != "" {
				_ = os.Remove(filepath.Join(repoRoot, filepath.FromSlash(oldStdin)))
			}
			if oldStdout != "" {
				_ = os.Remove(filepath.Join(repoRoot, filepath.FromSlash(oldStdout)))
			}
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			if err := ensureConversationLogHasDT(abs); err != nil {
				return err
			}
			created[rel] = struct{}{}
			changed = true
			if oldStdin != "" {
				_ = os.Remove(filepath.Join(repoRoot, filepath.FromSlash(oldStdin)))
			}
			if oldStdout != "" {
				_ = os.Remove(filepath.Join(repoRoot, filepath.FromSlash(oldStdout)))
			}
			continue
		}
		stdinRaw := []byte{}
		stdoutRaw := []byte{}
		if oldStdin != "" {
			if raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(oldStdin))); err == nil {
				stdinRaw = raw
			}
		}
		if oldStdout != "" {
			if raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(oldStdout))); err == nil {
				stdoutRaw = raw
			}
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		if err := writeConversationLog(abs, buildConversationMessages(stdinRaw, stdoutRaw)); err != nil {
			return err
		}
		if oldStdin != "" {
			_ = os.Remove(filepath.Join(repoRoot, filepath.FromSlash(oldStdin)))
		}
		if oldStdout != "" {
			_ = os.Remove(filepath.Join(repoRoot, filepath.FromSlash(oldStdout)))
		}
		created[rel] = struct{}{}
		changed = true
	}
	if changed {
		return writeInteractionTimeline(interactionsPath, interactions)
	}
	return nil
}

func writeInteractionTimeline(path string, interactions []interaction) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, item := range interactions {
		raw, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(raw, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func hardDeleteSession(repoRoot, stateDir, sessionKey string, interactions []interaction, hidden map[string]struct{}) (int, int, error) {
	interactionsPath := filepath.Join(stateDir, "interactions.ndjson")
	keep := make([]interaction, 0, len(interactions))
	logsToDelete := map[string]struct{}{}
	removedInteractions := 0
	for _, item := range interactions {
		if interactionSessionKey(item) == sessionKey {
			removedInteractions++
			if strings.TrimSpace(item.ConversationLog) != "" {
				logsToDelete[item.ConversationLog] = struct{}{}
			}
			if strings.TrimSpace(item.StdinLog) != "" {
				logsToDelete[item.StdinLog] = struct{}{}
			}
			if strings.TrimSpace(item.StdoutLog) != "" {
				logsToDelete[item.StdoutLog] = struct{}{}
			}
			continue
		}
		keep = append(keep, item)
	}
	if err := writeInteractionTimeline(interactionsPath, keep); err != nil {
		return 0, 0, err
	}
	delete(hidden, sessionKey)
	if err := writeHiddenSessionIDs(stateDir, hidden); err != nil {
		return 0, 0, err
	}

	referencedLogs := map[string]struct{}{}
	for _, item := range keep {
		if item.ConversationLog != "" {
			referencedLogs[item.ConversationLog] = struct{}{}
		}
		if item.StdinLog != "" {
			referencedLogs[item.StdinLog] = struct{}{}
		}
		if item.StdoutLog != "" {
			referencedLogs[item.StdoutLog] = struct{}{}
		}
	}

	removedFiles := 0
	for rel := range logsToDelete {
		if _, stillReferenced := referencedLogs[rel]; stillReferenced {
			continue
		}
		path := filepath.Join(repoRoot, filepath.FromSlash(rel))
		if err := os.Remove(path); err == nil {
			removedFiles++
		} else if !errors.Is(err, fs.ErrNotExist) {
			return 0, 0, err
		}
	}
	return removedInteractions, removedFiles, nil
}

func removeOrphanSessionLogs(repoRoot, stateDir string, interactions []interaction) (int, error) {
	referenced := map[string]struct{}{}
	for _, item := range interactions {
		if strings.TrimSpace(item.ConversationLog) != "" {
			referenced[item.ConversationLog] = struct{}{}
		}
		if strings.TrimSpace(item.StdinLog) != "" {
			referenced[item.StdinLog] = struct{}{}
		}
		if strings.TrimSpace(item.StdoutLog) != "" {
			referenced[item.StdoutLog] = struct{}{}
		}
	}
	sessionDir := filepath.Join(stateDir, "sessions")
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(stateDirName, "sessions", entry.Name()))
		if _, ok := referenced[rel]; ok {
			continue
		}
		path := filepath.Join(sessionDir, entry.Name())
		if err := os.Remove(path); err == nil {
			removed++
		} else if !errors.Is(err, fs.ErrNotExist) {
			return 0, err
		}
	}
	// Remove empty nested directories created by legacy paths.
	_ = filepath.WalkDir(sessionDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == sessionDir {
			return nil
		}
		childEntries, readErr := os.ReadDir(path)
		if readErr == nil && len(childEntries) == 0 {
			_ = os.Remove(path)
		}
		return nil
	})
	return removed, nil
}

func printSessionAbbreviated(out io.Writer, repoRoot string, summary sessionSummary, events []interaction) error {
	cmdLine := strings.TrimSpace(strings.Join(append([]string{summary.Command}, summary.Args...), " "))
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Number: %d\n", summary.Number)
	fmt.Fprintf(out, "Session: %s\n", summary.SessionID)
	fmt.Fprintf(out, "Command: %s\n", cmdLine)
	fmt.Fprintf(out, "Interactions: %d\n", summary.InteractionCount)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Abbreviated interactions:")
	if len(events) == 0 {
		fmt.Fprintln(out, "- none")
		return nil
	}
	for idx, item := range events {
		preview := strings.TrimSpace(item.InputPreview)
		if preview == "" {
			preview = buildInputPreview(repoRoot, item.StdinLog)
		}
		preview = abbreviatePreview(preview, 100)
		if preview == "" {
			preview = "[no input preview]"
		}
		if idx > 0 {
			fmt.Fprintln(out, "...")
		}
		fmt.Fprintf(out, "%d) %s | %s\n", idx+1, item.Timestamp, preview)
	}
	return nil
}

func printSessionDetail(out io.Writer, repoRoot string, item interaction, number int) error {
	cmdLine := strings.TrimSpace(strings.Join(append([]string{item.Command}, item.Args...), " "))
	fmt.Fprintln(out, "")
	if number >= 0 {
		fmt.Fprintf(out, "Number:  %d\n", number)
	}
	fmt.Fprintf(out, "Session: %s\n", nonEmpty(item.SessionID, "(legacy)"))
	fmt.Fprintf(out, "When:    %s\n", item.Timestamp)
	fmt.Fprintf(out, "Command: %s\n", cmdLine)
	if item.InProgress {
		fmt.Fprintln(out, "State:   in progress")
	} else {
		fmt.Fprintf(out, "Exit:    %d\n", item.ExitCode)
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Files modified:")
	if len(item.ModifiedFiles) == 0 {
		fmt.Fprintln(out, "- none recorded")
	} else {
		for _, file := range item.ModifiedFiles {
			fmt.Fprintf(out, "- %s\n", file)
		}
	}
	fmt.Fprintln(out, "")

	fmt.Fprintf(out, "Conversation log: %s\n\n", nonEmpty(item.ConversationLog, "(none)"))
	if strings.TrimSpace(item.ConversationLog) != "" {
		messages, err := readConversationLog(filepath.Join(repoRoot, filepath.FromSlash(item.ConversationLog)))
		if err == nil {
			fmt.Fprintln(out, "=== CONVERSATION ===")
			if len(messages) == 0 {
				fmt.Fprintln(out, "[no conversation messages]")
			} else {
				for _, msg := range messages {
					role := strings.TrimSpace(msg.Role)
					switch role {
					case "user":
						fmt.Fprintf(out, "User: %s\n", msg.Text)
					case "assistant":
						fmt.Fprintf(out, "Assistant: %s\n", msg.Text)
					default:
						fmt.Fprintf(out, "%s: %s\n", role, msg.Text)
					}
				}
			}
			return nil
		}
		fmt.Fprintf(out, "[unavailable conversation log] %v\n\n", err)
	}

	// Legacy fallback for pre-migration sessions.
	stdinRaw, stdinErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(item.StdinLog)))
	stdoutRaw, stdoutErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(item.StdoutLog)))
	fmt.Fprintf(out, "Raw logs: %s | %s\n\n", item.StdinLog, item.StdoutLog)
	fmt.Fprintln(out, "=== INPUT (cleaned) ===")
	cleanedInput := ""
	if stdinErr == nil {
		cleanedInput = sanitizeInputLog(stdinRaw)
	}
	if strings.TrimSpace(cleanedInput) == "" {
		fmt.Fprintln(out, "[no printable input captured]")
	} else {
		fmt.Fprint(out, cleanedInput)
		if cleanedInput[len(cleanedInput)-1] != '\n' {
			fmt.Fprintln(out)
		}
	}
	fmt.Fprintln(out, "=== OUTPUT (cleaned) ===")
	cleanedOutput := ""
	if stdoutErr == nil {
		cleanedOutput = sanitizeOutputLog(stdoutRaw)
	}
	if strings.TrimSpace(cleanedOutput) == "" {
		fmt.Fprintln(out, "[no printable output captured]")
	} else {
		fmt.Fprint(out, cleanedOutput)
		if cleanedOutput[len(cleanedOutput)-1] != '\n' {
			fmt.Fprintln(out)
		}
	}
	return nil
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findRepoRootFrom(cwd)
}

func findRepoRootFrom(start string) (string, error) {
	current := filepath.Clean(start)
	trail := []string{current}
	var gitRoot string

	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			gitRoot = current
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("not inside a git repository")
		}
		current = parent
		trail = append(trail, current)
	}

	// Prefer the nearest workspace folder within the git repo that already
	// carries spec/session state, then fall back to git root.
	for _, dir := range trail {
		if hasPath(filepath.Join(dir, stateDirName)) || hasPath(filepath.Join(dir, "SPEC.md")) {
			return dir, nil
		}
		if dir == gitRoot {
			break
		}
	}

	return gitRoot, nil
}

func hasPath(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func recentCommitLines(repoRoot string, limit int) []string {
	cmd := exec.Command("git", "-C", repoRoot, "--no-pager", "log", "--oneline", fmt.Sprintf("-%d", limit))
	out, err := cmd.Output()
	if err != nil {
		return []string{"No git history available for notable sequence summary."}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		trimmed = append(trimmed, line)
	}
	if len(trimmed) == 0 {
		return []string{"No commits found yet."}
	}
	return trimmed
}

func sanitizeFileSegment(segment string) string {
	segment = strings.ToLower(segment)
	segment = strings.ReplaceAll(segment, string(os.PathSeparator), "-")
	replacer := strings.NewReplacer(" ", "-", "\t", "-", "\n", "-", "\r", "-", ":", "-", "\\", "-", "/", "-")
	return replacer.Replace(segment)
}

func sanitizeInputLog(raw []byte) string {
	return sanitizeTerminalLog(raw, true)
}

func sanitizeOutputLog(raw []byte) string {
	return sanitizeTerminalLog(raw, false)
}

func sanitizeTerminalLog(raw []byte, input bool) string {
	out := make([]rune, 0, len(raw))
	popRune := func() {
		if len(out) == 0 {
			return
		}
		out = out[:len(out)-1]
	}

	for i := 0; i < len(raw); {
		b := raw[i]
		if b == 0x1b {
			// ESC sequence
			if i+1 < len(raw) {
				next := raw[i+1]
				switch next {
				case ']': // OSC ... BEL or ST (ESC \)
					i += 2
					for i < len(raw) {
						if raw[i] == 0x07 {
							i++
							break
						}
						if raw[i] == 0x1b && i+1 < len(raw) && raw[i+1] == '\\' {
							i += 2
							break
						}
						i++
					}
					continue
				case '[': // CSI
					i += 2
					var final byte
					for i < len(raw) {
						if raw[i] >= 0x40 && raw[i] <= 0x7e {
							final = raw[i]
							i++
							break
						}
						i++
					}
					if !input {
						switch final {
						case 'H', 'f', 'J', 'K':
							if len(out) == 0 || out[len(out)-1] != '\n' {
								out = append(out, '\n')
							}
						}
					}
					continue
				case 'O': // SS3
					i += 2
					if i < len(raw) {
						i++
					}
					continue
				default:
					i += 2
					continue
				}
			}
			i++
			continue
		}

		// C0 controls + DEL
		if b < 0x20 || b == 0x7f {
			switch b {
			case '\n':
				out = append(out, '\n')
			case '\r':
				// In input streams, Enter is typically CR; keep it as a newline.
				// In output streams, CR is often used for in-place redraw/spinners,
				// so suppress it to avoid newline explosions.
				if input && (len(out) == 0 || out[len(out)-1] != '\n') {
					out = append(out, '\n')
				}
			case '\t':
				out = append(out, '\t')
			case '\b', 0x7f:
				popRune()
			case 0x03:
				if input {
					out = append(out, []rune("<CTRL-C>")...)
				}
			case 0x04:
				if input {
					out = append(out, []rune("<CTRL-D>")...)
				}
			default:
				// drop bell and other controls
			}
			i++
			continue
		}

		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		out = append(out, r)
		i += size
	}

	lines := strings.Split(string(out), "\n")
	for idx, line := range lines {
		lines[idx] = strings.TrimRight(line, " \t")
	}

	maxBlankLines := 1
	if !input {
		maxBlankLines = 0
	}

	var compact []string
	blankCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount > maxBlankLines {
				continue
			}
			compact = append(compact, "")
			continue
		}
		blankCount = 0
		compact = append(compact, line)
	}

	if !input {
		deduped := make([]string, 0, len(compact))
		prev := ""
		for _, line := range compact {
			if strings.TrimSpace(line) != "" && line == prev {
				continue
			}
			deduped = append(deduped, line)
			prev = line
		}
		compact = deduped
	}

	return strings.TrimSpace(strings.Join(compact, "\n"))
}

func extractDialog(input, output string) []string {
	var lines []string

	for _, line := range strings.Split(input, "\n") {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "<CTRL-") {
			continue
		}
		lines = append(lines, "User: "+text)
	}

	for _, line := range strings.Split(output, "\n") {
		text := strings.TrimSpace(line)
		if !strings.HasPrefix(text, "● ") {
			continue
		}
		msg := strings.TrimSpace(strings.TrimPrefix(text, "● "))
		if msg == "" {
			continue
		}
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "loading environment") ||
			strings.Contains(lower, "environment loaded") ||
			strings.Contains(lower, "thinking") ||
			strings.Contains(lower, "ctrl+c again to exit") ||
			strings.Contains(lower, "no copilot instructions found") {
			continue
		}
		lines = append(lines, "Assistant: "+msg)
	}

	return lines
}

func buildInputPreview(repoRoot, stdinRel string) string {
	stdinRel = strings.TrimSpace(stdinRel)
	if stdinRel == "" {
		return ""
	}
	stdinRaw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(stdinRel)))
	if err != nil {
		return ""
	}
	cleaned := sanitizeInputLog(stdinRaw)
	lines := strings.Split(cleaned, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "<CTRL-") {
			continue
		}
		return line
	}
	return ""
}

func buildInputPreviewFromRaw(stdinRaw []byte) string {
	cleaned := sanitizeInputLog(stdinRaw)
	lines := strings.Split(cleaned, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "<CTRL-") {
			continue
		}
		return line
	}
	return ""
}

func buildInputPreviewFromConversation(messages []conversationMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		line := strings.TrimSpace(messages[i].Text)
		if line == "" || strings.HasPrefix(line, "<CTRL-") {
			continue
		}
		return line
	}
	return ""
}

func buildConversationMessages(stdinRaw, stdoutRaw []byte) []conversationMessage {
	dialog := extractDialog(sanitizeInputLog(stdinRaw), sanitizeOutputLog(stdoutRaw))
	out := make([]conversationMessage, 0, len(dialog))
	for _, line := range dialog {
		switch {
		case strings.HasPrefix(line, "User: "):
			text := strings.TrimSpace(strings.TrimPrefix(line, "User: "))
			if text != "" {
				out = append(out, conversationMessage{Dt: time.Now().UTC().Format(time.RFC3339), Role: "user", Text: text})
			}
		case strings.HasPrefix(line, "Assistant: "):
			text := strings.TrimSpace(strings.TrimPrefix(line, "Assistant: "))
			if text != "" {
				out = append(out, conversationMessage{Dt: time.Now().UTC().Format(time.RFC3339), Role: "assistant", Text: text})
			}
		}
	}
	if len(out) == 0 {
		// Fallback: preserve at least cleaned user input as one message.
		if text := strings.TrimSpace(buildInputPreviewFromRaw(stdinRaw)); text != "" {
			out = append(out, conversationMessage{Dt: time.Now().UTC().Format(time.RFC3339), Role: "user", Text: text})
		}
	}
	return out
}

func writeConversationLog(path string, messages []conversationMessage) error {
	for i := range messages {
		if strings.TrimSpace(messages[i].Dt) == "" {
			messages[i].Dt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	raw, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func readConversationLog(path string) ([]conversationMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var messages []conversationMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func ensureConversationLogHasDT(path string) error {
	messages, err := readConversationLog(path)
	if err != nil {
		return err
	}
	needsWrite := false
	for i := range messages {
		if strings.TrimSpace(messages[i].Dt) == "" {
			messages[i].Dt = time.Now().UTC().Format(time.RFC3339)
			needsWrite = true
		}
	}
	if !needsWrite {
		return nil
	}
	return writeConversationLog(path, messages)
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]byte, b.buf.Len())
	copy(cp, b.buf.Bytes())
	return cp
}

func extractInputSequence(raw []byte) []string {
	cleaned := sanitizeInputLog(raw)
	lines := strings.Split(cleaned, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "<CTRL-") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func collectInputHistoryEntries(repoRoot string, interactions []interaction, hidden map[string]struct{}, showAll bool) []inputHistoryEntry {
	entries := make([]inputHistoryEntry, 0, len(interactions))
	for _, item := range interactions {
		if isSessionMetaEvent(item.EventType) || item.EventType == eventTypeStart {
			continue
		}
		sessionKey := interactionSessionKey(item)
		if !showAll {
			if _, isHidden := hidden[sessionKey]; isHidden {
				continue
			}
		}

		text := strings.TrimSpace(item.InputPreview)
		if text == "" {
			if strings.TrimSpace(item.ConversationLog) != "" {
				if messages, err := readConversationLog(filepath.Join(repoRoot, filepath.FromSlash(item.ConversationLog))); err == nil {
					text = strings.TrimSpace(buildInputPreviewFromConversation(messages))
				}
			}
		}
		if text == "" {
			text = strings.TrimSpace(buildInputPreview(repoRoot, item.StdinLog))
		}
		if text == "" || strings.HasPrefix(text, "<CTRL-") {
			continue
		}
		ts, err := time.Parse(time.RFC3339, item.Timestamp)
		if err != nil {
			continue
		}
		entries = append(entries, inputHistoryEntry{
			Timestamp: ts,
			SessionID: sessionKey,
			Text:      text,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	deduped := make([]inputHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		if len(deduped) == 0 {
			deduped = append(deduped, entry)
			continue
		}
		prev := deduped[len(deduped)-1]
		if prev.SessionID == entry.SessionID && prev.Text == entry.Text {
			continue
		}
		deduped = append(deduped, entry)
	}
	return deduped
}

func printInputHistory(out io.Writer, entries []inputHistoryEntry) {
	currentDate := ""
	for idx, entry := range entries {
		date := entry.Timestamp.Format("2006-01-02")
		if date != currentDate {
			fmt.Fprintf(out, "%s%s%s\n", historyDayColorStart, date, historyColorReset)
			currentDate = date
		}

		wrapped := wrapWordsNoSplit(entry.Text, historyTextWrapWidth)
		for lineIdx, line := range wrapped {
			prefix := historyContinuation
			if lineIdx == 0 {
				prefix = fmt.Sprintf("  %s | ", entry.Timestamp.Format("15:04:05"))
			}
			fmt.Fprintf(out, "%s%s\n", prefix, line)
		}

		if idx < len(entries)-1 {
			fmt.Fprintln(out, historyContinuationBlank)
			nextDate := entries[idx+1].Timestamp.Format("2006-01-02")
			if nextDate != date {
				fmt.Fprintln(out, historyDayDivider)
			}
		}
	}
}

func wrapWordsNoSplit(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}
	if width <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	lines := make([]string, 0, len(words))
	current := words[0]
	for _, word := range words[1:] {
		if utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func abbreviatePreview(input string, maxLen int) string {
	input = strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
	if input == "" {
		return ""
	}
	if maxLen <= 0 || len(input) <= maxLen {
		return input
	}
	return strings.TrimSpace(input[:maxLen-3]) + "..."
}

type oscTerminalFilterWriter struct {
	dst    io.Writer
	mu     sync.Mutex
	state  int
	passth []byte
}

func newOSCTerminalFilterWriter(dst io.Writer) io.Writer {
	return &oscTerminalFilterWriter{dst: dst}
}

func (w *oscTerminalFilterWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	origLen := len(p)
	out := make([]byte, 0, len(w.passth)+len(p))
	if len(w.passth) > 0 {
		p = append(w.passth, p...)
		w.passth = nil
	}
	for i := 0; i < len(p); i++ {
		b := p[i]
		switch w.state {
		case 0: // normal
			if b == 0x1b {
				w.state = 1
				continue
			}
			out = append(out, b)
		case 1: // saw ESC
			if b == ']' {
				w.state = 2 // in OSC: drop until BEL or ST
				continue
			}
			out = append(out, 0x1b, b)
			w.state = 0
		case 2: // in OSC
			if b == 0x07 {
				w.state = 0
				continue
			}
			if b == 0x1b {
				w.state = 3
				continue
			}
		case 3: // OSC saw ESC, expect '\'
			if b == '\\' {
				w.state = 0
				continue
			}
			// Not ST; keep dropping as OSC payload.
			if b == 0x1b {
				w.state = 3
				continue
			}
			w.state = 2
		}
	}

	// If this chunk ended right after ESC, keep it for next write so we can decide.
	if w.state == 1 {
		w.passth = append(w.passth[:0], 0x1b)
		w.state = 0
	}
	if len(out) == 0 {
		return origLen, nil
	}
	_, err := w.dst.Write(out)
	return origLen, err
}

func snapshotRepoStatus(repoRoot string) (map[string]string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "--no-pager", "status", "--porcelain=v1", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	statusByPath := map[string]string{}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || len(line) < 4 {
			continue
		}
		status := line[:2]
		path := strings.TrimSpace(line[3:])
		if strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			path = strings.TrimSpace(parts[1])
		}
		if path == "" {
			continue
		}
		statusByPath[path] = status
	}
	return statusByPath, nil
}

func changedFilesBetween(before, after map[string]string) []string {
	var files []string
	for path, afterStatus := range after {
		beforeStatus, existed := before[path]
		if !existed || beforeStatus != afterStatus {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files
}

func filterIncrementalFiles(files []string) []string {
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.ToSlash(strings.TrimSpace(file))
		if path == "" {
			continue
		}
		if path == filepath.ToSlash(stateDirName+"/interactions.ndjson") {
			continue
		}
		if strings.HasPrefix(path, filepath.ToSlash(stateDirName+"/sessions/")) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}
