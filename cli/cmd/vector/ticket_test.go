package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/config"
	"github.com/mariocampbell/vector/internal/openspec"
	"github.com/mariocampbell/vector/internal/state"
)

func TestInferProvider(t *testing.T) {
	cases := []struct {
		ref    string
		want   state.TicketProvider
		wantOK bool
	}{
		{"https://acme.atlassian.net/browse/ACME-1", state.TicketJira, true},
		{"https://linear.app/acme/issue/ENG-7/title", state.TicketLinear, true},
		{"https://github.com/acme/api/issues/3", state.TicketGitHub, true},
		{"https://example.com/tickets/42", state.TicketOther, true},
		{"ACME-1", "", false}, // bare key: ambiguous, no host
	}
	for _, tc := range cases {
		got, ok := inferProvider(tc.ref)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("inferProvider(%q) = (%q,%t), want (%q,%t)", tc.ref, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestParseRef(t *testing.T) {
	cases := []struct {
		name     string
		ref      string
		forced   string
		wantErr  bool
		wantProv state.TicketProvider
		wantKey  string
		wantURL  string
	}{
		{
			name: "jira url", ref: "https://acme.atlassian.net/browse/ACME-12",
			wantProv: state.TicketJira, wantKey: "ACME-12", wantURL: "https://acme.atlassian.net/browse/ACME-12",
		},
		{
			name: "linear url with slug", ref: "https://linear.app/acme/issue/ENG-7/some-title",
			wantProv: state.TicketLinear, wantKey: "ENG-7", wantURL: "https://linear.app/acme/issue/ENG-7/some-title",
		},
		{
			name: "github issue url trailing slash", ref: "https://github.com/acme/api/issues/3/",
			wantProv: state.TicketGitHub, wantKey: "acme/api#3", wantURL: "https://github.com/acme/api/issues/3",
		},
		{
			name: "shorthand provider:key", ref: "jira:ACME-99",
			wantProv: state.TicketJira, wantKey: "ACME-99", wantURL: "",
		},
		{
			name: "bare key with forced provider", ref: "ACME-1", forced: "jira",
			wantProv: state.TicketJira, wantKey: "ACME-1", wantURL: "",
		},
		{name: "bare key no provider is ambiguous", ref: "ACME-1", wantErr: true},
		{name: "empty ref", ref: "   ", wantErr: true},
		{name: "invalid forced provider", ref: "ACME-1", forced: "bitbucket", wantErr: true},
		{name: "shorthand conflicts with forced", ref: "jira:ACME-1", forced: "github", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRef(tc.ref, tc.forced)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseRef(%q,%q) = %+v, want error", tc.ref, tc.forced, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRef(%q,%q): %v", tc.ref, tc.forced, err)
			}
			if got.Provider != tc.wantProv || got.Key != tc.wantKey || got.URL != tc.wantURL {
				t.Errorf("parseRef(%q,%q) = %+v, want {%q %q %q}", tc.ref, tc.forced, got, tc.wantProv, tc.wantKey, tc.wantURL)
			}
		})
	}
}

func TestDetectTicket(t *testing.T) {
	root := t.TempDir()

	writeChange := func(name string, files map[string]string) openspec.Change {
		dir := filepath.Join("openspec", "changes", name)
		abs := filepath.Join(root, dir)
		if err := os.MkdirAll(abs, 0o755); err != nil {
			t.Fatal(err)
		}
		for fname, content := range files {
			if err := os.WriteFile(filepath.Join(abs, fname), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return openspec.Change{Name: name, Dir: filepath.ToSlash(dir)}
	}

	t.Run("frontmatter wins", func(t *testing.T) {
		c := writeChange("fm", map[string]string{
			"proposal.md": "---\nticket: https://acme.atlassian.net/browse/ACME-5\n---\n\n# Proposal\nbody\n",
		})
		got := detectTicket(c, root, "", nil, "")
		if got == nil || got.Provider != state.TicketJira || got.Key != "ACME-5" || !got.Auto {
			t.Fatalf("frontmatter detect = %+v", got)
		}
	})

	t.Run("frontmatter shorthand", func(t *testing.T) {
		c := writeChange("fm-short", map[string]string{
			"proposal.md": "---\nticket: linear:ENG-9\n---\n# x\n",
		})
		got := detectTicket(c, root, "", nil, "")
		if got == nil || got.Provider != state.TicketLinear || got.Key != "ENG-9" {
			t.Fatalf("shorthand frontmatter detect = %+v", got)
		}
	})

	t.Run("prose fallback single url", func(t *testing.T) {
		c := writeChange("prose", map[string]string{
			"proposal.md": "# Proposal\n\nImplements https://github.com/acme/api/issues/12 as discussed.\n",
		})
		got := detectTicket(c, root, "", nil, "")
		if got == nil || got.Provider != state.TicketGitHub || got.Key != "acme/api#12" || !got.Auto {
			t.Fatalf("prose detect = %+v", got)
		}
	})

	t.Run("noisy prose without provider is nil", func(t *testing.T) {
		c := writeChange("noise", map[string]string{
			"proposal.md": "# Proposal\n\nSee ticket ACME-1 and https://example.com/wiki/page for context.\n",
			"design.md":   "Nothing trackable here, just ENG-2 mentioned in passing.\n",
		})
		if got := detectTicket(c, root, "", nil, ""); got != nil {
			t.Fatalf("expected nil for ambiguous prose, got %+v", got)
		}
	})

	t.Run("conflicting prose tickets is nil", func(t *testing.T) {
		c := writeChange("conflict", map[string]string{
			"proposal.md": "Links https://github.com/acme/api/issues/1 and https://linear.app/acme/issue/ENG-3/x\n",
		})
		if got := detectTicket(c, root, "", nil, ""); got != nil {
			t.Fatalf("expected nil for conflicting tickets, got %+v", got)
		}
	})

	t.Run("no artifacts is nil", func(t *testing.T) {
		c := openspec.Change{Name: "ghost", Dir: "openspec/changes/ghost"}
		if got := detectTicket(c, root, "", nil, ""); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("cue word with default provider", func(t *testing.T) {
		c := writeChange("cue", map[string]string{
			"proposal.md": "# Proposal\n\nTicket: MH-1592. Implements the thing.\n",
		})
		got := detectTicket(c, root, state.TicketJira, nil, "")
		if got == nil || got.Provider != state.TicketJira || got.Key != "MH-1592" || got.URL != "" || !got.Auto {
			t.Fatalf("cue detect = %+v", got)
		}
		// Same artifact, no default provider configured → no bare-key fallback.
		if got := detectTicket(c, root, "", nil, ""); got != nil {
			t.Fatalf("expected nil without default provider, got %+v", got)
		}
	})

	t.Run("known prefix with default provider", func(t *testing.T) {
		c := writeChange("prefix", map[string]string{
			"proposal.md": "# Proposal\n\nTouches MH-880 in the payment path. See ADR-007 and RFC-3 for rationale.\n",
		})
		got := detectTicket(c, root, state.TicketJira, []string{"MH"}, "")
		if got == nil || got.Key != "MH-880" {
			t.Fatalf("prefix detect = %+v (ADR/RFC must be ignored)", got)
		}
		// Without the prefix configured and no cue, nothing is detected.
		if got := detectTicket(c, root, state.TicketJira, nil, ""); got != nil {
			t.Fatalf("expected nil without prefix and no cue, got %+v", got)
		}
	})

	t.Run("branchKey is the last-resort fallback", func(t *testing.T) {
		c := writeChange("bk", map[string]string{
			"proposal.md": "# Proposal\n\nNothing trackable in the artifacts.\n",
		})
		got := detectTicket(c, root, state.TicketJira, nil, "MH-1592")
		if got == nil || got.Provider != state.TicketJira || got.Key != "MH-1592" || got.URL != "" || !got.Auto {
			t.Fatalf("branchKey fallback = %+v", got)
		}
	})

	t.Run("artifact wins over branchKey", func(t *testing.T) {
		c := writeChange("bk-artifact", map[string]string{
			"proposal.md": "# Proposal\n\nTicket: MH-880 is the real one.\n",
		})
		got := detectTicket(c, root, state.TicketJira, nil, "MH-1592")
		if got == nil || got.Key != "MH-880" {
			t.Fatalf("artifact must win over branchKey, got %+v", got)
		}
	})

	t.Run("denylisted branchKey is nil", func(t *testing.T) {
		c := writeChange("bk-deny", map[string]string{
			"proposal.md": "# Proposal\n\nNothing trackable.\n",
		})
		if got := detectTicket(c, root, state.TicketJira, nil, "ADR-7"); got != nil {
			t.Fatalf("expected nil for denylisted branchKey, got %+v", got)
		}
	})

	t.Run("branchKey without default provider is nil", func(t *testing.T) {
		c := writeChange("bk-noprov", map[string]string{
			"proposal.md": "# Proposal\n\nNothing trackable.\n",
		})
		if got := detectTicket(c, root, "", nil, "MH-1592"); got != nil {
			t.Fatalf("expected nil without default provider, got %+v", got)
		}
	})
}

func TestTicketFromContext(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		prefixes []string
		wantKey  string // "" = expect nil
	}{
		{name: "plain Ticket cue", content: "Ticket: MH-1592.", wantKey: "MH-1592"},
		{name: "bold Ticket cue", content: "**Ticket:** MH-1552", wantKey: "MH-1552"},
		{name: "blockquote cue ignores Epic on line", content: "> Ticket: MH-1611 · Epic MH-1528 · Story 1", wantKey: "MH-1611"},
		{name: "Issue cue", content: "Issue: ENG-7 is the tracker", wantKey: "ENG-7"},
		{name: "Jira provider-name cue", content: "Jira: MH-42", wantKey: "MH-42"},
		{name: "bare key without cue is ignored", content: "see MH-1558 for the gateway", wantKey: ""},
		{name: "denylisted prefix under cue is ignored", content: "Ref: ADR-007 governs this", wantKey: ""},
		{name: "conflicting cue keys is nil", content: "Ticket: MH-1\nTicket: MH-2", wantKey: ""},
		{name: "known prefix anywhere", content: "Touches MH-880 deeply", prefixes: []string{"MH"}, wantKey: "MH-880"},
		{name: "denylist beats prefix scan", content: "ADR-007 only, no ticket", prefixes: []string{"ADR"}, wantKey: ""},
		{name: "conflicting prefix keys is nil", content: "MH-1 and MH-2 both", prefixes: []string{"MH"}, wantKey: ""},
		{name: "cue wins over conflicting prefix matches", content: "Ticket: MH-1611 · Epic MH-1528", prefixes: []string{"MH"}, wantKey: "MH-1611"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ticketFromContext(tc.content, state.TicketJira, tc.prefixes)
			if tc.wantKey == "" {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil || got.Key != tc.wantKey || got.Provider != state.TicketJira || !got.Auto {
				t.Fatalf("ticketFromContext = %+v, want key %q", got, tc.wantKey)
			}
		})
	}
}

func TestTicketFromShorthands(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		wantNil      bool
		wantProvider state.TicketProvider
		wantKey      string
	}{
		{
			name:         "single jira shorthand",
			content:      "fix jira:ACME-12 before release",
			wantProvider: state.TicketJira, wantKey: "ACME-12",
		},
		{
			name:         "single linear shorthand",
			content:      "see linear:ENG-7 for context",
			wantProvider: state.TicketLinear, wantKey: "ENG-7",
		},
		{
			name:         "duplicate identical shorthand returns one",
			content:      "jira:ACME-12 and also jira:ACME-12 again",
			wantProvider: state.TicketJira, wantKey: "ACME-12",
		},
		{
			name:    "two distinct shorthands same provider is nil",
			content: "jira:ACME-12 and jira:ACME-99",
			wantNil: true,
		},
		{
			name:    "two distinct providers is nil",
			content: "jira:ACME-12 and linear:ENG-7",
			wantNil: true,
		},
		{
			name:    "no shorthands is nil",
			content: "just a plain text without any tracker refs",
			wantNil: true,
		},
		{
			name:    "unknown provider does not match",
			content: "bitbucket:ACME-12 is not a known provider",
			wantNil: true,
		},
		{
			name:         "github shorthand",
			content:      "closes github:owner/repo#123",
			wantProvider: state.TicketGitHub, wantKey: "owner/repo#123",
		},
		{
			name:         "other shorthand",
			content:      "tracked at other:TASK-1",
			wantProvider: state.TicketOther, wantKey: "TASK-1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ticketFromShorthands(tc.content)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil || got.Provider != tc.wantProvider || got.Key != tc.wantKey {
				t.Fatalf("ticketFromShorthands(%q) = %+v, want {%q %q}", tc.content, got, tc.wantProvider, tc.wantKey)
			}
			if got.Auto {
				t.Errorf("ticketFromShorthands must not set Auto (caller's responsibility); got Auto=true")
			}
		})
	}
}

func TestDetectTicketFromText(t *testing.T) {
	cases := []struct {
		name            string
		text            string
		defaultProvider state.TicketProvider
		keyPrefixes     []string
		wantNil         bool
		wantProvider    state.TicketProvider
		wantKey         string
		wantAuto        bool
	}{
		// Tier 1: URL of recognized tracker.
		{
			name:         "tier1 jira URL",
			text:         "implements https://acme.atlassian.net/browse/ACME-12",
			wantProvider: state.TicketJira, wantKey: "ACME-12", wantAuto: true,
		},
		{
			name:         "tier1 linear URL",
			text:         "see https://linear.app/acme/issue/ENG-7/title for context",
			wantProvider: state.TicketLinear, wantKey: "ENG-7", wantAuto: true,
		},
		{
			name:    "tier1 unknown host skipped",
			text:    "see https://example.com/tickets/42 for context",
			wantNil: true,
		},
		{
			name:    "tier1 two distinct URLs is nil",
			text:    "https://github.com/acme/api/issues/1 and https://linear.app/acme/issue/ENG-3/x",
			wantNil: true,
		},
		// Tier 2: shorthand.
		{
			name:         "tier2 shorthand",
			text:         "fix jira:ACME-12 now",
			wantProvider: state.TicketJira, wantKey: "ACME-12", wantAuto: true,
		},
		// Tier 1 wins over tier 2 when both present.
		{
			name:         "tier1 wins over tier2",
			text:         "jira:ENG-1 see https://acme.atlassian.net/browse/ACME-12 for details",
			wantProvider: state.TicketJira, wantKey: "ACME-12", wantAuto: true,
		},
		// Tier 3: cue-word bare key with defaultProvider.
		{
			name:            "tier3 cue-word with defaultProvider",
			text:            "Ticket: MH-1592 implements the thing",
			defaultProvider: state.TicketJira,
			wantProvider:    state.TicketJira, wantKey: "MH-1592", wantAuto: true,
		},
		{
			name:    "tier3 cue-word without defaultProvider is nil",
			text:    "Ticket: MH-1592 implements the thing",
			wantNil: true,
		},
		// Tier 4: configured prefix.
		{
			name:            "tier4 prefix configured",
			text:            "see MH-880 in the payment path",
			defaultProvider: state.TicketJira,
			keyPrefixes:     []string{"MH"},
			wantProvider:    state.TicketJira, wantKey: "MH-880", wantAuto: true,
		},
		// Denylist.
		{
			name:            "ADR denylist under cue",
			text:            "Ref: ADR-007 governs this",
			defaultProvider: state.TicketJira,
			wantNil:         true,
		},
		{
			name:            "RFC denylist under cue",
			text:            "Ref: RFC-3 is the standard",
			defaultProvider: state.TicketJira,
			wantNil:         true,
		},
		// Empty text.
		{
			name:    "empty text is nil",
			text:    "",
			wantNil: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectTicketFromText(tc.text, tc.defaultProvider, tc.keyPrefixes)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("detectTicketFromText = nil, want {%q %q auto=%v}", tc.wantProvider, tc.wantKey, tc.wantAuto)
			}
			if got.Provider != tc.wantProvider || got.Key != tc.wantKey || got.Auto != tc.wantAuto {
				t.Errorf("detectTicketFromText = %+v, want provider=%q key=%q auto=%v",
					got, tc.wantProvider, tc.wantKey, tc.wantAuto)
			}
		})
	}
}

func TestRunSpecLink(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "feat", Title: "Feat", Now: time.Now()}); err != nil {
		t.Fatal(err)
	}

	// Ambiguous bare key without --provider is an actionable error.
	if err := runSpecLink([]string{"feat", "ACME-1", "--repo-root", root}); err == nil {
		t.Fatal("expected error for ambiguous bare key without --provider")
	}

	// Success with --json: persists a manual (auto:false) link.
	if err := runSpecLink([]string{"feat", "ACME-1", "--provider", "jira", "--repo-root", root, "--json"}); err != nil {
		t.Fatalf("runSpecLink: %v", err)
	}
	got, err := store.ReadSpec("feat")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ticket == nil || got.Ticket.Provider != state.TicketJira || got.Ticket.Key != "ACME-1" || got.Ticket.Auto {
		t.Fatalf("link not persisted as manual: %+v", got.Ticket)
	}
}

func TestRunSpecLinkUsesDefaultProvider(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "feat", Title: "Feat", Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	// A repo config declaring a default provider lets a bare key link without --provider.
	cfg := config.Resolve(root)
	cfg.DefaultTicketProvider = state.TicketJira
	if err := config.Write(root, cfg); err != nil {
		t.Fatal(err)
	}

	if err := runSpecLink([]string{"feat", "MH-1592", "--repo-root", root, "--json"}); err != nil {
		t.Fatalf("runSpecLink with default provider: %v", err)
	}
	got, err := store.ReadSpec("feat")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ticket == nil || got.Ticket.Provider != state.TicketJira || got.Ticket.Key != "MH-1592" || got.Ticket.Auto {
		t.Fatalf("bare-key link did not use default provider as manual: %+v", got.Ticket)
	}
}
