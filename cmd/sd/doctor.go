package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type terminalDoctorInfo struct {
	TerminalName    string
	TermProgram     string
	Term            string
	InTmux          bool
	TmuxPane        string
	OverlaySupport  bool
	OverlayStatus   string
	SupportNextStep string
}

func runDoctor(args []string, out io.Writer) (int, error) {
	if len(args) > 0 {
		return 1, fmt.Errorf("unknown doctor argument %q", args[0])
	}

	info := detectTerminalDoctorInfo(os.Getenv)
	fmt.Fprintln(out, "sd doctor")
	fmt.Fprintf(out, "Terminal: %s\n", info.TerminalName)
	if info.TermProgram != "" {
		fmt.Fprintf(out, "TERM_PROGRAM: %s\n", info.TermProgram)
	}
	if info.Term != "" {
		fmt.Fprintf(out, "TERM: %s\n", info.Term)
	}
	fmt.Fprintf(out, "In tmux: %t\n", info.InTmux)
	if info.TmuxPane != "" {
		fmt.Fprintf(out, "TMUX_PANE: %s\n", info.TmuxPane)
	}
	fmt.Fprintf(out, "Panel overlay support: %s\n", info.OverlayStatus)
	fmt.Fprintf(out, "Next step: %s\n", info.SupportNextStep)
	return 0, nil
}

func detectTerminalDoctorInfo(getenv func(string) string) terminalDoctorInfo {
	termProgram := strings.TrimSpace(getenv("TERM_PROGRAM"))
	term := strings.TrimSpace(getenv("TERM"))
	tmux := strings.TrimSpace(getenv("TMUX"))
	tmuxPane := strings.TrimSpace(getenv("TMUX_PANE"))
	inTmux := tmux != "" && tmuxPane != ""

	name := "Unknown terminal"
	switch {
	case inTmux:
		name = "tmux"
	case termProgram == "Apple_Terminal":
		name = "macOS Terminal"
	case termProgram == "iTerm.app":
		name = "iTerm2"
	case termProgram == "WezTerm":
		name = "WezTerm"
	case termProgram == "vscode":
		name = "VS Code terminal"
	case termProgram == "WarpTerminal":
		name = "Warp"
	case strings.Contains(strings.ToLower(term), "xterm"):
		name = "xterm-compatible terminal"
	}

	info := terminalDoctorInfo{
		TerminalName: name,
		TermProgram:  termProgram,
		Term:         term,
		InTmux:       inTmux,
		TmuxPane:     tmuxPane,
	}
	if inTmux {
		info.OverlaySupport = true
		info.OverlayStatus = "supported (tmux popup overlay)"
		info.SupportNextStep = "Use double-press shortcuts to open and dismiss panel (Esc also dismisses)."
		return info
	}
	if termProgram == "Apple_Terminal" {
		info.OverlaySupport = true
		info.OverlayStatus = "supported (native macOS Terminal overlay)"
		info.SupportNextStep = "Use double-press shortcuts to open and dismiss panel (Esc also dismisses)."
		return info
	}

	info.OverlaySupport = false
	info.OverlayStatus = "not supported in this terminal yet"
	info.SupportNextStep = "Use tmux for popup overlays, or macOS Terminal for native non-tmux overlays."
	return info
}
