package state

import (
	"os"
	"testing"
	"time"
)

func TestReadSummariesMissingFileIsEmpty(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	all, err := store.ReadSummaries()
	if err != nil {
		t.Fatalf("ReadSummaries: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected empty map, got %d entries", len(all))
	}
	sum, err := store.ReadSummary("nope")
	if err != nil {
		t.Fatalf("ReadSummary: %v", err)
	}
	if sum != nil {
		t.Errorf("expected nil summary for absent id, got %+v", sum)
	}
}

func TestWriteSummaryRoundTrip(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	now := fixedNow()
	if err := store.WriteSummary("alpha", "did the thing", "apply", now); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}

	sum, err := store.ReadSummary("alpha")
	if err != nil {
		t.Fatalf("ReadSummary: %v", err)
	}
	if sum == nil {
		t.Fatal("expected a summary, got nil")
	}
	if sum.Summary != "did the thing" || sum.Action != "apply" || sum.ID != "alpha" {
		t.Errorf("round-trip mismatch: %+v", sum)
	}
	if sum.SchemaVersion != SummarySchemaVersion {
		t.Errorf("schema version = %d, want %d", sum.SchemaVersion, SummarySchemaVersion)
	}
	if !sum.GeneratedAt.Equal(now.UTC()) {
		t.Errorf("generatedAt = %v, want %v", sum.GeneratedAt, now.UTC())
	}

	all, err := store.ReadSummaries()
	if err != nil {
		t.Fatalf("ReadSummaries: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 entry, got %d", len(all))
	}
}

func TestWriteSummaryLastWriterWinsPerID(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	now := fixedNow()
	if err := store.WriteSummary("alpha", "first", "propose", now); err != nil {
		t.Fatalf("WriteSummary first: %v", err)
	}
	if err := store.WriteSummary("beta", "other spec", "apply", now); err != nil {
		t.Fatalf("WriteSummary beta: %v", err)
	}
	if err := store.WriteSummary("alpha", "second", "apply", now.Add(time.Hour)); err != nil {
		t.Fatalf("WriteSummary second: %v", err)
	}

	alpha, err := store.ReadSummary("alpha")
	if err != nil {
		t.Fatalf("ReadSummary alpha: %v", err)
	}
	if alpha.Summary != "second" || alpha.Action != "apply" {
		t.Errorf("alpha not overwritten: %+v", alpha)
	}
	beta, err := store.ReadSummary("beta")
	if err != nil {
		t.Fatalf("ReadSummary beta: %v", err)
	}
	if beta.Summary != "other spec" {
		t.Errorf("beta clobbered: %+v", beta)
	}
}

func TestWriteSummaryNeverPartialFile(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.WriteSummary("alpha", "prose", "close", fixedNow()); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}
	// The on-disk file must be valid JSON ending in a newline (atomic write).
	b, err := os.ReadFile(store.summariesPath())
	if err != nil {
		t.Fatalf("read summaries file: %v", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Errorf("summaries file not newline-terminated")
	}
	if _, err := store.ReadSummaries(); err != nil {
		t.Errorf("on-disk file did not parse: %v", err)
	}
}
