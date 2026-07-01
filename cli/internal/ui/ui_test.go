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
