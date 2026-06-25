// Package board derives the read-only board projection that the web panel
// consumes: spec cards grouped into status columns, plus the Token Savings
// Meter rolled up from the local activity log (the agent-routing wedge). It is
// a pure projection of state owned by package state — it never writes.
package board

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

// SchemaVersion guards the public board contract consumed by web/.
const SchemaVersion = 1

// Board is the full projection served at GET /api/board. The web frontend owns
// no canonical state; this is the single shape it renders.
type Board struct {
	SchemaVersion int          `json:"schemaVersion"`
	Repo          string       `json:"repo"`
	GeneratedAt   time.Time    `json:"generatedAt"`
	UpdatedAt     time.Time    `json:"updatedAt"` // latest spec mutation → board freshness
	Columns       []Column     `json:"columns"`
	TokenSavings  TokenSavings `json:"tokenSavings"`
	Totals        Totals       `json:"totals"`
}

// Column is one status lane (single-axis board: column == lifecycle status).
type Column struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Cards  []Card `json:"cards"`
	Count  int    `json:"count"`
}

// Card is a spec projected for display. Token economics churn per run and are
// personal, so SavedUSD is derived from the activity log, not stored on state.
type Card struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	Priority        string     `json:"priority"`
	Repo            string     `json:"repo,omitempty"`
	Stage           string     `json:"stage,omitempty"`
	Assignee        string     `json:"assignee,omitempty"`
	Labels          []string   `json:"labels,omitempty"`
	EstimateMin     int        `json:"estimateMinutes,omitempty"`
	Ticket          *Ticket    `json:"ticket,omitempty"`
	HasOpenSpec     bool       `json:"hasOpenSpec"`
	Artifacts       *Artifacts `json:"artifacts,omitempty"`
	AttentionReason string     `json:"attentionReason,omitempty"`
	SavedUSD        float64    `json:"savedUsd"`
	Routes          int        `json:"routes"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// Ticket mirrors the linked tracker (subset of state.Ticket for display).
type Ticket struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	URL      string `json:"url"`
}

// Artifacts mirrors which OpenSpec artifacts a card's change has.
type Artifacts struct {
	Proposal bool `json:"proposal"`
	Design   bool `json:"design"`
	Tasks    bool `json:"tasks"`
}

// TokenSavings is the differentiator: the cost saved by routing trivial work to
// cheap agents, rolled up from agent.routed events.
type TokenSavings struct {
	TotalSavedUSD float64       `json:"totalSavedUsd"`
	TotalSpentUSD float64       `json:"totalSpentUsd"`
	BaselineUSD   float64       `json:"baselineUsd"` // what the baseline model would have cost
	Routes        int           `json:"routes"`
	TokensIn      int           `json:"tokensIn"`
	TokensOut     int           `json:"tokensOut"`
	ByModel       []ModelRollup `json:"byModel"`
}

// ModelRollup breaks savings down by the cheap model that handled the work.
type ModelRollup struct {
	Model    string  `json:"model"`
	Baseline string  `json:"baseline"`
	Routes   int     `json:"routes"`
	SavedUSD float64 `json:"savedUsd"`
}

// Totals are board-wide counters.
type Totals struct {
	Specs int `json:"specs"`
}

// columnOrder is the canonical single-axis lane order. Archived lives in a
// separate view (docs/domain-contract.md) and is excluded from the board.
var columnOrder = []state.Status{
	state.StatusDraft,
	state.StatusOpen,
	state.StatusInProgress,
	state.StatusNeedsAttention,
	state.StatusReview,
	state.StatusClosed,
}

var columnLabels = map[state.Status]string{
	state.StatusDraft:          "Draft",
	state.StatusOpen:           "Open",
	state.StatusInProgress:     "In progress",
	state.StatusNeedsAttention: "Needs attention",
	state.StatusReview:         "Review",
	state.StatusClosed:         "Closed",
}

// priorityRank orders cards within a column (urgent first).
var priorityRank = map[state.Priority]int{
	state.PriorityUrgent: 0,
	state.PriorityHigh:   1,
	state.PriorityNormal: 2,
	state.PriorityLow:    3,
}

// Source is what Build reads from — satisfied by *state.Store.
type Source interface {
	ListSpecs() ([]*state.SpecState, error)
	ReadEvents() ([]state.Event, error)
}

// Build projects the store into a Board. repo labels the board (the repo name).
func Build(src Source, repo string, now time.Time) (*Board, error) {
	specs, err := src.ListSpecs()
	if err != nil {
		return nil, err
	}
	events, err := src.ReadEvents()
	if err != nil {
		return nil, err
	}

	savings, perSpec := rollupSavings(events)

	byStatus := make(map[state.Status][]Card, len(columnOrder))
	var latest time.Time
	for _, spec := range specs {
		if spec.Status == state.StatusArchived {
			continue // archived has its own view
		}
		card := toCard(spec, perSpec[spec.ID])
		byStatus[spec.Status] = append(byStatus[spec.Status], card)
		if spec.UpdatedAt.After(latest) {
			latest = spec.UpdatedAt
		}
	}

	columns := make([]Column, 0, len(columnOrder))
	for _, status := range columnOrder {
		cards := byStatus[status]
		if cards == nil {
			cards = []Card{} // serialize as [] not null (stable web contract)
		}
		sortCards(cards)
		columns = append(columns, Column{
			Status: string(status),
			Label:  columnLabels[status],
			Cards:  cards,
			Count:  len(cards),
		})
	}

	return &Board{
		SchemaVersion: SchemaVersion,
		Repo:          repo,
		GeneratedAt:   now.UTC(),
		UpdatedAt:     latest.UTC(),
		Columns:       columns,
		TokenSavings:  savings,
		Totals:        Totals{Specs: len(specs)},
	}, nil
}

func toCard(spec *state.SpecState, econ specEconomics) Card {
	card := Card{
		ID:          spec.ID,
		Title:       spec.Title,
		Status:      string(spec.Status),
		Priority:    string(spec.Priority),
		Repo:        spec.Repo,
		Stage:       spec.Stage,
		Assignee:    spec.Assignee,
		Labels:      spec.Labels,
		EstimateMin: spec.EstimateMin,
		HasOpenSpec: spec.OpenSpec != nil,
		SavedUSD:    econ.savedUSD,
		Routes:      econ.routes,
		UpdatedAt:   spec.UpdatedAt.UTC(),
	}
	if spec.Ticket != nil {
		card.Ticket = &Ticket{Provider: string(spec.Ticket.Provider), Key: spec.Ticket.Key, URL: spec.Ticket.URL}
	}
	if spec.OpenSpec != nil {
		card.Artifacts = &Artifacts{
			Proposal: spec.OpenSpec.Artifacts.Proposal,
			Design:   spec.OpenSpec.Artifacts.Design,
			Tasks:    spec.OpenSpec.Artifacts.Tasks,
		}
	}
	if spec.Flag != nil {
		card.AttentionReason = spec.Flag.Reason
	}
	return card
}

// sortCards orders by priority, then most-recently-updated first.
func sortCards(cards []Card) {
	sort.SliceStable(cards, func(i, j int) bool {
		ri, rj := priorityRank[state.Priority(cards[i].Priority)], priorityRank[state.Priority(cards[j].Priority)]
		if ri != rj {
			return ri < rj
		}
		return cards[i].UpdatedAt.After(cards[j].UpdatedAt)
	})
}

type specEconomics struct {
	savedUSD float64
	routes   int
}

// rollupSavings folds agent.routed events into the board-wide Token Savings
// Meter and a per-spec map for card badges.
func rollupSavings(events []state.Event) (TokenSavings, map[string]specEconomics) {
	perSpec := make(map[string]specEconomics)
	byModel := make(map[string]*ModelRollup)
	var s TokenSavings

	for _, e := range events {
		if e.Type != state.EvtAgentRouted {
			continue
		}
		var d state.AgentRoutedData
		if err := json.Unmarshal(e.Data, &d); err != nil {
			continue
		}
		s.Routes++
		s.TotalSavedUSD += d.SavedUSD
		s.TotalSpentUSD += d.CostUSD
		s.TokensIn += d.TokensIn
		s.TokensOut += d.TokensOut

		if e.SpecID != "" {
			econ := perSpec[e.SpecID]
			econ.savedUSD += d.SavedUSD
			econ.routes++
			perSpec[e.SpecID] = econ
		}

		key := d.Model + "→" + d.Baseline
		m := byModel[key]
		if m == nil {
			m = &ModelRollup{Model: d.Model, Baseline: d.Baseline}
			byModel[key] = m
		}
		m.Routes++
		m.SavedUSD += d.SavedUSD
	}

	s.BaselineUSD = s.TotalSpentUSD + s.TotalSavedUSD
	s.ByModel = make([]ModelRollup, 0, len(byModel))
	for _, m := range byModel {
		s.ByModel = append(s.ByModel, *m)
	}
	sort.Slice(s.ByModel, func(i, j int) bool {
		return s.ByModel[i].SavedUSD > s.ByModel[j].SavedUSD
	})
	return s, perSpec
}
