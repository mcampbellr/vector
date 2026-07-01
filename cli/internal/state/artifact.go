package state

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ReadSpecArtifact resolves an artifact key for a spec to an on-disk path from
// committed state, defends it against escaping the repo root, and returns the
// file bytes. It is read-only and the only seam the board's /api/file handler
// uses to serve a spec's source documents.
//
// The key maps as: "spec" → the spec's SpecDoc; "proposal"/"design"/"tasks" →
// openspec/changes/<change>/<key>.md, gated by OpenSpec being set and the
// matching Artifacts flag. A missing spec, an unset artifact flag, or a missing
// file all surface as a wrapped fs.ErrNotExist (the handler maps these to 404);
// a path that escapes the repo root or the allowed prefixes is a non-not-exist
// error (mapped to 500). The client never sends a path: callers pass a spec id
// and an artifact enum, so traversal is removed by design — the prefix check is
// defense in depth over already-trusted committed state.
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
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact %q for spec %q: %w", artifact, specID, fs.ErrNotExist)
		}
		return nil, err
	}
	return b, nil
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
