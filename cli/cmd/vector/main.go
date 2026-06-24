// Command vector is the developer-focused spec/kanban companion CLI for Claude
// Code. It is the sole writer of Vector's on-disk state; the /vector:* project
// commands (seeded by `vector init`) invoke this binary rather than editing
// state directly.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/scaffold"
	"github.com/mariocampbell/vector/internal/state"
)

const version = "0.0.1-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "spec":
		err = runSpec(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("vector", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runInit seeds the /vector:* project commands into the repo's
// .claude/commands/vector/ and initializes the .vector state skeleton. It is
// additive: nothing else under .claude is touched, and existing command files
// are left intact unless --force is given.
func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	force := fs.Bool("force", false, "overwrite existing /vector:* command files")
	dryRun := fs.Bool("dry-run", false, "show what would be written without writing")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}

	results, err := scaffold.SeedCommands(root, scaffold.SeedOptions{Force: *force, DryRun: *dryRun})
	if err != nil {
		return fmt.Errorf("seed vector commands: %w", err)
	}

	// Initialize the state skeleton so the repo is board-ready.
	if !*dryRun {
		if _, err := state.Open(root); err != nil {
			return fmt.Errorf("init state: %w", err)
		}
	}

	if *jsonOut {
		b, err := json.MarshalIndent(struct {
			Root   string                `json:"root"`
			DryRun bool                  `json:"dryRun"`
			Files  []scaffold.FileResult `json:"files"`
		}{Root: root, DryRun: *dryRun, Files: results}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json result: %w", err)
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("vector init: %s\n", root)
	for _, r := range results {
		fmt.Printf("  %-12s %s\n", r.Action, r.Path)
	}
	if *dryRun {
		fmt.Println("\n(dry run — nothing written)")
		return nil
	}
	fmt.Println("\nReload Claude Code (/reload-plugins) to pick up the /vector:* commands.")
	return nil
}

func runSpec(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vector spec <create|list> ...")
	}
	switch args[0] {
	case "create":
		return runSpecCreate(args[1:])
	case "list":
		return runSpecList(args[1:])
	default:
		return fmt.Errorf("unknown spec subcommand %q", args[0])
	}
}

func runSpecCreate(args []string) error {
	fs := flag.NewFlagSet("spec create", flag.ContinueOnError)
	title := fs.String("title", "", "spec title (required unless --id is given)")
	id := fs.String("id", "", "spec id (kebab-case); derived from title if empty")
	repo := fs.String("repo", "", "repo name for the board")
	priority := fs.String("priority", "normal", "urgent|high|normal|low")
	bodyFile := fs.String("body-file", "", "path to spec.md body, or - for stdin")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*title) == "" && *id == "" {
		return fmt.Errorf("--title or --id is required")
	}

	body, err := readBody(*bodyFile)
	if err != nil {
		return err
	}
	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}
	store, err := state.Open(root)
	if err != nil {
		return err
	}
	spec, err := store.CreateSpec(state.CreateSpecParams{
		Title:    *title,
		ID:       *id,
		Repo:     *repo,
		Priority: state.Priority(*priority),
		Body:     body,
		Actor:    resolveActor(),
		Now:      time.Now(),
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(map[string]string{
			"id":     spec.ID,
			"status": string(spec.Status),
			"state":  store.StatePath(spec.ID),
		})
	}
	fmt.Printf("created spec %q (status: %s)\n", spec.ID, spec.Status)
	return nil
}

func runSpecList(args []string) error {
	fs := flag.NewFlagSet("spec list", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := resolveRepoRoot(*repoRoot)
	if err != nil {
		return err
	}
	store, err := state.Open(root)
	if err != nil {
		return err
	}
	specs, err := store.ListSpecs()
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		fmt.Println("no specs")
		return nil
	}
	for _, s := range specs {
		fmt.Printf("%-40s %-16s %-8s %s\n", s.ID, s.Status, s.Priority, s.Title)
	}
	return nil
}

func readBody(path string) (string, error) {
	switch path {
	case "":
		return "", nil
	case "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(b), nil
	default:
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read body file: %w", err)
		}
		return string(b), nil
	}
}

// resolveRepoRoot returns the explicit root if given, else the git toplevel,
// else the current working directory.
func resolveRepoRoot(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		if root := strings.TrimSpace(string(out)); root != "" {
			return root, nil
		}
	}
	return os.Getwd()
}

// resolveActor identifies who triggered an action, for the activity log.
func resolveActor() string {
	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "unknown"
}

func printJSON(v map[string]string) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json result: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `vector — developer-focused spec/kanban companion for Claude Code

usage:
  vector init [--repo-root path] [--force] [--dry-run] [--json]
  vector spec create --title "..." [--id slug] [--repo name] [--priority normal] [--body-file -|path] [--json]
  vector spec list
  vector version
`)
}
