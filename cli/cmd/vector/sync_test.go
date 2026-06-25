package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/openspec"
	"github.com/mariocampbell/vector/internal/state"
)

func TestSyncNeedsUAT(t *testing.T) {
	cases := []struct {
		name string
		c    openspec.Change
		want bool
	}{
		{
			name: "only verification tasks left, work started",
			c:    openspec.Change{HasTasks: true, TasksTotal: 5, TasksDone: 4, PendingReal: 0},
			want: true,
		},
		{
			name: "all tasks done (clean review)",
			c:    openspec.Change{HasTasks: true, TasksTotal: 5, TasksDone: 5, PendingReal: 0},
			want: false,
		},
		{
			name: "real implementation work pending",
			c:    openspec.Change{HasTasks: true, TasksTotal: 5, TasksDone: 2, PendingReal: 1},
			want: false,
		},
		{
			name: "nothing done yet (would be open)",
			c:    openspec.Change{HasTasks: true, TasksTotal: 5, TasksDone: 0, PendingReal: 0},
			want: false,
		},
		{
			name: "no tasks",
			c:    openspec.Change{HasTasks: false},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncNeedsUAT(tc.c); got != tc.want {
				t.Errorf("syncNeedsUAT = %v, want %v", got, tc.want)
			}
		})
	}
}

// The UAT flag must agree with syncStatus: it is only ever true when the change
// is in review (so it never marks an open/in-progress card).
func TestSyncNeedsUATImpliesReview(t *testing.T) {
	cases := []openspec.Change{
		{HasTasks: true, TasksTotal: 5, TasksDone: 4, PendingReal: 0},
		{HasTasks: true, TasksTotal: 5, TasksDone: 0, PendingReal: 0},
		{HasTasks: true, TasksTotal: 5, TasksDone: 2, PendingReal: 1},
	}
	for _, c := range cases {
		if syncNeedsUAT(c) && syncStatus(c) != state.StatusReview {
			t.Errorf("needsUAT true but status %q != review for %+v", syncStatus(c), c)
		}
	}
}

// runSync links a ticket from a worktree folder name (the 4th detectTicket
// source) when a default provider is set and the change's own artifacts carry no
// ticket. The change is read from a single-level worktree (code/dev/...) while the
// key lives on a sibling multi-level branch folder (code/feat/MH-1592-payments);
// they associate by exact slug == change name.
func TestRunSyncLinksTicketFromWorktreeName(t *testing.T) {
	root := t.TempDir()

	cfg := &config.Config{
		SchemaVersion:         config.SchemaVersion,
		SpecPath:              config.VectorFallbackSpecPath,
		SpecFilename:          "spec.md",
		SpecStore:             config.StoreVector,
		Source:                config.SourceDefault,
		ChangesPath:           "code/[branch]/openspec/changes",
		DefaultTicketProvider: state.TicketJira,
	}
	if err := config.Write(root, cfg); err != nil {
		t.Fatal(err)
	}

	// The change, read by ChangesDirs from a single-level worktree.
	changeDir := filepath.Join(root, "code", "dev", "openspec", "changes", "payments")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("## Why\nno ticket in here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The ticket key, carried only by a sibling branch folder name.
	if err := os.MkdirAll(filepath.Join(root, "code", "feat", "MH-1592-payments"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runSync([]string{"--repo-root", root}); err != nil {
		t.Fatalf("runSync: %v", err)
	}

	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := store.ReadSpec("payments")
	if err != nil {
		t.Fatalf("ReadSpec: %v", err)
	}
	if spec.Ticket == nil || spec.Ticket.Key != "MH-1592" || spec.Ticket.Provider != state.TicketJira || !spec.Ticket.Auto {
		t.Fatalf("expected auto ticket MH-1592 from worktree name, got %+v", spec.Ticket)
	}
}
