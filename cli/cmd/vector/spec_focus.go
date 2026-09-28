package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

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

// newSpecEpicCmd assigns a spec to an epic (`vector spec epic <spec-id> <epic-id>`)
// or clears it (`--clear`), via Store.AssignEpic — metadata only, never a status
// change. The epic must exist; assigning the current epic is a no-op.
func newSpecEpicCmd() *cobra.Command {
	var (
		clearEpic bool
		repoRoot  string
		jsonOut   bool
	)
	cmd := &cobra.Command{
		Use:   "epic <spec-id> [epic-id]",
		Short: "assign a spec to an epic (or --clear it)",
		RunE: func(_ *cobra.Command, args []string) error {
			specID, rest := leadingID(args)
			var epicID string
			if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
				epicID = rest[0]
			}
			if specID == "" || (epicID == "" && !clearEpic) {
				return errors.New("usage: vector spec epic <spec-id> <epic-id> | vector spec epic <spec-id> --clear")
			}
			if epicID != "" && clearEpic {
				return errors.New("pass either an epic id or --clear, not both")
			}
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
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
		},
	}
	f := cmd.Flags()
	f.BoolVar(&clearEpic, "clear", false, "remove the spec from its epic")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}
