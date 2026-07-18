package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mariocampbell/vector/internal/state"
	"github.com/mariocampbell/vector/internal/ui"
	"github.com/spf13/cobra"
)

// strayStore is one .vector/ directory found below the canonical root with no
// loadable config.json — an orphan store that no command reads or writes.
type strayStore struct {
	Path  string   `json:"path"`  // absolute path to the stray .vector directory
	Specs []string `json:"specs"` // spec slugs stranded inside it
}

// skippedScanDirs are never descended into while scanning for strays: heavy,
// vendored, or irrelevant to Vector's own layout.
var skippedScanDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"vendor":       true,
}

// newDoctorCmd diagnoses store fragmentation. Bare `doctor` is read-only: it
// reports stray .vector/ directories below the canonical root. `doctor adopt`
// consolidates one of them into the canonical store, and mutates only with
// --force (see .claude/rules/security/destructive-ops-consent.md).
func newDoctorCmd() *cobra.Command {
	var (
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "report stray .vector/ stores below the canonical root",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, strays, err := resolveRepoRootStrays(repoRoot)
			if err != nil {
				return err
			}
			found, err := scanStrayStores(root)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSONValue(found)
			}
			warnStrayStores(strays, root)
			if len(found) == 0 {
				fmt.Println(ui.Success(fmt.Sprintf("no stray .vector/ stores below %s", root)))
				return nil
			}
			rows := make([][]string, 0, len(found))
			for _, s := range found {
				rows = append(rows, []string{s.Path, strings.Join(s.Specs, ", ")})
			}
			fmt.Println(ui.Table([]string{"stray store", "specs"}, rows))
			fmt.Println(ui.Info("consolidate with: vector doctor adopt <stray-path> --force"))
			return nil
		},
	}
	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "repo root (default: nearest ancestor Vector store)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable JSON")
	cmd.AddCommand(newDoctorAdoptCmd())
	return cmd
}

// adoptResult is the outcome of `doctor adopt`, in plan or applied form.
type adoptResult struct {
	Stray     string   `json:"stray"`
	Canonical string   `json:"canonical"`
	Applied   bool     `json:"applied"` // false for the dry-run plan
	Specs     []string `json:"specs"`   // spec slugs migrated (or that would be)
	Events    int      `json:"events"`  // activity.jsonl lines merged (or that would be)
	Local     []string `json:"local"`   // local state files moved (or that would be)
	Conflicts []string `json:"conflicts,omitempty"`
	Removed   bool     `json:"removed"` // the stray directory was deleted
}

// newDoctorAdoptCmd migrates a stray store into the canonical one. Without
// --force it prints the plan and touches nothing; with --force it moves the
// specs, merges the activity log chronologically, moves the remaining local
// state, and deletes the stray LAST — only when everything else succeeded, so a
// failure part-way leaves the stray intact.
func newDoctorAdoptCmd() *cobra.Command {
	var (
		repoRoot string
		force    bool
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "adopt <stray-path>",
		Short: "migrate a stray .vector/ store into the canonical one",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := resolveRepoRoot(repoRoot)
			if err != nil {
				return err
			}
			stray, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if err := validateStray(stray, root); err != nil {
				return err
			}
			result, err := adoptStray(stray, root, force)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSONValue(result)
			}
			printAdoptResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "repo root (default: nearest ancestor Vector store)")
	cmd.Flags().BoolVar(&force, "force", false, "apply the migration (without it, only the plan is printed)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable JSON")
	return cmd
}

// scanStrayStores walks below root for .vector/ directories with no loadable
// config.json. The canonical store itself is never reported. Read-only.
func scanStrayStores(root string) ([]strayStore, error) {
	canonical := filepath.Join(root, ".vector")
	var found []strayStore
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable subtree: skip, never fail the whole scan
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && skippedScanDirs[entry.Name()] {
			return filepath.SkipDir
		}
		if entry.Name() != ".vector" {
			return nil
		}
		if path == canonical {
			return filepath.SkipDir
		}
		if isStrayStore(path) {
			found = append(found, strayStore{Path: path, Specs: straySpecs(path)})
		}
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("scan for stray stores: %w", err)
	}
	if found == nil {
		found = []strayStore{}
	}
	return found, nil
}

// isStrayStore reports whether path is a .vector directory whose config.json is
// missing or unreadable — the definition of a store no command anchors to.
func isStrayStore(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(path, "config.json"))
	return err != nil
}

// straySpecs lists the spec slugs stranded in a stray store.
func straySpecs(strayPath string) []string {
	entries, err := os.ReadDir(filepath.Join(strayPath, "specs"))
	if err != nil {
		return nil
	}
	var slugs []string
	for _, entry := range entries {
		if entry.IsDir() {
			slugs = append(slugs, entry.Name())
		}
	}
	sort.Strings(slugs)
	return slugs
}

// validateStray rejects anything that is not a stray store, or the canonical
// store itself, before any plan is computed.
func validateStray(stray, root string) error {
	if stray == filepath.Join(root, ".vector") {
		return fmt.Errorf("%q is the canonical store, not a stray", stray)
	}
	if !isStrayStore(stray) {
		return fmt.Errorf("%q is not a stray .vector/ directory (missing or has config.json)", stray)
	}
	return nil
}

// adoptStray computes the migration plan and, when apply is set, performs it.
// Order matters: specs, then activity, then local state, and only then the
// deletion of the stray — so any failure leaves the stray recoverable.
func adoptStray(stray, root string, apply bool) (adoptResult, error) {
	canonical := filepath.Join(root, ".vector")
	result := adoptResult{Stray: stray, Canonical: canonical, Applied: apply}

	if apply {
		if _, err := state.Open(root); err != nil {
			return result, fmt.Errorf("open canonical store: %w", err)
		}
	}

	for _, slug := range straySpecs(stray) {
		src := filepath.Join(stray, "specs", slug)
		dst := filepath.Join(canonical, "specs", slug)
		if _, err := os.Stat(dst); err == nil {
			result.Conflicts = append(result.Conflicts, slug)
			continue
		}
		result.Specs = append(result.Specs, slug)
		if !apply {
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			return result, fmt.Errorf("migrate spec %s: %w", slug, err)
		}
	}

	events, err := mergeActivity(stray, canonical, apply)
	if err != nil {
		return result, err
	}
	result.Events = events

	local, err := moveLocalState(stray, canonical, apply)
	if err != nil {
		return result, err
	}
	result.Local = local

	// The stray is removed only when nothing was left behind: a conflicted spec
	// still lives inside it, and deleting would destroy the only copy.
	if apply && len(result.Conflicts) == 0 {
		if err := os.RemoveAll(stray); err != nil {
			return result, fmt.Errorf("remove stray store: %w", err)
		}
		result.Removed = true
	}
	return result, nil
}

// mergeActivity appends the stray's activity lines into the canonical log and
// rewrites it in timestamp order. Unparseable lines are kept (sorted last)
// rather than dropped — the log is append-only history, not a cache.
func mergeActivity(stray, canonical string, apply bool) (int, error) {
	strayLines, err := readActivityLines(filepath.Join(stray, "local", "activity.jsonl"))
	if err != nil {
		return 0, err
	}
	if len(strayLines) == 0 || !apply {
		return len(strayLines), nil
	}
	canonicalPath := filepath.Join(canonical, "local", "activity.jsonl")
	canonicalLines, err := readActivityLines(canonicalPath)
	if err != nil {
		return 0, err
	}
	merged := append(canonicalLines, strayLines...)
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].event.TS.Before(merged[j].event.TS) })

	var buf strings.Builder
	for _, line := range merged {
		buf.WriteString(line.raw)
		buf.WriteString("\n")
	}
	if err := os.MkdirAll(filepath.Dir(canonicalPath), 0o755); err != nil {
		return 0, fmt.Errorf("create local dir: %w", err)
	}
	if err := os.WriteFile(canonicalPath, []byte(buf.String()), 0o644); err != nil {
		return 0, fmt.Errorf("write merged activity: %w", err)
	}
	return len(strayLines), nil
}

// rawEvent keeps a JSONL activity line verbatim alongside its decoded form, so
// the merge can reorder by timestamp without re-encoding (and without dropping
// fields this binary does not know about).
type rawEvent struct {
	raw   string
	event state.Event
}

// readActivityLines reads a JSONL activity log, keeping each line verbatim and
// extracting its timestamp for ordering. A missing file is not an error.
func readActivityLines(path string) ([]rawEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read activity log: %w", err)
	}
	defer f.Close()

	var lines []rawEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var evt state.Event
		_ = json.Unmarshal([]byte(text), &evt) // zero TS sorts first; the line is kept regardless
		lines = append(lines, rawEvent{raw: text, event: evt})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan activity log: %w", err)
	}
	return lines, nil
}

// moveLocalState moves the stray's remaining local files (everything but the
// already-merged activity log) into the canonical local dir. Existing files are
// never overwritten: they are left in place and reported as untouched.
func moveLocalState(stray, canonical string, apply bool) ([]string, error) {
	srcDir := filepath.Join(stray, "local")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read stray local state: %w", err)
	}
	dstDir := filepath.Join(canonical, "local")
	moved := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if name == "activity.jsonl" {
			continue
		}
		dst := filepath.Join(dstDir, name)
		if _, err := os.Stat(dst); err == nil {
			continue // canonical wins; never clobber local state
		}
		moved = append(moved, name)
		if !apply {
			continue
		}
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, fmt.Errorf("create local dir: %w", err)
		}
		if err := os.Rename(filepath.Join(srcDir, name), dst); err != nil {
			return nil, fmt.Errorf("move local state %s: %w", name, err)
		}
	}
	return moved, nil
}

// printAdoptResult renders the plan or the applied migration for humans.
func printAdoptResult(result adoptResult) {
	verb := "would migrate"
	if result.Applied {
		verb = "migrated"
	}
	fmt.Println(ui.Info(fmt.Sprintf("%s %s → %s", verb, result.Stray, result.Canonical)))
	fmt.Println(ui.KeyValue("specs", strings.Join(result.Specs, ", ")))
	fmt.Println(ui.KeyValue("activity events", fmt.Sprint(result.Events)))
	fmt.Println(ui.KeyValue("local state", strings.Join(result.Local, ", ")))
	for _, slug := range result.Conflicts {
		fmt.Println(ui.Warning(fmt.Sprintf("skipped %s: a spec with that id already exists in the canonical store", slug)))
	}
	switch {
	case result.Removed:
		fmt.Println(ui.Success("stray store removed"))
	case result.Applied:
		fmt.Println(ui.Warning("stray store kept: resolve the conflicts above, then re-run adopt"))
	default:
		fmt.Println(ui.Info("nothing was written — re-run with --force to apply"))
	}
}
