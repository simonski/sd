package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractCoreOmitsSkills(t *testing.T) {
	stateDir := t.TempDir()
	if err := ExtractCore(stateDir); err != nil {
		t.Fatalf("ExtractCore error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "agents", "README.md")); err != nil {
		t.Fatalf("expected agents README extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "skills", "README.md")); !os.IsNotExist(err) {
		t.Fatalf("expected skills README omitted by ExtractCore, got err=%v", err)
	}
}

func TestExtractSkillsCreatesSkillFiles(t *testing.T) {
	stateDir := t.TempDir()
	if err := ExtractSkills(stateDir); err != nil {
		t.Fatalf("ExtractSkills error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "skills", "README.md")); err != nil {
		t.Fatalf("expected skills README extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "skills", "respec", "SKILL.md")); err != nil {
		t.Fatalf("expected respec skill extracted: %v", err)
	}
}
