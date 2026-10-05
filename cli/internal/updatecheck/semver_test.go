package updatecheck

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		tag                 string
		major, minor, patch int
		pre                 string
		ok                  bool
	}{
		{"v1.2.3", 1, 2, 3, "", true},
		{"1.2.3", 1, 2, 3, "", true},
		{"v10.0.12", 10, 0, 12, "", true},
		{"v1.2.3-rc.1", 1, 2, 3, "rc.1", true},
		{"v1.2.3+build.7", 1, 2, 3, "", true},
		{"v1.2.3-", 0, 0, 0, "", false},
		{"dev", 0, 0, 0, "", false},
		{"", 0, 0, 0, "", false},
		{"v1.2", 0, 0, 0, "", false},
		{"v1.2.3.4", 0, 0, 0, "", false},
		{"v1.x.3", 0, 0, 0, "", false},
		{"v-1.2.3", 0, 0, 0, "", false},
		{"v1..3", 0, 0, 0, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			major, minor, patch, pre, ok := ParseVersion(tc.tag)
			if ok != tc.ok || major != tc.major || minor != tc.minor || patch != tc.patch || pre != tc.pre {
				t.Errorf("ParseVersion(%q) = %d.%d.%d pre=%q ok=%v, want %d.%d.%d pre=%q ok=%v",
					tc.tag, major, minor, patch, pre, ok, tc.major, tc.minor, tc.patch, tc.pre, tc.ok)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name    string
		a, b    string
		wantCmp int
		wantOK  bool
	}{
		{"major newer", "v2.0.0", "v1.9.9", 1, true},
		{"minor older", "v1.1.0", "v1.2.0", -1, true},
		{"patch newer", "v1.2.4", "v1.2.3", 1, true},
		{"equal with and without prefix", "1.2.3", "v1.2.3", 0, true},
		{"numeric not lexical", "v1.10.0", "v1.9.0", 1, true},
		{"pre-release on left", "v1.3.0-rc.1", "v1.2.0", 0, false},
		{"pre-release on right", "v1.2.0", "v1.3.0-beta", 0, false},
		{"dev", "dev", "v1.2.0", 0, false},
		{"invalid", "v1.2", "v1.2.0", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmp, ok := CompareVersions(tc.a, tc.b)
			if cmp != tc.wantCmp || ok != tc.wantOK {
				t.Errorf("CompareVersions(%q, %q) = (%d, %v), want (%d, %v)", tc.a, tc.b, cmp, ok, tc.wantCmp, tc.wantOK)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.1", "v1.0.1", false},
		{"v1.0.2", "v1.0.1", false},
		{"v1.0.0", "v2.0.0-rc.1", false},
		{"dev", "v1.0.0", false},
		{"v1.0.0-snapshot", "v1.0.1", false},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.current, tc.latest); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestNormalizeTag(t *testing.T) {
	cases := map[string]string{"1.2.3": "v1.2.3", "v1.2.3": "v1.2.3", " v1.2.3 ": "v1.2.3", "": ""}
	for input, want := range cases {
		if got := NormalizeTag(input); got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", input, got, want)
		}
	}
}
