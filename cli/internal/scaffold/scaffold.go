// Package scaffold seeds Vector's Claude Code artifacts (the /vector:* project
// commands) into a target repo. The artifacts are embedded into the binary so
// the global `vector` binary can seed any repo without needing kit/ on disk.
//
// The embedded assets under assets/ are a vendored copy of kit/commands/, kept
// in sync via `go generate`. Everything under assets/ mirrors into the target
// repo's .claude/ directory (assets/commands/vector/raw.md -> .claude/commands/
// vector/raw.md), so only files meant to live under .claude/ belong in assets/.
package scaffold

//go:generate sh -c "rm -rf assets/commands && cp -R ../../../kit/commands assets/commands"

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed all:assets
var assets embed.FS

// embedRoot is the directory inside the embedded FS whose contents mirror into
// the target repo's .claude/ directory.
const embedRoot = "assets"

// Action describes what SeedCommands did with a single file.
type Action string

const (
	ActionCreated     Action = "created"
	ActionOverwritten Action = "overwritten"
	ActionSkipped     Action = "skipped" // already present; left untouched
)

// FileResult is the outcome for one seeded file. Path is relative to the repo root.
type FileResult struct {
	Path   string `json:"path"`
	Action Action `json:"action"`
}

// SeedOptions controls SeedCommands.
type SeedOptions struct {
	Force  bool // overwrite files that already exist
	DryRun bool // report intended actions without writing anything
}

// SeedCommands writes the embedded .claude artifacts into repoRoot/.claude,
// without touching anything else under .claude. Existing files are left intact
// unless Force is set. Results are returned in deterministic (lexical) order.
func SeedCommands(repoRoot string, opts SeedOptions) ([]FileResult, error) {
	claudeDir := filepath.Join(repoRoot, ".claude")

	var results []FileResult
	walkErr := fs.WalkDir(assets, embedRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, embedRoot+"/") // embed paths always use "/"
		target := filepath.Join(append([]string{claudeDir}, strings.Split(rel, "/")...)...)

		data, err := assets.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", p, err)
		}
		action, err := writeSeed(target, data, opts)
		if err != nil {
			return fmt.Errorf("seed %s: %w", rel, err)
		}
		relToRepo, err := filepath.Rel(repoRoot, target)
		if err != nil {
			relToRepo = target
		}
		results = append(results, FileResult{Path: relToRepo, Action: action})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return results, nil
}

// CommandPaths returns the repo-relative paths SeedCommands would write, for
// reporting and tests. Order is deterministic.
func CommandPaths() ([]string, error) {
	var paths []string
	err := fs.WalkDir(assets, embedRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, embedRoot+"/")
		paths = append(paths, path.Join(".claude", rel))
		return nil
	})
	return paths, err
}

func writeSeed(target string, data []byte, opts SeedOptions) (Action, error) {
	_, statErr := os.Stat(target)
	exists := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("stat %s: %w", target, statErr)
	}
	if exists && !opts.Force {
		return ActionSkipped, nil
	}
	if opts.DryRun {
		if exists {
			return ActionOverwritten, nil
		}
		return ActionCreated, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}
	if err := writeFileAtomic(target, data); err != nil {
		return "", err
	}
	if exists {
		return ActionOverwritten, nil
	}
	return ActionCreated, nil
}

// writeFileAtomic writes data via a temp file in the same directory and an
// atomic rename, so readers never observe a partial file.
func writeFileAtomic(targetPath string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}
	return nil
}
