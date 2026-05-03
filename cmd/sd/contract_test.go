package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cliContract struct {
	Version  int `json:"version"`
	Commands []struct {
		Name string `json:"name"`
	} `json:"commands"`
}

func TestCLIContractMatchesUsage(t *testing.T) {
	contractPath := filepath.Join("..", "..", "docs", "cli-contract.json")
	raw, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read cli contract: %v", err)
	}
	var contract cliContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatalf("unmarshal cli contract: %v", err)
	}
	if contract.Version <= 0 {
		t.Fatalf("invalid contract version: %d", contract.Version)
	}
	var sb strings.Builder
	printUsage(&sb)
	usage := sb.String()
	for _, cmd := range contract.Commands {
		if strings.TrimSpace(cmd.Name) == "" {
			t.Fatalf("contract contains empty command name")
		}
		if !strings.Contains(usage, "  "+cmd.Name+" ") {
			t.Fatalf("command %q missing from usage output", cmd.Name)
		}
	}
}
