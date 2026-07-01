package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// runSpecRoute records an agent.routed event — the commercialization wedge that
// feeds the Token Savings Meter. The kit commands call this after routing a step
// to a cheap model (e.g. /vector:raw's Haiku refiner) so the saved cost is
// captured. The binary owns the economics: callers pass the models + token
// counts, and the price table derives cost/saved.
func newSpecRouteCmd() *cobra.Command {
	var (
		idFlag    string
		model     string
		baseline  string
		task      string
		tokensIn  int
		tokensOut int
		precision string
		repoRoot  string
		jsonOut   bool
	)
	cmd := &cobra.Command{
		Use:   "route [id]",
		Short: "record an agent.routed event for the Token Savings Meter",
		RunE: func(_ *cobra.Command, args []string) error {
			id, _ := leadingID(args)
			if id == "" {
				id = idFlag
			}
			if model == "" {
				return errors.New("usage: vector spec route [id] --model <m> [--baseline opus] --tokens-in N --tokens-out M [--task ...]")
			}

			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			data, err := store.RouteAgent(id, task, model, baseline, tokensIn, tokensOut, precision, resolveActor(), time.Now())
			if err != nil {
				return err
			}

			if jsonOut {
				return printJSON(map[string]string{
					"model":     data.Model,
					"baseline":  data.Baseline,
					"tokensIn":  fmt.Sprintf("%d", data.TokensIn),
					"tokensOut": fmt.Sprintf("%d", data.TokensOut),
					"costUsd":   fmt.Sprintf("%.6f", data.CostUSD),
					"savedUsd":  fmt.Sprintf("%.6f", data.SavedUSD),
					"precision": data.Precision,
				})
			}
			label := data.Task
			if label == "" {
				label = "agent task"
			}
			fmt.Printf("routed %q → %s (baseline %s): saved $%.4f (cost $%.4f)\n",
				label, data.Model, data.Baseline, data.SavedUSD, data.CostUSD)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&idFlag, "id", "", "spec id this routing belongs to (optional; or pass it as the first argument)")
	f.StringVar(&model, "model", "", "cheap model that handled the work (haiku|sonnet|opus|fable, or a claude-* id)")
	f.StringVar(&baseline, "baseline", "opus", "baseline model the work would otherwise have used")
	f.StringVar(&task, "task", "", "short label for the routed task")
	f.IntVar(&tokensIn, "tokens-in", 0, "input tokens consumed")
	f.IntVar(&tokensOut, "tokens-out", 0, "output tokens produced")
	f.StringVar(&precision, "precision", "", "data quality: actual|estimated (default: estimated)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}
