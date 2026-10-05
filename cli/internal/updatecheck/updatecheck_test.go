package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fixedNow is the clock every test pins; values are synthetic, not real releases.
var fixedNow = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

func writeCache(t *testing.T, dir string, entry cacheEntry) {
	t.Helper()
	if err := writeJSONAtomic(filepath.Join(dir, cacheFileName), entry); err != nil {
		t.Fatal(err)
	}
}

func readCacheFile(t *testing.T, dir string) cacheEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, cacheFileName))
	if err != nil {
		t.Fatal(err)
	}
	var entry cacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

// countingFetcher returns tag and counts calls.
func countingFetcher(tag string, calls *atomic.Int32) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		calls.Add(1)
		return tag, nil
	}
}

func TestCheckFreshCacheNoNetwork(t *testing.T) {
	dir := t.TempDir()
	writeCache(t, dir, cacheEntry{CheckedAt: fixedNow.Add(-time.Hour), LatestTag: "v1.3.0"})
	var calls atomic.Int32
	checker := &Checker{CacheDir: dir, FetchLatestTag: countingFetcher("v9.0.0", &calls), Now: func() time.Time { return fixedNow }}

	result := checker.Check("1.2.0")
	if calls.Load() != 0 {
		t.Errorf("fresh cache must not hit the network, got %d calls", calls.Load())
	}
	want := Result{Available: true, Current: "v1.2.0", Latest: "v1.3.0"}
	if result == nil || *result != want {
		t.Errorf("result = %+v, want %+v", result, want)
	}
}

func TestCheckNotAvailableWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	writeCache(t, dir, cacheEntry{CheckedAt: fixedNow, LatestTag: "v1.2.0"})
	checker := &Checker{CacheDir: dir, Now: func() time.Time { return fixedNow }}
	result := checker.Check("v1.2.0")
	if result == nil || result.Available {
		t.Errorf("result = %+v, want not available", result)
	}
}

func TestCheckStaleCacheRefreshesInline(t *testing.T) {
	dir := t.TempDir()
	writeCache(t, dir, cacheEntry{CheckedAt: fixedNow.Add(-25 * time.Hour), LatestTag: "v1.3.0"})
	var calls atomic.Int32
	checker := &Checker{CacheDir: dir, FetchLatestTag: countingFetcher("v1.4.0", &calls), Now: func() time.Time { return fixedNow }}

	result := checker.Check("v1.2.0")
	if calls.Load() != 1 {
		t.Fatalf("stale cache must refresh once, calls = %d", calls.Load())
	}
	want := Result{Available: true, Current: "v1.2.0", Latest: "v1.4.0"}
	if result == nil || *result != want {
		t.Errorf("result = %+v, want the refreshed %+v", result, want)
	}
	entry := readCacheFile(t, dir)
	if entry.LatestTag != "v1.4.0" || !entry.CheckedAt.Equal(fixedNow) {
		t.Errorf("refreshed cache = %+v, want v1.4.0 at %v", entry, fixedNow)
	}
}

func TestCheckRefreshIsBoundedByTimeout(t *testing.T) {
	dir := t.TempDir()
	writeCache(t, dir, cacheEntry{CheckedAt: fixedNow.Add(-25 * time.Hour), LatestTag: "v1.3.0"})
	checker := &Checker{
		CacheDir: dir,
		Now:      func() time.Time { return fixedNow },
		FetchLatestTag: func(ctx context.Context) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
		RefreshTimeout: 50 * time.Millisecond,
	}

	started := time.Now()
	result := checker.Check("v1.2.0")
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Check blocked %v, want it bounded by RefreshTimeout", elapsed)
	}
	if result == nil || result.Latest != "v1.3.0" || !result.Available {
		t.Errorf("result = %+v, want the cached v1.3.0 after a timed-out refresh", result)
	}
}

func TestCheckMissingCacheRefreshesInline(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "vector")
	var calls atomic.Int32
	checker := &Checker{CacheDir: dir, FetchLatestTag: countingFetcher("v1.5.0", &calls), Now: func() time.Time { return fixedNow }}
	if result := checker.Check("v1.2.0"); result == nil || result.Latest != "v1.5.0" {
		t.Errorf("result = %+v, want the freshly fetched v1.5.0", result)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
	if entry := readCacheFile(t, dir); entry.LatestTag != "v1.5.0" {
		t.Errorf("cache = %+v, want v1.5.0", entry)
	}
}

func TestCheckCorruptCacheTreatedAsMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, cacheFileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	checker := &Checker{CacheDir: dir, FetchLatestTag: countingFetcher("v1.5.0", &calls), Now: func() time.Time { return fixedNow }}
	if result := checker.Check("v1.2.0"); result == nil || result.Latest != "v1.5.0" {
		t.Errorf("result = %+v, want the refreshed v1.5.0 for a corrupt cache", result)
	}
	if calls.Load() != 1 {
		t.Errorf("corrupt cache must trigger a refresh, calls = %d", calls.Load())
	}
}

func TestCheckFetchErrorKeepsTagAndRestamps(t *testing.T) {
	dir := t.TempDir()
	writeCache(t, dir, cacheEntry{CheckedAt: fixedNow.Add(-48 * time.Hour), LatestTag: "v1.3.0"})
	checker := &Checker{
		CacheDir:       dir,
		Now:            func() time.Time { return fixedNow },
		FetchLatestTag: func(context.Context) (string, error) { return "", errors.New("offline") },
	}
	checker.Check("v1.2.0")
	if entry := readCacheFile(t, dir); entry.LatestTag != "v1.3.0" || !entry.CheckedAt.Equal(fixedNow) {
		t.Errorf("cache after a failed refresh = %+v, want v1.3.0 re-stamped at %v", entry, fixedNow)
	}
}

func TestCheckOfflineWithoutCacheRetriesOncePerTTL(t *testing.T) {
	dir := t.TempDir()
	var calls atomic.Int32
	now := fixedNow
	checker := &Checker{
		CacheDir: dir,
		Now:      func() time.Time { return now },
		FetchLatestTag: func(context.Context) (string, error) {
			calls.Add(1)
			return "", errors.New("offline")
		},
	}
	for range 3 {
		if result := checker.Check("v1.2.0"); result != nil {
			t.Fatalf("result = %+v, want nil while offline", result)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("offline calls within the TTL = %d, want 1", calls.Load())
	}
	now = fixedNow.Add(cacheTTL)
	checker.Check("v1.2.0")
	if calls.Load() != 2 {
		t.Errorf("calls after the TTL = %d, want 2", calls.Load())
	}
}

func TestCheckDevSkipsAllIO(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "never-created")
	var calls atomic.Int32
	checker := &Checker{CacheDir: dir, FetchLatestTag: countingFetcher("v1.5.0", &calls)}
	if result := checker.Check(DevVersion); result != nil {
		t.Errorf("dev result = %+v, want nil", result)
	}
	if calls.Load() != 0 {
		t.Errorf("dev must not fetch, calls = %d", calls.Load())
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dev must not touch the cache dir, stat err = %v", err)
	}
	if result := Check(DevVersion); result != nil {
		t.Errorf("package Check(dev) = %+v, want nil", result)
	}
}

func TestCheckConcurrentProcessesNeverCorruptCache(t *testing.T) {
	dir := t.TempDir()
	var calls atomic.Int32
	var wg sync.WaitGroup
	for index := 0; index < 16; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checker := &Checker{CacheDir: dir, FetchLatestTag: countingFetcher("v1.5.0", &calls), Now: func() time.Time { return fixedNow }}
			checker.Check("v1.2.0")
		}()
	}
	wg.Wait()
	if entry := readCacheFile(t, dir); entry.LatestTag != "v1.5.0" {
		t.Errorf("cache = %+v, want v1.5.0", entry)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}
