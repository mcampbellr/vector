// Package openspec reads a repo's OpenSpec changes so `vector sync` can project
// them onto the Vector board. It is read-only: it parses openspec/changes/* and
// reports each change's artifacts and task progress; the caller maps that to
// Vector state and the state package performs all writes (CLI-owns-writes).
package openspec

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	datePrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)
	taskLine   = regexp.MustCompile(`^\s*-\s*\[([ xX])\]`)
)

// Change is one OpenSpec change directory.
type Change struct {
	Name        string // id: change dir name (date prefix stripped when archived)
	Dir         string // repo-relative change directory
	Archived    bool   // lives under changes/archive/
	HasProposal bool
	HasDesign   bool
	HasTasks    bool
	TasksTotal  int
	TasksDone   int
	// PendingReal counts unchecked tasks that are implementation work (not manual
	// QA/verification). When it reaches 0 with work already done, the change is
	// effectively in review — implementation complete, only QA remains.
	PendingReal int
	ProposalRel string // repo-relative proposal.md path, if present
}

func changesDir(repoRoot string) string {
	return filepath.Join(repoRoot, "openspec", "changes")
}

// Detected reports whether repoRoot has an openspec/changes directory.
func Detected(repoRoot string) bool {
	info, err := os.Stat(changesDir(repoRoot))
	return err == nil && info.IsDir()
}

// ReadChanges returns every change under the repo's default openspec/changes/.
func ReadChanges(repoRoot string) ([]Change, error) {
	return ReadChangesAt(changesDir(repoRoot), repoRoot)
}

// ReadChangesAt reads changes from an explicit changes directory (resolved by
// the caller, e.g. from config for bare+worktree layouts). Change paths are
// reported relative to repoRoot. Returns nil if the directory is absent.
func ReadChangesAt(changesDirAbs, repoRoot string) ([]Change, error) {
	base := changesDirAbs
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read openspec changes: %w", err)
	}
	var changes []Change
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == "archive" {
			archived, err := readArchive(repoRoot, filepath.Join(base, "archive"))
			if err != nil {
				return nil, err
			}
			changes = append(changes, archived...)
			continue
		}
		changes = append(changes, readChange(repoRoot, base, e.Name(), false))
	}
	return changes, nil
}

func readArchive(repoRoot, archiveDir string) ([]Change, error) {
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read openspec archive: %w", err)
	}
	var out []Change
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, readChange(repoRoot, archiveDir, e.Name(), true))
		}
	}
	return out, nil
}

func readChange(repoRoot, parentDir, dirName string, archived bool) Change {
	dirAbs := filepath.Join(parentDir, dirName)
	relDir, err := filepath.Rel(repoRoot, dirAbs)
	if err != nil {
		relDir = dirAbs
	}
	c := Change{
		Name:        changeName(dirName, archived),
		Dir:         filepath.ToSlash(relDir),
		Archived:    archived,
		HasProposal: fileExists(filepath.Join(dirAbs, "proposal.md")),
		HasDesign:   fileExists(filepath.Join(dirAbs, "design.md")),
		HasTasks:    fileExists(filepath.Join(dirAbs, "tasks.md")),
	}
	if c.HasProposal {
		c.ProposalRel = path.Join(c.Dir, "proposal.md")
	}
	if c.HasTasks {
		c.TasksTotal, c.TasksDone, c.PendingReal = scanTasks(filepath.Join(dirAbs, "tasks.md"))
	}
	return c
}

// changeName strips the YYYY-MM-DD- archive prefix so an archived change keeps
// the same id it had while active (id == OpenSpec change name).
func changeName(dirName string, archived bool) string {
	if archived {
		return datePrefix.ReplaceAllString(dirName, "")
	}
	return dirName
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func scanTasks(taskPath string) (total, done, pendingReal int) {
	b, err := os.ReadFile(taskPath)
	if err != nil {
		return 0, 0, 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		m := taskLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		total++
		switch {
		case m[1] == "x" || m[1] == "X":
			done++
		case !isVerificationTask(line):
			pendingReal++
		}
	}
	return total, done, pendingReal
}

// isVerificationTask reports whether an unchecked task is manual QA / verification
// rather than implementation work — so a change with only these left counts as
// review. Conservative: the QA/check/test/verify branch requires "manual" to
// avoid catching implementation tasks like "verify the schema".
func isVerificationTask(line string) bool {
	l := strings.ToLower(line)
	if strings.Contains(l, "smoke test") || strings.Contains(l, "e2e") || strings.Contains(l, "end-to-end") {
		return true
	}
	if strings.Contains(l, "manual") &&
		(strings.Contains(l, "check") || strings.Contains(l, "qa") ||
			strings.Contains(l, "test") || strings.Contains(l, "verif")) {
		return true
	}
	return false
}
