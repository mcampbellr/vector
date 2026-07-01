// Package ui is Vector's terminal presentation layer — the analog of
// internal/webui but for the CLI. It wraps lipgloss styles behind small helpers
// (Bold/Green/Red/Dim/Cyan, Success/Info/Warning/Error, Table, KeyValue) that the
// human output branch of cmd/vector uses. It is applied ONLY in the human branch:
// no ui.* call ever sits inside an `if jsonOut` branch, so the --json contract
// stays byte-identical and machine-consumable. lipgloss auto-degrades to plain
// text under NO_COLOR, TERM=dumb, or a non-TTY stdout — defense in depth on top of
// that hard rule.
//
// The palette reuses flagify's hex tokens as a documented placeholder (no Vector
// brand palette is recorded in the repo yet — Open question #1 of the change
// adopt-cobra-lipgloss-cli); swapping the five colors below is the whole change if
// a brand palette lands later.
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	cyan   = lipgloss.Color("#00D4FF")
	green  = lipgloss.Color("#00CC88")
	red    = lipgloss.Color("#FF6B6B")
	yellow = lipgloss.Color("#FFCC00")
	dim    = lipgloss.Color("#666666")

	boldStyle    = lipgloss.NewStyle().Bold(true)
	labelStyle   = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	successStyle = lipgloss.NewStyle().Foreground(green)
	dimStyle     = lipgloss.NewStyle().Foreground(dim)
	warnStyle    = lipgloss.NewStyle().Foreground(yellow)
	redStyle     = lipgloss.NewStyle().Foreground(red)
	cyanStyle    = lipgloss.NewStyle().Foreground(cyan)
)

// Bold/Green/Red/Dim/Cyan are the low-level color wrappers.
func Bold(s string) string  { return boldStyle.Render(s) }
func Green(s string) string { return successStyle.Render(s) }
func Red(s string) string   { return redStyle.Render(s) }
func Dim(s string) string   { return dimStyle.Render(s) }
func Cyan(s string) string  { return cyanStyle.Render(s) }

// Success/Info/Warning/Error prefix a message with a status glyph in the matching
// color: ✓ success, ● info, ⚠ warning, ✗ error.
func Success(msg string) string { return fmt.Sprintf("%s %s", successStyle.Render("✓"), msg) }
func Info(msg string) string    { return fmt.Sprintf("%s %s", dimStyle.Render("●"), msg) }
func Warning(msg string) string { return fmt.Sprintf("%s %s", warnStyle.Render("⚠"), msg) }
func Error(msg string) string   { return redStyle.Render("✗ " + msg) }

// Table renders a bordered table with a bold cyan header row, using
// charmbracelet/lipgloss/table.
func Table(headers []string, rows [][]string) string {
	allRows := make([][]string, 0, len(rows))
	allRows = append(allRows, rows...)

	t := table.New().
		Headers(headers...).
		Rows(allRows...).
		BorderStyle(lipgloss.NewStyle().Foreground(dim)).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Foreground(cyan).Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	return t.Render()
}

// KeyValue renders a left-aligned bold-cyan label (padded to a fixed width) next
// to its value, for the human output of context-style key/value listings.
func KeyValue(label, value string) string {
	return fmt.Sprintf("  %s  %s", labelStyle.Render(fmt.Sprintf("%-14s", label)), value)
}
