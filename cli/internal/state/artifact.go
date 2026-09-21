package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ArtifactFallbacks tells ReadSpecArtifact where else to look when an artifact's
// recorded location is gone. In bare+worktree layouts a spec doc and its OpenSpec
// change live inside a per-spec worktree (code/<slug>/openspec/changes/<slug>/)
// that is removed once the branch merges, so the recorded pointer dangles while a
// copy survives elsewhere (the base worktree, the OpenSpec archive).
type ArtifactFallbacks struct {
	// ChangesDirs returns absolute OpenSpec changes roots, searched in order. Each
	// is probed for <change>/<file> and then archive/<date>-<change>/<file>. It is
	// called per lookup so worktrees added while the board runs are seen. Nil
	// disables the changes-dir fallbacks.
	ChangesDirs func() []string
}

// SetArtifactFallbacks configures the extra locations ReadSpecArtifact searches.
// Call it once before the store serves reads (it is not synchronized with them).
func (s *Store) SetArtifactFallbacks(fallbacks ArtifactFallbacks) {
	s.fallbacks = fallbacks
}

// changesDirs resolves the configured changes roots; nil-safe.
func (f ArtifactFallbacks) changesDirs() []string {
	if f.ChangesDirs == nil {
		return nil
	}
	return f.ChangesDirs()
}

// artifactCandidate is one place an artifact may live: a cleaned absolute path
// plus the directory it must stay under (defense in depth over trusted state).
type artifactCandidate struct {
	abs     string
	allowed string
}

// ReadSpecArtifact resolves an artifact key for a spec to an on-disk path from
// committed state, defends it against escaping the repo root, and returns the
// file bytes. It is read-only and the only seam the board's /api/file handler
// uses to serve a spec's source documents.
//
// The key maps as: "spec" → the spec's SpecDoc; "proposal"/"design"/"tasks" →
// openspec/changes/<change>/<key>.md, gated by OpenSpec being set and the
// matching Artifacts flag. When the recorded location is missing, the lookup
// falls back in order to: the durable snapshot under .vector/specs/<id>/ (spec
// doc only), the configured ChangesDirs (live change, then its archive), and the
// composer's .vector/tmp/<id>/spec.md output (spec doc only). A missing spec, an
// unset artifact flag, or no surviving copy surface as a wrapped fs.ErrNotExist
// (the handler maps these to 404); a recorded path that escapes the repo root or
// the allowed prefixes is a non-not-exist error (mapped to 500). The client never
// sends a path: callers pass a spec id and an artifact enum, so traversal is
// removed by design — the prefix check is defense in depth over already-trusted
// committed state.
func (s *Store) ReadSpecArtifact(specID, artifact string) ([]byte, error) {
	spec, err := s.ReadSpec(specID)
	if err != nil {
		return nil, err // ReadSpec wraps a missing state.json as fs.ErrNotExist
	}

	rel, err := artifactRelPath(spec, artifact)
	if err != nil {
		return nil, err
	}

	repoRoot := filepath.Dir(s.root)
	abs := filepath.Clean(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err := verifyArtifactPath(repoRoot, abs, spec); err != nil {
		return nil, err // non-not-exist → 500
	}

	b, err := os.ReadFile(abs)
	if err == nil {
		return b, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	for _, candidate := range s.fallbackCandidates(spec, artifact, rel) {
		if !isUnder(repoRoot, candidate.abs) || !isUnder(candidate.allowed, candidate.abs) {
			continue // a fallback never widens the trusted surface; skip, don't fail
		}
		b, err := os.ReadFile(candidate.abs)
		if err == nil {
			return b, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("artifact %q for spec %q: %w", artifact, specID, fs.ErrNotExist)
}

// fallbackCandidates lists, in priority order, where a copy of the artifact may
// survive once its recorded path is gone. rel is the recorded repo-relative path.
func (s *Store) fallbackCandidates(spec *SpecState, artifact, rel string) []artifactCandidate {
	if artifact == "sketch" {
		return nil // sketches live only in the spec's own .vector shard
	}
	isSpecDoc := artifact == "spec"
	var candidates []artifactCandidate
	if isSpecDoc && s.specDocSnapshotPath(spec) != "" {
		candidates = append(candidates, artifactCandidate{abs: s.bodyPath(spec.ID), allowed: s.specDir(spec.ID)})
	}
	if change := artifactChangeName(spec, isSpecDoc, rel); change != "" {
		fileName := filepath.Base(filepath.FromSlash(rel))
		for _, changesDir := range s.fallbacks.changesDirs() {
			candidates = append(candidates, changeDirCandidates(changesDir, change, fileName)...)
		}
	}
	if isSpecDoc {
		tmpDir := filepath.Join(s.root, "tmp", spec.ID)
		candidates = append(candidates, artifactCandidate{abs: filepath.Join(tmpDir, "spec.md"), allowed: tmpDir})
	}
	return candidates
}

// artifactChangeName is the OpenSpec change folder an artifact belongs to: the
// recorded change, or — for a spec doc without OpenSpec provenance — the folder
// that holds the recorded doc (…/changes/<change>/spec.md). "" when the name is
// not a single safe path segment, so fallbacks never build a traversing path.
func artifactChangeName(spec *SpecState, isSpecDoc bool, rel string) string {
	change := ""
	switch {
	case spec.OpenSpec != nil && spec.OpenSpec.Change != "":
		change = spec.OpenSpec.Change
	case isSpecDoc:
		change = filepath.Base(filepath.Dir(filepath.FromSlash(rel)))
	}
	if change == "" || change == "." || change == ".." || strings.ContainsAny(change, `/\*?[]`) {
		return ""
	}
	return change
}

// changeDirCandidates returns <changesDir>/<change>/<fileName> followed by the
// archived copies (archive/<YYYY-MM-DD>-<change>/<fileName>), newest first.
func changeDirCandidates(changesDir, change, fileName string) []artifactCandidate {
	liveDir := filepath.Join(changesDir, change)
	candidates := []artifactCandidate{{abs: filepath.Join(liveDir, fileName), allowed: liveDir}}

	archiveRoot := filepath.Join(changesDir, "archive")
	matches, err := filepath.Glob(filepath.Join(archiveRoot, "*-"+change))
	if err != nil {
		return candidates // only ErrBadPattern, excluded by artifactChangeName
	}
	// Archive folders are date-prefixed: the lexically-last one is the newest.
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, archivedDir := range matches {
		candidates = append(candidates, artifactCandidate{abs: filepath.Join(archivedDir, fileName), allowed: archivedDir})
	}
	return candidates
}

// specDocSnapshotPath returns the absolute path of a spec's recorded SpecDoc when
// it lives outside the spec's own .vector/specs/<id>/spec.md (the durable
// snapshot location). "" when the spec has no SpecDoc, the SpecDoc already is the
// snapshot location, or the pointer escapes the repo root.
func (s *Store) specDocSnapshotPath(spec *SpecState) string {
	if spec.SpecDoc == "" {
		return ""
	}
	repoRoot := filepath.Dir(s.root)
	docAbs := filepath.Clean(filepath.Join(repoRoot, filepath.FromSlash(spec.SpecDoc)))
	if docAbs == s.bodyPath(spec.ID) || !isUnder(repoRoot, docAbs) {
		return ""
	}
	return docAbs
}

// snapshotSpecDoc copies a spec's SpecDoc into .vector/specs/<id>/spec.md so the
// board can still serve it after the worktree that holds the original is removed.
// A missing source keeps the previous snapshot, and an unchanged source is not
// rewritten. The caller must hold s.mu.
func (s *Store) snapshotSpecDoc(spec *SpecState) error {
	docAbs := s.specDocSnapshotPath(spec)
	if docAbs == "" {
		return nil
	}
	content, err := os.ReadFile(docAbs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read spec doc for snapshot: %w", err)
	}
	snapshotPath := s.bodyPath(spec.ID)
	current, err := os.ReadFile(snapshotPath)
	switch {
	case err == nil && bytes.Equal(current, content):
		return nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("read spec doc snapshot: %w", err)
	}
	if err := writeFileAtomic(snapshotPath, content); err != nil {
		return fmt.Errorf("write spec doc snapshot: %w", err)
	}
	return nil
}

// artifactRelPath maps an artifact key to a repo-relative path, returning a
// wrapped fs.ErrNotExist when the key has no backing artifact for this spec.
func artifactRelPath(spec *SpecState, artifact string) (string, error) {
	switch artifact {
	case "spec":
		if spec.SpecDoc == "" {
			return "", fmt.Errorf("spec %q has no spec doc: %w", spec.ID, fs.ErrNotExist)
		}
		return spec.SpecDoc, nil
	case "proposal", "design", "tasks":
		if spec.OpenSpec == nil || !hasArtifact(spec.OpenSpec.Artifacts, artifact) {
			return "", fmt.Errorf("spec %q has no %s artifact: %w", spec.ID, artifact, fs.ErrNotExist)
		}
		return filepath.ToSlash(filepath.Join("openspec", "changes", spec.OpenSpec.Change, artifact+".md")), nil
	case "sketch":
		// V1 serves the first sketch; it lives under the spec's own
		// .vector/specs/<id>/sketches/ shard, already covered by verifyArtifactPath's
		// allowed prefix. No sketch → fs.ErrNotExist (handler maps to 404).
		if len(spec.Sketches) == 0 {
			return "", fmt.Errorf("spec %q has no sketch: %w", spec.ID, fs.ErrNotExist)
		}
		return filepath.ToSlash(filepath.Join(".vector", "specs", spec.ID, "sketches", spec.Sketches[0].Name)), nil
	default:
		return "", fmt.Errorf("unknown artifact %q: %w", artifact, fs.ErrNotExist)
	}
}

// hasArtifact reports whether the OpenSpec artifact named by key exists.
func hasArtifact(set ArtifactSet, key string) bool {
	switch key {
	case "proposal":
		return set.Proposal
	case "design":
		return set.Design
	case "tasks":
		return set.Tasks
	}
	return false
}

// verifyArtifactPath enforces, as defense in depth, that the resolved absolute
// path stays under the repo root and under one of the expected locations
// (.vector/specs/<id>/, the spec's own SpecDoc, or openspec/changes/<change>/).
// The SpecDoc's own location is allowed because CreateSpec writes the spec body
// there for the convention store — it can live outside .vector/ (e.g. under the
// repo's configured spec-path), and it is already-trusted committed state.
// A violation is a non-fs.ErrNotExist error so the handler maps it to 500, never 404.
func verifyArtifactPath(repoRoot, abs string, spec *SpecState) error {
	if !isUnder(repoRoot, abs) {
		return fmt.Errorf("artifact path %q escapes repo root", abs)
	}
	allowed := []string{filepath.Join(repoRoot, ".vector", "specs", spec.ID)}
	if spec.SpecDoc != "" {
		allowed = append(allowed, filepath.Join(repoRoot, filepath.FromSlash(spec.SpecDoc)))
	}
	if spec.OpenSpec != nil {
		allowed = append(allowed, filepath.Join(repoRoot, "openspec", "changes", spec.OpenSpec.Change))
	}
	for _, prefix := range allowed {
		if isUnder(prefix, abs) {
			return nil
		}
	}
	return fmt.Errorf("artifact path %q outside allowed locations", abs)
}

// isUnder reports whether target is root itself or nested under it. Both are
// expected to be cleaned absolute paths.
func isUnder(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
