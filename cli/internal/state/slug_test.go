package state

import "testing"

func TestSlug(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "New checkout flow", "new-checkout-flow"},
		{"already-slug", "new-patient-expediente", "new-patient-expediente"},
		{"trim-and-collapse", "  Add   SEO   Best Practices!! ", "add-seo-best-practices"},
		{"punctuation", "HTML/CSS Markup", "html-css-markup"},
		{"leading-trailing", "--Foo Bar--", "foo-bar"},
		{"empty", "   ", ""},
		{"only-symbols", "@#$%", ""},
		{"mixed-case-numbers", "MH-1438 Resident Docs", "mh-1438-resident-docs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slug(tc.in); got != tc.want {
				t.Errorf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSlugTruncatesAtMaxLen(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "ab "
	}
	got := Slug(long)
	if len(got) > maxSlugLen {
		t.Fatalf("slug length = %d, want <= %d", len(got), maxSlugLen)
	}
	if got == "" {
		t.Fatal("expected non-empty slug")
	}
	if got[len(got)-1] == '-' {
		t.Errorf("slug should not end with a dash: %q", got)
	}
}
