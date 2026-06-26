package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mariocampbell/vector/internal/openspec"
	"github.com/mariocampbell/vector/internal/state"
)

// Ticket-key shapes per tracker. Jira/Linear keys are PROJECT-123; GitHub issue
// and pull URLs carry owner/repo/<number>.
var (
	jiraKeyRe = regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)
	// Linear issue URLs look like /issue/ENG-123/slug; its keys are case-insensitive.
	linearKeyRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*-\d+`)
	ghIssueRe   = regexp.MustCompile(`github\.com/([^/\s]+)/([^/\s]+)/(?:issues|pull)/(\d+)`)
	ticketURLRe = regexp.MustCompile(`https?://[^\s)\]>"']+`)
)

// inferProvider identifies the tracker behind a ref by URL host: jira (Atlassian),
// linear, or github. A URL on an unrecognized host resolves to TicketOther. A
// bare key with no host is ambiguous — ok is false and the caller must supply an
// explicit provider.
func inferProvider(ref string) (state.TicketProvider, bool) {
	host := refHost(ref)
	if host == "" {
		return "", false
	}
	switch {
	case strings.Contains(host, "atlassian.net") || strings.Contains(host, "jira"):
		return state.TicketJira, true
	case strings.Contains(host, "linear.app"):
		return state.TicketLinear, true
	case strings.Contains(host, "github.com"):
		return state.TicketGitHub, true
	default:
		return state.TicketOther, true
	}
}

// parseRef turns a user-supplied reference into a Ticket. ref may be a full URL
// (provider inferred from the host, key extracted from the path), a
// "<provider>:<key>" shorthand, or a bare key. forced, when non-empty, pins the
// provider and is required for a bare key (there is no host to infer from). The
// URL may be left empty (a bare key with no derivable canonical URL) — the board
// then shows the key without a link. An empty/unparseable ref, an unknown forced
// provider, or a key that cannot be extracted from a URL is an error.
func parseRef(ref, forced string) (state.Ticket, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return state.Ticket{}, errors.New("empty ticket ref")
	}

	var forcedProvider state.TicketProvider
	if strings.TrimSpace(forced) != "" {
		forcedProvider = state.TicketProvider(strings.ToLower(strings.TrimSpace(forced)))
		if !validProvider(forcedProvider) {
			return state.Ticket{}, fmt.Errorf("invalid provider %q: allowed jira,linear,github,other", forced)
		}
	}

	// URL form: infer the provider (unless forced) and extract the key.
	if refHost(ref) != "" {
		provider := forcedProvider
		if provider == "" {
			provider, _ = inferProvider(ref)
		}
		key := extractKey(provider, ref)
		if key == "" {
			return state.Ticket{}, fmt.Errorf("could not extract a ticket key from URL %q (pass it as <provider>:<key>)", ref)
		}
		return state.Ticket{Provider: provider, Key: key, URL: normalizeURL(ref)}, nil
	}

	// Shorthand "<provider>:<key>" (e.g. jira:ACME-12).
	if provider, key, ok := splitShorthand(ref); ok {
		if forcedProvider != "" && forcedProvider != provider {
			return state.Ticket{}, fmt.Errorf("ref provider %q conflicts with provider %q", provider, forcedProvider)
		}
		return state.Ticket{Provider: provider, Key: key}, nil
	}

	// Bare key — there is no host to infer from, so a provider must be forced.
	if forcedProvider == "" {
		return state.Ticket{}, fmt.Errorf("ambiguous ticket ref %q: pass an explicit provider (jira|linear|github|other) — no URL to infer from", ref)
	}
	return state.Ticket{Provider: forcedProvider, Key: ref}, nil
}

// detectTicket best-effort detects a ticket linked from an OpenSpec change's
// artifacts, for auto-linking during `vector sync`. Precedence: a `ticket:`
// frontmatter key (a full URL or a "<provider>:<key>" shorthand) wins; otherwise
// a conservative prose scan looks for a single recognized-tracker ticket URL;
// otherwise, ONLY when defaultProvider is set, a context scan resolves a bare key
// anchored to a ticket cue word or carrying a known project prefix (see
// ticketFromContext); otherwise, as a last resort, the ticket key carried by the
// change's worktree folder name (branchKey, supplied by the caller from
// config.WorktreeTicketKeys) is linked against the default provider. Anything
// ambiguous — no match, conflicting matches, or a value that needs a provider it
// cannot infer — yields nil (auto-detection never guesses). An artifact match always
// wins over branchKey (the change's own docs are the more explicit signal). The
// returned ticket carries Auto:true.
func detectTicket(change openspec.Change, root string, defaultProvider state.TicketProvider, keyPrefixes []string, branchKey string) *state.Ticket {
	artifacts := []string{"proposal.md", "design.md", "tasks.md"}
	read := func(name string) (string, bool) {
		b, err := os.ReadFile(filepath.Join(root, change.Dir, name))
		if err != nil {
			return "", false
		}
		return string(b), true
	}

	for _, name := range artifacts {
		if content, ok := read(name); ok {
			if t := ticketFromFrontmatter(content); t != nil {
				t.Auto = true
				return t
			}
		}
	}
	for _, name := range artifacts {
		if content, ok := read(name); ok {
			if t := ticketFromProse(content); t != nil {
				t.Auto = true
				return t
			}
		}
	}
	if defaultProvider == "" {
		return nil // bare-key fallback is opt-in via defaultTicketProvider
	}
	for _, name := range artifacts {
		if content, ok := read(name); ok {
			if t := ticketFromContext(content, defaultProvider, keyPrefixes); t != nil {
				return t
			}
		}
	}
	// Last resort: the change's worktree folder name carried a ticket key. The
	// artifact scans above always win; this only fires when nothing in the change's
	// own docs matched. Gated on the default provider (we are already past the
	// defaultProvider == "" guard) and the same ADR/RFC denylist.
	if branchKey != "" && !denylistedKey(branchKey) {
		return &state.Ticket{Provider: defaultProvider, Key: branchKey, URL: "", Auto: true}
	}
	return nil
}

var (
	frontmatterRe = regexp.MustCompile(`(?s)\A\s*---\s*\n(.*?)\n---`)
	ticketLineRe  = regexp.MustCompile(`(?m)^ticket:\s*(.+?)\s*$`)
)

// ticketFromFrontmatter extracts a ticket from a `ticket:` key in the document's
// leading YAML frontmatter. A bare key (no provider to infer) is treated as
// ambiguous and ignored — frontmatter must carry a URL or a <provider>:<key>.
func ticketFromFrontmatter(content string) *state.Ticket {
	fm := frontmatterRe.FindStringSubmatch(content)
	if fm == nil {
		return nil
	}
	line := ticketLineRe.FindStringSubmatch(fm[1])
	if line == nil {
		return nil
	}
	value := strings.Trim(strings.TrimSpace(line[1]), `"'`)
	if value == "" {
		return nil
	}
	t, err := parseRef(value, "")
	if err != nil {
		return nil
	}
	return &t
}

// ticketFromProse scans free text for ticket URLs, returning one only when the
// artifacts reference a single recognized tracker (jira/linear/github). Unknown
// hosts and conflicting multiple tickets yield nil — conservative by design.
func ticketFromProse(content string) *state.Ticket {
	var found *state.Ticket
	for _, raw := range ticketURLRe.FindAllString(content, -1) {
		provider, ok := inferProvider(raw)
		if !ok || provider == state.TicketOther {
			continue
		}
		t, err := parseRef(raw, "")
		if err != nil {
			continue
		}
		if found == nil {
			found = &t
			continue
		}
		if found.Provider != t.Provider || found.Key != t.Key {
			return nil // ambiguous: more than one distinct ticket
		}
	}
	return found
}

var (
	// ticketCueRe matches a ticket cue word at line start — tolerating a blockquote
	// (>) and bold (**…**) around it — and captures the rest of the line. Cues:
	// Ticket / Issue / Ref / Tracking, or a provider name. NOT Epic/Story/Sprint.
	ticketCueRe = regexp.MustCompile(`(?mi)^\s*>?\s*\*{0,2}\s*(?:ticket|issue|ref|tracking|jira|linear|github)\s*\*{0,2}\s*:\s*\*{0,2}\s*(.+)$`)
	// bareKeyRe matches a tracker key like MH-1592 or ENG-7 (case-insensitive shape).
	bareKeyRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*-\d+`)
)

// ticketKeyDenylist holds key prefixes that are documentation conventions, never
// tickets. Matched case-insensitively on the part before the first hyphen.
var ticketKeyDenylist = map[string]bool{"ADR": true, "RFC": true}

// ticketFromContext resolves a bare key (no URL) to a ticket using the configured
// default provider, by two deterministic signals, in order:
//  1. a key anchored to a ticket cue word at line start (the first key after the
//     cue; the cue line's Epic/Story/etc. are ignored) — the strongest signal;
//  2. a key carrying a known project prefix (keyPrefixes) anywhere in prose.
//
// Keys whose prefix is in the denylist (ADR, RFC) are skipped. Each signal yields
// nil on conflicting distinct keys; (1) wins over (2). Returns Auto:true.
func ticketFromContext(content string, provider state.TicketProvider, keyPrefixes []string) *state.Ticket {
	var cueKeys []string
	for _, m := range ticketCueRe.FindAllStringSubmatch(content, -1) {
		cueKeys = append(cueKeys, bareKeyRe.FindString(m[1]))
	}
	switch key, conflict := pickSingleKey(cueKeys); {
	case conflict:
		return nil
	case key != "":
		return &state.Ticket{Provider: provider, Key: key, Auto: true}
	}

	if re := knownPrefixRe(keyPrefixes); re != nil {
		if key, conflict := pickSingleKey(re.FindAllString(content, -1)); !conflict && key != "" {
			return &state.Ticket{Provider: provider, Key: key, Auto: true}
		}
	}
	return nil
}

// pickSingleKey returns the single distinct non-denylisted key in keys. conflict
// is true when two different keys appear; key is "" when none qualify.
func pickSingleKey(keys []string) (key string, conflict bool) {
	for _, k := range keys {
		if k == "" || denylistedKey(k) {
			continue
		}
		switch {
		case key == "":
			key = k
		case key != k:
			return "", true
		}
	}
	return key, false
}

// denylistedKey reports whether key's prefix (before the first hyphen) is a known
// non-ticket convention (ADR, RFC).
func denylistedKey(key string) bool {
	i := strings.IndexByte(key, '-')
	if i <= 0 {
		return false
	}
	return ticketKeyDenylist[strings.ToUpper(key[:i])]
}

// knownPrefixRe builds a case-insensitive regex matching a key whose prefix is one
// of the configured project prefixes (e.g. MH-1592). Returns nil when none given.
func knownPrefixRe(prefixes []string) *regexp.Regexp {
	parts := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, regexp.QuoteMeta(p))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(parts, "|") + `)-\d+\b`)
}

// parseTicketFlag decodes the --ticket JSON ({provider,key,url,auto}) passed by
// /vector:raw when it detects a ticket in the raw idea text. An empty flag yields
// (nil, nil) — no ticket. The provider and key are required and the provider must
// be known; the URL is optional.
func parseTicketFlag(raw string) (*state.Ticket, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var t state.Ticket
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return nil, fmt.Errorf("parse --ticket JSON: %w", err)
	}
	t.Provider = state.TicketProvider(strings.ToLower(strings.TrimSpace(string(t.Provider))))
	t.Key = strings.TrimSpace(t.Key)
	t.URL = normalizeURL(t.URL)
	if !validProvider(t.Provider) {
		return nil, fmt.Errorf("invalid --ticket provider %q: allowed jira,linear,github,other", t.Provider)
	}
	if t.Key == "" {
		return nil, errors.New("--ticket requires a non-empty key")
	}
	return &t, nil
}

// refHost returns the lowercased host of a URL ref, or "" when ref is not a URL.
func refHost(ref string) string {
	if !strings.Contains(ref, "://") {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Host)
}

// splitShorthand parses a "<provider>:<key>" ref. The prefix must be a known
// tracker, so a bare key with a stray colon is not mistaken for a provider.
func splitShorthand(ref string) (state.TicketProvider, string, bool) {
	i := strings.IndexByte(ref, ':')
	if i <= 0 {
		return "", "", false
	}
	provider := state.TicketProvider(strings.ToLower(ref[:i]))
	key := strings.TrimSpace(ref[i+1:])
	if key == "" || !validProvider(provider) {
		return "", "", false
	}
	return provider, key, true
}

// extractKey pulls the ticket key out of a URL for a known provider, falling
// back to the last path segment for unrecognized shapes. "" when none is found.
func extractKey(provider state.TicketProvider, ref string) string {
	switch provider {
	case state.TicketGitHub:
		if m := ghIssueRe.FindStringSubmatch(ref); m != nil {
			return fmt.Sprintf("%s/%s#%s", m[1], m[2], m[3])
		}
	case state.TicketJira:
		if m := jiraKeyRe.FindString(ref); m != "" {
			return m
		}
	case state.TicketLinear:
		if m := linearKeyRe.FindString(ref); m != "" {
			return m
		}
	}
	return lastPathSegment(ref)
}

// lastPathSegment returns the final non-empty path component of a URL.
func lastPathSegment(ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	p := strings.Trim(u.Path, "/")
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	return parts[len(parts)-1]
}

// normalizeURL trims surrounding whitespace and a trailing slash from a URL ref.
func normalizeURL(ref string) string {
	return strings.TrimRight(strings.TrimSpace(ref), "/")
}

// validProvider reports whether p is a known tracker provider.
func validProvider(p state.TicketProvider) bool {
	switch p {
	case state.TicketJira, state.TicketLinear, state.TicketGitHub, state.TicketOther:
		return true
	}
	return false
}
