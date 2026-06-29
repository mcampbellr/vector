package intel

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// maxWorkspaces caps how many workspaces the structure index records, a guard
// against pathological monorepos (spec §11). When exceeded, the index is
// truncated with an explicit note rather than silently.
const maxWorkspaces = 256

// workspaceKinds maps a manifest base name to the workspace kind it implies.
var workspaceKinds = map[string]string{
	"go.mod":         "go-module",
	"package.json":   "node",
	"pyproject.toml": "python",
	"setup.py":       "python",
	"Cargo.toml":     "rust",
}

// BuildStructureIndex derives structure-index.json: the repo's workspaces
// (directories carrying a manifest, including the root) classified by kind, each
// with its entry points. It uses the bounded filesystem walk (git-independent, so
// it works with or without git), storing only paths — never file contents. A repo
// with no manifests yields an available, empty index.
func BuildStructureIndex(repoRoot string) StructureIndex {
	si := StructureIndex{
		SchemaVersion: CacheSchemaVersion,
		Available:     true,
		Workspaces:    []Workspace{},
		GeneratedAt:   nowStamp(),
	}
	files, err := collectFiles(repoRoot)
	if err != nil && len(files) == 0 {
		si.Available = false
		si.Note = "filesystem walk failed"
		return si
	}

	// Classify each directory by the first recognized manifest it carries.
	kinds := map[string]string{}
	for _, f := range files {
		if kind, ok := workspaceKinds[path.Base(f)]; ok {
			dir := path.Dir(f) // "." for the repo root
			if _, seen := kinds[dir]; !seen {
				kinds[dir] = kind
			}
		}
	}

	dirs := make([]string, 0, len(kinds))
	for dir := range kinds {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	if len(dirs) > maxWorkspaces {
		si.Truncated = true
		si.Note = "workspace count exceeded cap; index truncated"
		dirs = dirs[:maxWorkspaces]
	}

	for _, dir := range dirs {
		si.Workspaces = append(si.Workspaces, Workspace{
			Path:        dir,
			Kind:        kinds[dir],
			EntryPoints: entryPoints(repoRoot, dir, kinds[dir], files),
		})
	}
	return si
}

// entryPoints returns the workspace-relative entry-point paths for a workspace,
// by a minimal per-language heuristic (spec §6, Open question #3): Go uses
// main.go and cmd/*/main.go; Node uses src/index.*, src/main.* plus package.json
// main/bin; Python uses __main__.py. Paths are sorted and de-duplicated.
func entryPoints(repoRoot, dir, kind string, files []string) []string {
	prefix := ""
	if dir != "." {
		prefix = dir + "/"
	}
	set := map[string]bool{}
	matchRel := func(pattern, rel string) bool {
		ok, _ := filepath.Match(pattern, rel)
		return ok
	}
	for _, f := range files {
		if prefix != "" && !strings.HasPrefix(f, prefix) {
			continue
		}
		rel := strings.TrimPrefix(f, prefix)
		switch kind {
		case "go-module":
			if rel == "main.go" || matchRel("cmd/*/main.go", rel) {
				set[rel] = true
			}
		case "node":
			if matchRel("src/index.*", rel) || matchRel("src/main.*", rel) {
				set[rel] = true
			}
		case "python":
			if rel == "__main__.py" || matchRel("*/__main__.py", rel) {
				set[rel] = true
			}
		}
	}
	if kind == "node" {
		for _, m := range nodeMains(repoRoot, dir) {
			set[m] = true
		}
	}

	out := make([]string, 0, len(set))
	for ep := range set {
		out = append(out, ep)
	}
	sort.Strings(out)
	return out
}
