package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/state"
	"github.com/spf13/cobra"
)

// epicColorHelp is the --color flag help, listing the fixed palette.
func epicColorHelp() string {
	names := make([]string, 0, len(state.EpicColors))
	for _, color := range state.EpicColors {
		names = append(names, string(color))
	}
	return "chip color token: " + strings.Join(names, "|")
}

// newEpicCmd is the `epic` parent command: epics group specs under a larger
// initiative (e.g. "App Mobile"). Like `spec`, it keeps an explicit RunE so a bare
// `vector epic` returns a usage error (exit 1) instead of help + exit 0.
func newEpicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "create and manage epics that group specs",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("usage: vector epic <create|list|show|update|focus|unfocus|delete> ...")
		},
	}
	cmd.AddCommand(
		newEpicCreateCmd(),
		newEpicListCmd(),
		newEpicShowCmd(),
		newEpicUpdateCmd(),
		newEpicFocusCmd(),
		newEpicUnfocusCmd(),
		newEpicDeleteCmd(),
	)
	return cmd
}

// epicJSON is the --json shape of one epic: the persisted fields plus its
// membership roll-up (see state.EpicCounts: total/done exclude dropped specs,
// counted in dropped; byStatus covers every member). order/focus/dropped are
// additive and present only when set.
type epicJSON struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Color       string         `json:"color,omitempty"`
	Order       int            `json:"order,omitempty"`
	Focus       bool           `json:"focus,omitempty"`
	Total       int            `json:"total"`
	Done        int            `json:"done"`
	Dropped     int            `json:"dropped,omitempty"`
	ByStatus    map[string]int `json:"byStatus"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// epicMemberJSON is one spec listed under `vector epic show --json`. focus is
// the spec's own marker; focusInherited marks focus derived from the epic.
type epicMemberJSON struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	Priority       string `json:"priority"`
	Focus          bool   `json:"focus,omitempty"`
	FocusInherited bool   `json:"focusInherited,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
}

func toEpicJSON(epic *state.Epic, counts state.EpicCounts) epicJSON {
	byStatus := make(map[string]int, len(counts.ByStatus))
	for status, count := range counts.ByStatus {
		byStatus[string(status)] = count
	}
	return epicJSON{
		ID:          epic.ID,
		Title:       epic.Title,
		Description: epic.Description,
		Color:       string(epic.Color),
		Order:       epic.Order,
		Focus:       epic.Focus,
		Total:       counts.Total,
		Done:        counts.Done,
		Dropped:     counts.Dropped,
		ByStatus:    byStatus,
		CreatedAt:   epic.CreatedAt.UTC(),
		UpdatedAt:   epic.UpdatedAt.UTC(),
	}
}

// epicProgress renders "done/total done" (plus dropped, when any) for the
// human output.
func epicProgress(counts state.EpicCounts) string {
	progress := fmt.Sprintf("%d/%d done", counts.Done, counts.Total)
	if counts.Dropped > 0 {
		progress += fmt.Sprintf(", %d dropped", counts.Dropped)
	}
	return progress
}

// epicOrderLabel renders the order column of `epic list` ("#1", or "-" unset).
func epicOrderLabel(order int) string {
	if order <= 0 {
		return "-"
	}
	return fmt.Sprintf("#%d", order)
}

// orderFlagHelp documents --order on create/update.
const orderFlagHelp = "display/selection order (1 = first); 0 unsets it (unordered epics sort last, by title)"

func newEpicCreateCmd() *cobra.Command {
	var (
		title       string
		id          string
		description string
		color       string
		order       int
		repoRoot    string
		jsonOut     bool
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "create an epic",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(title) == "" {
				return errors.New("--title is required")
			}
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			epic, err := store.CreateEpic(state.CreateEpicParams{
				Title:       title,
				ID:          id,
				Description: description,
				Color:       state.EpicColor(color),
				Order:       order,
				Actor:       resolveActor(),
				Now:         time.Now(),
			})
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSONValue(toEpicJSON(epic, state.EpicCounts{}))
			}
			fmt.Printf("created epic %q (%s)\n  assign specs with: vector spec epic --epic %s <spec-id>...\n", epic.ID, epic.Title, epic.ID)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "epic title (required)")
	f.StringVar(&id, "id", "", "epic id (kebab-case); derived from the title if empty")
	f.StringVar(&description, "description", "", "optional description")
	f.StringVar(&color, "color", "", epicColorHelp())
	f.IntVar(&order, "order", 0, orderFlagHelp)
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

func newEpicListCmd() *cobra.Command {
	var (
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list epics with their progress",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			epics, err := store.ListEpics()
			if err != nil {
				return err
			}
			specs, err := store.ListSpecs()
			if err != nil {
				return err
			}
			counts := state.CountEpicSpecs(specs)
			if jsonOut {
				out := make([]epicJSON, 0, len(epics))
				for _, epic := range epics {
					out = append(out, toEpicJSON(epic, counts[epic.ID]))
				}
				return printJSONValue(out)
			}
			if len(epics) == 0 {
				fmt.Println("no epics (create one with: vector epic create --title \"...\")")
				return nil
			}
			// Epics are already in display order (order, then title); "*" marks a
			// focused epic.
			for _, epic := range epics {
				fmt.Printf("%s %-4s %-32s %-14s %s\n", focusMarker(epic.Focus), epicOrderLabel(epic.Order), epic.ID, epicProgress(counts[epic.ID]), epic.Title)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit epics as a JSON array for tooling")
	return cmd
}

func newEpicShowCmd() *cobra.Command {
	var (
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "show an epic and its specs",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			epic, err := store.GetEpic(args[0])
			if err != nil {
				return err
			}
			specs, err := store.ListSpecs()
			if err != nil {
				return err
			}
			epicIndex := state.NewEpicIndex([]*state.Epic{epic})
			members := make([]epicMemberJSON, 0)
			for _, spec := range specs {
				if spec.Epic == epic.ID {
					members = append(members, epicMemberJSON{
						ID: spec.ID, Title: spec.Title, Status: string(spec.Status),
						Priority: string(spec.Priority), Focus: spec.Focus,
						FocusInherited: epicIndex.InheritsFocus(spec),
						Resolution:     string(spec.Resolution),
					})
				}
			}
			counts := state.CountEpicSpecs(specs)[epic.ID]
			if jsonOut {
				return printJSONValue(struct {
					epicJSON
					Specs []epicMemberJSON `json:"specs"`
				}{epicJSON: toEpicJSON(epic, counts), Specs: members})
			}
			focusNote := ""
			if epic.Focus {
				focusNote = " · focused"
			}
			fmt.Printf("%s — %s (%s)%s\n", epic.ID, epic.Title, epicProgress(counts), focusNote)
			if epic.Description != "" {
				fmt.Printf("  %s\n", epic.Description)
			}
			if len(members) == 0 {
				fmt.Printf("  no specs yet (assign some with: vector spec epic --epic %s <spec-id>...)\n", epic.ID)
				return nil
			}
			for _, member := range members {
				marker := focusMarker(member.Focus)
				if member.FocusInherited {
					marker = "~"
				}
				resolutionSuffix := ""
				if member.Resolution != "" && member.Resolution != string(state.ResolutionDone) {
					resolutionSuffix = "  [" + member.Resolution + "]"
				}
				fmt.Printf("  %s %-40s %-16s %-8s %s%s\n", marker, member.ID, member.Status, member.Priority, member.Title, resolutionSuffix)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

func newEpicUpdateCmd() *cobra.Command {
	var (
		title       string
		description string
		color       string
		order       int
		repoRoot    string
		jsonOut     bool
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "change an epic's title, description, color or order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Only flags the user actually passed are applied; `--color ""` and
			// `--description ""` clear the field.
			var params state.UpdateEpicParams
			flags := cmd.Flags()
			if flags.Changed("title") {
				params.Title = &title
			}
			if flags.Changed("description") {
				params.Description = &description
			}
			if flags.Changed("color") {
				epicColor := state.EpicColor(color)
				params.Color = &epicColor
			}
			if flags.Changed("order") {
				params.Order = &order
			}
			if params.Title == nil && params.Description == nil && params.Color == nil && params.Order == nil {
				return errors.New("nothing to update: pass --title, --description, --color and/or --order")
			}
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			epic, changed, err := store.UpdateEpic(args[0], params, resolveActor(), time.Now())
			if err != nil {
				return err
			}
			if jsonOut {
				specs, listErr := store.ListSpecs()
				if listErr != nil {
					return listErr
				}
				return printJSONValue(toEpicJSON(epic, state.CountEpicSpecs(specs)[epic.ID]))
			}
			if !changed {
				fmt.Printf("epic %q unchanged\n", epic.ID)
				return nil
			}
			fmt.Printf("updated epic %q (%s)\n", epic.ID, epic.Title)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "new title")
	f.StringVar(&description, "description", "", "new description (\"\" clears it)")
	f.StringVar(&color, "color", "", epicColorHelp()+" (\"\" clears it)")
	f.IntVar(&order, "order", 0, orderFlagHelp)
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

// newEpicFocusCmd / newEpicUnfocusCmd toggle an epic's focus marker. Every
// non-terminal spec of a focused epic inherits focus (derived, never copied onto
// the specs), so it sorts first in its column and in `vector spec next`.
func newEpicFocusCmd() *cobra.Command {
	return newEpicFocusToggleCmd("focus", "focus an epic: its open work inherits focus", true)
}

func newEpicUnfocusCmd() *cobra.Command {
	return newEpicFocusToggleCmd("unfocus", "remove an epic's focus (individually focused specs stay focused)", false)
}

func newEpicFocusToggleCmd(name, short string, focus bool) *cobra.Command {
	var (
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   name + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			id := args[0]
			changed, err := store.SetEpicFocus(id, focus, resolveActor(), time.Now())
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSONValue(map[string]any{"id": id, "focus": focus, "changed": changed})
			}
			switch {
			case !changed && focus:
				fmt.Printf("epic %q is already focused (no change)\n", id)
			case !changed:
				fmt.Printf("epic %q is not focused (no change)\n", id)
			case focus:
				fmt.Printf("focused epic %q — its open specs now sort first in their column and in `vector spec next`\n", id)
			default:
				fmt.Printf("unfocused epic %q (individually focused specs keep their focus)\n", id)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

func newEpicDeleteCmd() *cobra.Command {
	var (
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "delete an epic that no spec references",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			if err := store.DeleteEpic(args[0], resolveActor(), time.Now()); err != nil {
				return err
			}
			if jsonOut {
				return printJSONValue(map[string]any{"id": args[0], "deleted": true})
			}
			fmt.Printf("deleted epic %q\n", args[0])
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

// focusMarker is the one-column focus flag of the human list outputs.
func focusMarker(focus bool) string {
	if focus {
		return "*"
	}
	return " "
}
