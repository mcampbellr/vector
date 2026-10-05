package updatecheck

import (
	"strconv"
	"strings"
)

// ParseVersion parses a `vMAJOR.MINOR.PATCH[-pre][+build]` tag. The leading "v"
// is optional and build metadata is ignored (semver §10). A non-empty pre marks a
// pre-release: ok stays true for the parse itself, and callers treat it
// conservatively (CompareVersions refuses to compare it). ok is false for any
// other shape, including "dev".
func ParseVersion(tag string) (major, minor, patch int, pre string, ok bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if buildIndex := strings.IndexByte(trimmed, '+'); buildIndex >= 0 {
		trimmed = trimmed[:buildIndex]
	}
	if preIndex := strings.IndexByte(trimmed, '-'); preIndex >= 0 {
		pre = trimmed[preIndex+1:]
		trimmed = trimmed[:preIndex]
		if pre == "" {
			return 0, 0, 0, "", false
		}
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return 0, 0, 0, "", false
	}
	numbers := make([]int, 0, 3)
	for _, part := range parts {
		if part == "" || strings.TrimLeft(part, "0123456789") != "" {
			return 0, 0, 0, "", false
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return 0, 0, 0, "", false
		}
		numbers = append(numbers, value)
	}
	return numbers[0], numbers[1], numbers[2], pre, true
}

// CompareVersions returns -1, 0 or 1 as a is older than, equal to or newer than
// b. ok is false when either side fails to parse or carries a pre-release —
// pre-releases are never compared (nor offered) in V1.
func CompareVersions(a, b string) (cmp int, ok bool) {
	aMajor, aMinor, aPatch, aPre, aOK := ParseVersion(a)
	bMajor, bMinor, bPatch, bPre, bOK := ParseVersion(b)
	if !aOK || !bOK || aPre != "" || bPre != "" {
		return 0, false
	}
	for _, pair := range [][2]int{{aMajor, bMajor}, {aMinor, bMinor}, {aPatch, bPatch}} {
		switch {
		case pair[0] < pair[1]:
			return -1, true
		case pair[0] > pair[1]:
			return 1, true
		}
	}
	return 0, true
}

// IsNewer reports whether latest is strictly newer than current. It is false
// whenever the comparison is not meaningful (see CompareVersions).
func IsNewer(current, latest string) bool {
	cmp, ok := CompareVersions(latest, current)
	return ok && cmp > 0
}

// NormalizeTag returns tag with a single leading "v" (e.g. "1.2.3" → "v1.2.3").
func NormalizeTag(tag string) string {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return ""
	}
	return "v" + strings.TrimPrefix(trimmed, "v")
}
