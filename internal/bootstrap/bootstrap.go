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
	return extractWithFilter(stateDir, func(_ string, _ bool) bool { return true })
}

func ExtractCore(stateDir string) error {
	return extractWithFilter(stateDir, func(relative string, _ bool) bool {
		return !strings.HasPrefix(relative, "skills/")
	})
}

func ExtractSkills(stateDir string) error {
	return extractWithFilter(stateDir, func(relative string, _ bool) bool {
		return relative == "skills" || strings.HasPrefix(relative, "skills/")
	})
}

func extractWithFilter(stateDir string, include func(relative string, isDir bool) bool) error {
	return fs.WalkDir(embeddedAssets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "assets" {
			return nil
		}

		relative := strings.TrimPrefix(path, "assets/")
		if !include(relative, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
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
		return writeFileAtomic(target, raw, 0o644)
	})
}

func writeFileAtomic(path string, raw []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}
