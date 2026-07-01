package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mariocampbell/vector/internal/state"
	"github.com/spf13/cobra"
)

// newSpecAttachSketchCmd implements `vector spec attach-sketch <id> --file <path>`:
// it validates that the file is a well-formed Excalidraw document (parseable JSON
// carrying top-level type/version/elements), sanitizes the destination name, and
// persists it via Store.AttachSketch — the sole writer of the sketch artifact and
// its state. The binary makes no LLM calls; the vector-ui-ux-designer agent writes
// the JSON to a temp path and calls this to commit it. A malformed document is a
// descriptive error here; the calling command/agent degrades softly (silent reject,
// the spec stays a clean draft). Mirrors route.go.
func newSpecAttachSketchCmd() *cobra.Command {
	var (
		idFlag   string
		file     string
		name     string
		repoRoot string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "attach-sketch [id]",
		Short: "attach a validated .excalidraw sketch to a spec",
		RunE: func(_ *cobra.Command, args []string) error {
			id, _ := leadingID(args)
			if id == "" {
				id = idFlag
			}
			if id == "" {
				return errors.New("usage: vector spec attach-sketch <id> --file <path.excalidraw>")
			}
			if file == "" {
				return errors.New("usage: vector spec attach-sketch <id> --file <path.excalidraw>")
			}

			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read sketch file: %w", err)
			}
			if err := validateExcalidraw(data); err != nil {
				return err
			}

			sketchName, err := sanitizeSketchName(name, file)
			if err != nil {
				return err
			}

			store, err := openStore(repoRoot)
			if err != nil {
				return err
			}
			ref := state.SketchRef{Name: sketchName, CreatedAt: time.Now().UTC()}
			if err := store.AttachSketch(id, data, ref, resolveActor()); err != nil {
				return err
			}

			if jsonOut {
				return printJSON(map[string]string{"id": id, "sketch": sketchName})
			}
			fmt.Printf("attached sketch %q to spec %q\n", sketchName, id)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&idFlag, "id", "", "spec id to attach the sketch to (optional; or pass it as the first argument)")
	f.StringVar(&file, "file", "", "path to the .excalidraw JSON to attach (required)")
	f.StringVar(&name, "name", "", "stored file name (defaults to the base name of --file)")
	f.StringVar(&repoRoot, "repo-root", "", "repo root (defaults to git toplevel or cwd)")
	f.BoolVar(&jsonOut, "json", false, "emit a JSON result for tooling")
	return cmd
}

// validateExcalidraw checks that data is a well-formed Excalidraw document: valid
// JSON with a non-empty string top-level "type", a present "version", and an array
// "elements". Persisting a broken .excalidraw would mislead the board and anyone
// downloading it, so the shape is verified before it reaches disk.
func validateExcalidraw(data []byte) error {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("invalid sketch: not a JSON object: %w", err)
	}
	for _, key := range []string{"type", "version", "elements"} {
		if _, ok := doc[key]; !ok {
			return fmt.Errorf("invalid sketch: missing top-level %q", key)
		}
	}
	var typ string
	if err := json.Unmarshal(doc["type"], &typ); err != nil || strings.TrimSpace(typ) == "" {
		return errors.New(`invalid sketch: "type" must be a non-empty string`)
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(doc["elements"], &elements); err != nil {
		return errors.New(`invalid sketch: "elements" must be an array`)
	}
	return nil
}

// sanitizeSketchName resolves the stored file name from --name (or the base name of
// --file) and rejects anything that is not a bare, safe file name (no separators,
// no traversal). The store re-validates under its lock; this surfaces a clear error
// before any I/O.
func sanitizeSketchName(nameFlag, file string) (string, error) {
	name := strings.TrimSpace(nameFlag)
	if name == "" {
		name = filepath.Base(file)
	}
	base := filepath.Base(name)
	if name != base || name == "." || name == ".." || name == "" || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid --name %q: must be a bare file name (no path separators)", nameFlag)
	}
	return name, nil
}
