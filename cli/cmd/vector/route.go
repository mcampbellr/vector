package main

import (
	"errors"
	"flag"
	"fmt"
	"time"
)

// runSpecRoute records an agent.routed event — the commercialization wedge that
// feeds the Token Savings Meter. The kit commands call this after routing a step
// to a cheap model (e.g. /vector:raw's Haiku refiner) so the saved cost is
// captured. The binary owns the economics: callers pass the models + token
// counts, and the price table derives cost/saved.
func runSpecRoute(args []string) error {
	id, rest := leadingID(args)
	fs := flag.NewFlagSet("spec route", flag.ContinueOnError)
	idFlag := fs.String("id", "", "spec id this routing belongs to (optional; or pass it as the first argument)")
	model := fs.String("model", "", "cheap model that handled the work (haiku|sonnet|opus|fable, or a claude-* id)")
	baseline := fs.String("baseline", "opus", "baseline model the work would otherwise have used")
	task := fs.String("task", "", "short label for the routed task")
	tokensIn := fs.Int("tokens-in", 0, "input tokens consumed")
	tokensOut := fs.Int("tokens-out", 0, "output tokens produced")
	precision := fs.String("precision", "", "data quality: actual|estimated (default: estimated)")
	repoRoot := fs.String("repo-root", "", "repo root (defaults to git toplevel or cwd)")
	jsonOut := fs.Bool("json", false, "emit a JSON result for tooling")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if id == "" {
		id = *idFlag
	}
	if *model == "" {
		return errors.New("usage: vector spec route [id] --model <m> [--baseline opus] --tokens-in N --tokens-out M [--task ...]")
	}

	store, err := openStore(*repoRoot)
	if err != nil {
		return err
	}
	data, err := store.RouteAgent(id, *task, *model, *baseline, *tokensIn, *tokensOut, *precision, resolveActor(), time.Now())
	if err != nil {
		return err
	}

	if *jsonOut {
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
}
