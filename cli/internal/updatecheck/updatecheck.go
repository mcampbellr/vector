// Package updatecheck answers "is a newer Vector release available?" for released
// builds. It owns a global (per-user, not per-repo) cache under
// os.UserCacheDir()/vector, the GitHub Releases client and a hand-rolled semver
// comparator. Check reads the cache and, at most once per cacheTTL, refreshes it
// inline under a short timeout (syncRefreshTimeout). The refresh is synchronous
// on purpose: most vector commands exit in milliseconds, so a background
// goroutine would be killed before GitHub answers and the cache would never
// fill. Every failure degrades to silence.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DevVersion is the version string of an unreleased (locally built) binary. It
// disables the check entirely: no disk, no network.
const DevVersion = "dev"

// cacheTTL is how long a cached latest tag stays fresh (≤1 network check/day).
const cacheTTL = 24 * time.Hour

// cacheFileName is the cache file under the vector user-cache directory.
const cacheFileName = "update-check.json"

// syncRefreshTimeout bounds the inline refresh, so a stale cache costs at most
// this much latency once per cacheTTL (offline included: failures are stamped).
const syncRefreshTimeout = time.Second

// Result is the outcome of a check. Current and Latest are normalized with a
// leading "v" regardless of how the binary version was stamped.
type Result struct {
	Available bool
	Current   string
	Latest    string
}

// cacheEntry is the on-disk cache shape. An empty LatestTag records a failed
// check: it keeps the once-per-TTL cadence without claiming a release.
type cacheEntry struct {
	CheckedAt time.Time `json:"checkedAt"`
	LatestTag string    `json:"latestTag,omitempty"`
}

// Checker performs the cached check. Its fields are injectable for tests; the
// package-level Check uses a Checker rooted at the user cache directory and the
// public GitHub API.
type Checker struct {
	// CacheDir holds update-check.json. Empty disables caching (and therefore the
	// check, since results only ever come from the cache).
	CacheDir string
	// FetchLatestTag resolves the newest published release tag.
	FetchLatestTag func(ctx context.Context) (string, error)
	// Now returns the current time (freshness comparison and cache stamp).
	Now func() time.Time
	// RefreshTimeout bounds the inline refresh (default syncRefreshTimeout).
	RefreshTimeout time.Duration
}

// Check runs the default, user-scoped check for the given binary version. It
// returns nil for "dev", when no release tag is known (never fetched, or every
// fetch failed), or when the cache directory cannot be resolved.
func Check(current string) *Result {
	if current == DevVersion {
		return nil
	}
	return defaultChecker().Check(current)
}

// defaultChecker builds the production Checker. An unresolvable user cache dir
// yields an empty CacheDir, which turns the check into a silent no-op.
func defaultChecker() *Checker {
	dir, err := cacheDir()
	if err != nil {
		dir = ""
	}
	return &Checker{
		CacheDir: dir,
		FetchLatestTag: func(ctx context.Context) (string, error) {
			release, err := FetchLatestRelease(ctx)
			if err != nil {
				return "", err
			}
			return release.TagName, nil
		},
		Now:            time.Now,
		RefreshTimeout: syncRefreshTimeout,
	}
}

// cacheDir resolves os.UserCacheDir()/vector.
func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}
	return filepath.Join(base, "vector"), nil
}

// Check returns the best available result. A stale, missing or corrupt cache is
// refreshed inline first, bounded by RefreshTimeout.
func (checker *Checker) Check(current string) *Result {
	if current == DevVersion || checker.CacheDir == "" {
		return nil
	}
	entry, found := checker.readCache()
	if !found || checker.now().Sub(entry.CheckedAt) >= cacheTTL {
		entry = checker.refresh(entry)
	}
	if entry.LatestTag == "" {
		return nil
	}
	return &Result{
		Available: IsNewer(current, entry.LatestTag),
		Current:   NormalizeTag(current),
		Latest:    NormalizeTag(entry.LatestTag),
	}
}

// now returns the injected clock or time.Now.
func (checker *Checker) now() time.Time {
	if checker.Now != nil {
		return checker.Now()
	}
	return time.Now()
}

// cachePath is the absolute path of the cache file.
func (checker *Checker) cachePath() string {
	return filepath.Join(checker.CacheDir, cacheFileName)
}

// readCache returns the cached entry; a missing, unreadable or corrupt file is
// reported as not found. An entry with an empty LatestTag (a stamped failure) is
// found.
func (checker *Checker) readCache() (cacheEntry, bool) {
	raw, err := os.ReadFile(checker.cachePath())
	if err != nil {
		return cacheEntry{}, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil || entry.CheckedAt.IsZero() {
		return cacheEntry{}, false
	}
	return entry, true
}

// refresh fetches the latest tag within RefreshTimeout and stamps the cache.
// On failure the previous tag (possibly empty) is kept but re-stamped, so an
// offline machine pays the timeout at most once per cacheTTL. A write failure
// is ignored: the fetched value is still used for this invocation.
func (checker *Checker) refresh(previous cacheEntry) cacheEntry {
	next := cacheEntry{CheckedAt: checker.now().UTC(), LatestTag: previous.LatestTag}
	if checker.FetchLatestTag != nil {
		timeout := checker.RefreshTimeout
		if timeout <= 0 {
			timeout = syncRefreshTimeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		tag, err := checker.FetchLatestTag(ctx)
		cancel()
		if err == nil && tag != "" {
			next.LatestTag = tag
		}
	}
	_ = writeJSONAtomic(checker.cachePath(), next)
	return next
}

// writeJSONAtomic marshals v and writes it via a temp file + rename in the same
// directory, so concurrent vector processes never observe a partial cache (same
// idiom as internal/intel, deliberately not imported: that cache is per-repo).
func writeJSONAtomic(targetPath string, v any) error {
	payload, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(targetPath), err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(payload, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}
	return nil
}
