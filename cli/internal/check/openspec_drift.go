package check

import (
	"fmt"
	"os"
	"sort"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/openspec"
	"github.com/mariocampbell/vector/internal/state"
)

// changeIndex groups every OpenSpec change read across the configured changes
// directories (one per worktree on [branch] layouts) by change name. It is loaded
// once per run so no tasks.md is scanned more than once.
type changeIndex map[string][]openspec.Change

// loadChanges reads every configured changes directory. It returns a nil index
// (no error) when none of the directories exists: a repo without OpenSpec simply
// produces no OpenSpec drift.
func loadChanges(cfg *config.Config, root string) (changeIndex, error) {
	dirs, err := cfg.ChangesDirs(root)
	if err != nil {
		return nil, err
	}
	var index changeIndex
	for _, dir := range dirs {
		if info, err := os.Stat(dir.Dir); err != nil || !info.IsDir() {
			continue
		}
		if index == nil {
			index = changeIndex{}
		}
		changes, err := openspec.ReadChangesAt(dir.Dir, root)
		if err != nil {
			return nil, err
		}
		for _, change := range changes {
			change.Branch = dir.Branch
			index[change.Name] = append(index[change.Name], change)
		}
	}
	return index, nil
}

// active returns the canonical non-archived copy of a change: the one read from a
// worktree named after the change, else the lexically-smallest branch — the same
// deterministic preference `vector sync` applies.
func (idx changeIndex) active(name string) (openspec.Change, bool) {
	var best openspec.Change
	found := false
	for _, change := range idx[name] {
		if change.Archived {
			continue
		}
		if !found || moreCanonical(change, best) {
			best, found = change, true
		}
	}
	return best, found
}

func moreCanonical(candidate, current openspec.Change) bool {
	candidateNamed, currentNamed := candidate.Branch == candidate.Name, current.Branch == current.Name
	if candidateNamed != currentNamed {
		return candidateNamed
	}
	return candidate.Branch < current.Branch
}

// checkOpenSpecDrift compares the changes tree against the cards. An active change
// no card references (in any status, archived included) is
// openspec-change-without-card; a non-archived card whose change exists nowhere
// (active or archived) is card-without-change.
func checkOpenSpecDrift(report *CheckReport, specs []*state.SpecState, changes changeIndex) {
	referenced := make(map[string]bool, len(specs))
	for _, spec := range specs {
		referenced[spec.ID] = true
		if spec.OpenSpec != nil && spec.OpenSpec.Change != "" {
			referenced[spec.OpenSpec.Change] = true
		}
	}

	names := make([]string, 0, len(changes))
	for name := range changes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		change, ok := changes.active(name)
		if !ok || referenced[name] {
			continue
		}
		report.add(Discrepancy{
			SpecID:      name,
			Type:        TypeOpenSpecChangeWithoutCard,
			Severity:    SeverityMedium,
			Description: fmt.Sprintf("OpenSpec change %s has no card on the board", change.Dir),
			Suggestion:  "vector sync",
		})
	}

	for _, spec := range specs {
		if spec.Status == state.StatusArchived || spec.OpenSpec == nil || spec.OpenSpec.Change == "" {
			continue
		}
		if len(changes[spec.OpenSpec.Change]) > 0 {
			continue
		}
		report.add(Discrepancy{
			SpecID:      spec.ID,
			Type:        TypeCardWithoutChange,
			Severity:    SeverityLow,
			Description: fmt.Sprintf("card references OpenSpec change %q, which exists in no configured changes directory (active or archived); restore it or, if abandoned, close the card", spec.OpenSpec.Change),
			Suggestion:  fmt.Sprintf("vector spec close %s --resolution obsolete", spec.ID),
		})
	}
}
