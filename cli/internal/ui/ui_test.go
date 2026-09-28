package ui

import (
	"strings"
	"testing"
)

// TestColorWrappersContainInput asserts each low-level wrapper returns non-empty
// text that still contains the input string (styling wraps, never drops content).
func TestColorWrappersContainInput(t *testing.T) {
	const in = "hello"
	for name, got := range map[string]string{
		"Bold":  Bold(in),
		"Green": Green(in),
		"Red":   Red(in),
		"Dim":   Dim(in),
		"Cyan":  Cyan(in),
	} {
		if got == "" {
			t.Errorf("%s(%q) = empty", name, in)
		}
		if !strings.Contains(got, in) {
			t.Errorf("%s(%q) = %q, does not contain input", name, in, got)
		}
	}
}

// TestStatusHelpersContainMessage asserts the status helpers carry their message.
func TestStatusHelpersContainMessage(t *testing.T) {
	const msg = "it worked"
	for name, got := range map[string]string{
		"Success": Success(msg),
		"Info":    Info(msg),
		"Warning": Warning(msg),
		"Error":   Error(msg),
	} {
		if !strings.Contains(got, msg) {
			t.Errorf("%s(%q) = %q, does not contain message", name, msg, got)
		}
	}
}

// TestTableIncludesHeadersAndRows asserts Table renders the header labels and the
// cell values.
func TestTableIncludesHeadersAndRows(t *testing.T) {
	out := Table([]string{"ID", "STATUS"}, [][]string{{"alpha", "open"}, {"beta", "review"}})
	for _, want := range []string{"ID", "STATUS", "alpha", "open", "beta", "review"} {
		if !strings.Contains(out, want) {
			t.Errorf("Table output missing %q:\n%s", want, out)
		}
	}
}

// TestKeyValueContainsBoth asserts KeyValue renders both the label and the value.
func TestKeyValueContainsBoth(t *testing.T) {
	out := KeyValue("language", "es")
	if !strings.Contains(out, "language") || !strings.Contains(out, "es") {
		t.Errorf("KeyValue = %q, want both label and value", out)
	}
}

// TestTitleContainsInput asserts Title wraps, never drops, its input.
func TestTitleContainsInput(t *testing.T) {
	if got := Title("vector"); !strings.Contains(got, "vector") {
		t.Errorf("Title(%q) = %q, does not contain input", "vector", got)
	}
}

// TestWarningBlockIndentsDetails asserts WarningBlock keeps the headline and puts
// each detail on its own line, indented under the message.
func TestWarningBlockIndentsDetails(t *testing.T) {
	out := WarningBlock("port busy", "first detail", "  nested detail")
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("WarningBlock produced %d lines, want 3:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "port busy") {
		t.Errorf("headline line = %q, want it to contain the headline", lines[0])
	}
	if lines[1] != "  first detail" {
		t.Errorf("detail line = %q, want %q", lines[1], "  first detail")
	}
	if lines[2] != "    nested detail" {
		t.Errorf("nested detail line = %q, want %q", lines[2], "    nested detail")
	}
}
