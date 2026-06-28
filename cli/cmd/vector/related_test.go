package main

import (
	"testing"

	"github.com/mariocampbell/vector/internal/state"
)

func TestParseRelatedFlag(t *testing.T) {
	t.Run("empty yields nil", func(t *testing.T) {
		items, err := parseRelatedFlag("")
		if err != nil || items != nil {
			t.Fatalf("parseRelatedFlag(\"\") = (%+v, %v), want (nil, nil)", items, err)
		}
	})

	t.Run("valid array normalizes and defaults source", func(t *testing.T) {
		items, err := parseRelatedFlag(`[{"kind":"SPEC","ref":" add-login ","source":"blame"},{"kind":"ticket","ref":"jira:ACME-7"}]`)
		if err != nil {
			t.Fatalf("parseRelatedFlag: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("len = %d, want 2", len(items))
		}
		if items[0].Kind != state.RelatedSpec || items[0].Ref != "add-login" || items[0].Source != state.RelatedBlame {
			t.Errorf("item[0] = %+v", items[0])
		}
		if items[1].Source != state.RelatedManual {
			t.Errorf("item[1] source = %q, want manual (defaulted)", items[1].Source)
		}
	})

	t.Run("invalid input errors", func(t *testing.T) {
		cases := map[string]string{
			"malformed json": `[{"kind":`,
			"bad kind":       `[{"kind":"commit","ref":"x"}]`,
			"empty ref":      `[{"kind":"spec","ref":"  "}]`,
			"bad source":     `[{"kind":"spec","ref":"x","source":"robot"}]`,
		}
		for name, raw := range cases {
			if _, err := parseRelatedFlag(raw); err == nil {
				t.Errorf("%s: expected error for %q", name, raw)
			}
		}
	})
}

func TestParseRelateFlags(t *testing.T) {
	item, err := parseRelateFlags("ticket", "jira:ACME-1", "")
	if err != nil {
		t.Fatalf("parseRelateFlags: %v", err)
	}
	if item.Kind != state.RelatedTicket || item.Ref != "jira:ACME-1" || item.Source != state.RelatedManual {
		t.Errorf("item = %+v", item)
	}

	for _, bad := range []struct{ kind, ref, source string }{
		{"commit", "x", "manual"},
		{"spec", "", "manual"},
		{"spec", "x", "robot"},
	} {
		if _, err := parseRelateFlags(bad.kind, bad.ref, bad.source); err == nil {
			t.Errorf("expected error for %+v", bad)
		}
	}
}
