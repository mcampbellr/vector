package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/state"
	"github.com/spf13/cobra"
)

// newSpecFocusCmd / newSpecUnfocusCmd toggle the developer's "work on this first"
// marker. Two verbs rather than `focus --off`, mirroring close/archive: each
// command names exactly one action, with no boolean flag inverting its meaning.
func newSpecFocusCmd() *cobra.Command {
	return newFocusToggleCmd("focus", "mark a spec to work on first (sorts ahead of priority)", true)
}

func newSpecUnfocusCmd() *cobra.Command {
	return newFocusToggleCmd("unfocus", "remove a spec's focus marker", false)
}

// newFocusToggleCmd builds the shared focus/unfocus command: a leading id
// positional (or --id), persisted via Store.SetFocus — metadata only, never a
// status change. Idempotent: re-applying the current value reports changed:false.
func newFocusToggleCmd(name, short string, focus bool) *cobra.Command {
	var (
		idFlag   string
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   name + " [id]",
		Short: short,
		RunE: func(_ *cobra.Command, args []string) error {
			id, _ := leadingID(args)
			if id == "" {
				id = idFlag
			}
			if id == "" {
				return fmt.Errorf("usage: vector spec %s <id>", name)
			}
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			changed, err := store.SetFocus(id, focus, resolveActor(), time.Now())
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSONValue(map[string]any{"id": id, "focus": focus, "changed": changed})
			}
			switch {
			case !changed && focus:
				fmt.Printf("spec %q is already focused (no change)\n", id)
			case !changed:
				fmt.Printf("spec %q is not focused (no change)\n", id)
			case focus:
				fmt.Printf("focused spec %q — it now sorts first in its column and in `vector spec next`\n", id)
			default:
				fmt.Printf("unfocused spec %q\n", id)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&idFlag, "id", "", "spec id (or pass it as the first argument)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

// newSpecEpicCmd assigns specs to an epic, or clears it — metadata only, never
// a status change. Three forms:
//
//	vector spec epic <spec-id> <epic-id>                  single (legacy)
//	vector spec epic <spec-id> --clear                    single clear (legacy)
//	vector spec epic --epic <epic-id> <spec-id>...        bulk assign
//	vector spec epic --clear <spec-id> <spec-id>...       bulk clear
//
// --stdin adds newline-separated spec ids read from stdin (bulk form). Every id
// is validated before anything is written: an unknown spec fails the whole call.
// The single legacy forms keep their {id, epic, changed} JSON; the bulk forms
// report per-spec results (see epicBulkJSON).
func newSpecEpicCmd() *cobra.Command {
	var (
		epicFlag  string
		clearEpic bool
		fromStdin bool
		repoRoot  string
		jsonOut   bool
	)
	cmd := &cobra.Command{
		Use:   "epic [spec-id...] [epic-id]",
		Short: "assign specs to an epic (--epic <id> <spec>... | <spec> <epic>) or --clear them",
		RunE: func(cmd *cobra.Command, args []string) error {
			positionals := make([]string, 0, len(args))
			for _, arg := range args {
				if !strings.HasPrefix(arg, "-") {
					positionals = append(positionals, arg)
				}
			}
			bulk := cmd.Flags().Changed("epic") || fromStdin
			if bulk && strings.TrimSpace(epicFlag) == "" && !clearEpic {
				return errors.New("--epic needs an epic id (use --clear to remove specs from their epic)")
			}
			if strings.TrimSpace(epicFlag) != "" && clearEpic {
				return errors.New("pass either --epic or --clear, not both")
			}

			var specIDs []string
			epicID := strings.TrimSpace(epicFlag)
			switch {
			case bulk || (clearEpic && len(positionals) > 1):
				bulk = true
				specIDs = positionals
			case clearEpic:
				if len(positionals) != 1 {
					return errors.New(specEpicUsage)
				}
				specIDs = positionals
			default:
				// Legacy single form: exactly <spec-id> <epic-id>.
				switch len(positionals) {
				case 2:
					specIDs, epicID = positionals[:1], positionals[1]
				case 0, 1:
					return errors.New(specEpicUsage)
				default:
					return fmt.Errorf("got %d ids: to assign several specs use `vector spec epic --epic <epic-id> <spec-id>...`", len(positionals))
				}
			}
			if fromStdin {
				stdinIDs, err := readIDLines(os.Stdin)
				if err != nil {
					return err
				}
				specIDs = append(specIDs, stdinIDs...)
			}
			if len(specIDs) == 0 {
				return errors.New(specEpicUsage)
			}

			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			if !bulk {
				return runSingleEpicAssign(store, specIDs[0], epicID, jsonOut)
			}
			results, err := store.AssignEpicBulk(specIDs, epicID, resolveActor(), time.Now())
			if err != nil {
				return err
			}
			return printEpicBulkResults(epicID, results, jsonOut)
		},
	}
	f := cmd.Flags()
	f.StringVar(&epicFlag, "epic", "", "bulk form: the epic to assign every listed spec to")
	f.BoolVar(&clearEpic, "clear", false, "remove the listed spec(s) from their epic")
	f.BoolVar(&fromStdin, "stdin", false, "also read newline-separated spec ids from stdin (bulk form)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

const specEpicUsage = "usage: vector spec epic <spec-id> <epic-id> | vector spec epic <spec-id> --clear | vector spec epic --epic <epic-id> <spec-id>... [--stdin] | vector spec epic --clear <spec-id>... [--stdin]"

// runSingleEpicAssign is the legacy one-spec form, with its original output.
func runSingleEpicAssign(store *state.Store, specID, epicID string, jsonOut bool) error {
	changed, err := store.AssignEpic(specID, epicID, resolveActor(), time.Now())
	if err != nil {
		return err
	}
	if jsonOut {
		return printJSONValue(map[string]any{"id": specID, "epic": epicID, "changed": changed})
	}
	switch {
	case !changed && epicID == "":
		fmt.Printf("spec %q has no epic (no change)\n", specID)
	case !changed:
		fmt.Printf("spec %q already belongs to epic %q (no change)\n", specID, epicID)
	case epicID == "":
		fmt.Printf("cleared the epic of spec %q\n", specID)
	default:
		fmt.Printf("assigned spec %q → epic %q\n", specID, epicID)
	}
	return nil
}

// epicBulkSpecJSON is one spec of the bulk `spec epic --json` result.
type epicBulkSpecJSON struct {
	ID       string `json:"id"`
	Changed  bool   `json:"changed"`
	Previous string `json:"previous,omitempty"`
}

// epicBulkJSON is the bulk `spec epic --json` result: the target epic ("" when
// clearing), per-spec outcomes in input order, and the changed/unchanged counts.
type epicBulkJSON struct {
	Epic      string             `json:"epic"`
	Specs     []epicBulkSpecJSON `json:"specs"`
	Changed   int                `json:"changed"`
	Unchanged int                `json:"unchanged"`
}

func printEpicBulkResults(epicID string, results []state.EpicAssignResult, jsonOut bool) error {
	out := epicBulkJSON{Epic: epicID, Specs: make([]epicBulkSpecJSON, 0, len(results))}
	for _, result := range results {
		out.Specs = append(out.Specs, epicBulkSpecJSON{ID: result.ID, Changed: result.Changed, Previous: result.Previous})
		if result.Changed {
			out.Changed++
		} else {
			out.Unchanged++
		}
	}
	if jsonOut {
		return printJSONValue(out)
	}
	target := "epic " + strconv.Quote(epicID)
	if epicID == "" {
		target = "no epic"
	}
	fmt.Printf("%d spec(s) → %s (%d changed, %d unchanged)\n", len(results), target, out.Changed, out.Unchanged)
	for _, result := range out.Specs {
		outcome := "unchanged"
		if result.Changed {
			outcome = "changed"
			if result.Previous != "" {
				outcome += " (was " + result.Previous + ")"
			}
		}
		fmt.Printf("  %-40s %s\n", result.ID, outcome)
	}
	return nil
}

// readIDLines reads newline-separated ids, skipping blank lines and # comments.
func readIDLines(reader io.Reader) ([]string, error) {
	ids := make([]string, 0)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids = append(ids, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read spec ids from stdin: %w", err)
	}
	return ids, nil
}
