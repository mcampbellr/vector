package main

import (
	"testing"

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
