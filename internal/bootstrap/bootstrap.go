package bootstrap

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/**
var embeddedAssets embed.FS

func Extract(stateDir string) error {
	return fs.WalkDir(embeddedAssets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "assets" {
			return nil
		}

		relative := strings.TrimPrefix(path, "assets/")
		target := filepath.Join(stateDir, relative)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		raw, err := embeddedAssets.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}
