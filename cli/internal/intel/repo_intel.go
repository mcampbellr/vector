package intel

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// BuildRepoIntel derives repo-intel.json from the repo's manifests and lockfiles:
// the primary runtime, package manager, detected frameworks, and tsconfig paths.
// Heuristic and minimal (V1) — a polyglot repo resolves a single primary runtime
// by precedence (go > node > python > rust); per-workspace detail lives in the
// structure index. It never loads source file contents beyond the manifests it
// needs.
func BuildRepoIntel(repoRoot string) RepoIntel {
	files, _ := collectFiles(repoRoot)
	ri := RepoIntel{
		SchemaVersion: CacheSchemaVersion,
		Frameworks:    []string{},
		TsconfigPaths: []string{},
		GeneratedAt:   nowStamp(),
	}

	hasGo := anyBase(files, "go.mod")
	hasNode := anyBase(files, "package.json")
	hasPy := anyBase(files, "pyproject.toml") || anyBase(files, "setup.py")
	hasRust := anyBase(files, "Cargo.toml")

	switch {
	case hasGo:
		ri.Runtime = Runtime{Name: "go", Version: goVersion(repoRoot, files)}
		ri.PackageManager = "go-modules"
	case hasNode:
		ri.Runtime = Runtime{Name: "node"}
		ri.PackageManager = nodePackageManager(repoRoot, files)
	case hasPy:
		ri.Runtime = Runtime{Name: "python"}
		ri.PackageManager = pythonPackageManager(files)
	case hasRust:
		ri.Runtime = Runtime{Name: "rust"}
		ri.PackageManager = "cargo"
	}

	for _, f := range files {
		base := path.Base(f)
		if ok, _ := filepath.Match("tsconfig*.json", base); ok {
			ri.TsconfigPaths = append(ri.TsconfigPaths, f)
		}
		if ok, _ := filepath.Match("next.config.*", base); ok {
			ri.Frameworks = appendUnique(ri.Frameworks, "next")
		}
		if ok, _ := filepath.Match("vite.config.*", base); ok {
			ri.Frameworks = appendUnique(ri.Frameworks, "vite")
		}
	}
	sort.Strings(ri.TsconfigPaths)
	sort.Strings(ri.Frameworks)
	return ri
}

// anyBase reports whether any collected file has the given base name.
func anyBase(files []string, base string) bool {
	for _, f := range files {
		if path.Base(f) == base {
			return true
		}
	}
	return false
}

// appendUnique appends v to s only if absent.
func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// goVersion parses the `go X.Y` directive from the first go.mod found (sorted),
// returning "" when none is readable.
func goVersion(repoRoot string, files []string) string {
	for _, f := range files {
		if path.Base(f) != "go.mod" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if v, ok := strings.CutPrefix(line, "go "); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

// nodePackageManager identifies the package manager from a lockfile anywhere in
// the repo, defaulting to "npm".
func nodePackageManager(repoRoot string, files []string) string {
	for _, cand := range []struct{ base, pm string }{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	} {
		if anyBase(files, cand.base) {
			return cand.pm
		}
	}
	return "npm"
}

// pythonPackageManager reports "poetry" when a poetry lockfile is present, else
// "pip".
func pythonPackageManager(files []string) string {
	if anyBase(files, "poetry.lock") {
		return "poetry"
	}
	return "pip"
}

// nodeMains returns the workspace-relative entry points declared in a node
// workspace's package.json "main" and "bin" fields (bin may be a string or an
// object). Paths are normalized to slash form; unreadable/absent fields yield none.
func nodeMains(repoRoot, dir string) []string {
	pkgPath := filepath.Join(repoRoot, filepath.FromSlash(dir), "package.json")
	if dir == "." {
		pkgPath = filepath.Join(repoRoot, "package.json")
	}
	b, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}
	var pkg struct {
		Main string          `json:"main"`
		Bin  json.RawMessage `json:"bin"`
	}
	if json.Unmarshal(b, &pkg) != nil {
		return nil
	}
	var out []string
	if m := strings.TrimSpace(pkg.Main); m != "" {
		out = append(out, filepath.ToSlash(m))
	}
	if len(pkg.Bin) > 0 {
		var binStr string
		if json.Unmarshal(pkg.Bin, &binStr) == nil {
			if binStr = strings.TrimSpace(binStr); binStr != "" {
				out = append(out, filepath.ToSlash(binStr))
			}
		} else {
			var binMap map[string]string
			if json.Unmarshal(pkg.Bin, &binMap) == nil {
				for _, v := range binMap {
					if v = strings.TrimSpace(v); v != "" {
						out = append(out, filepath.ToSlash(v))
					}
				}
			}
		}
	}
	return out
}
