package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mariocampbell/vector/internal/state"
)

func TestValidateExcalidraw(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantErr bool
	}{
		{"valid", `{"type":"excalidraw","version":2,"elements":[]}`, false},
		{"valid with elements", `{"type":"excalidraw","version":2,"elements":[{"id":"r"}],"appState":{}}`, false},
		{"not json", `not json at all`, true},
		{"json array", `[1,2,3]`, true},
		{"missing type", `{"version":2,"elements":[]}`, true},
		{"missing version", `{"type":"excalidraw","elements":[]}`, true},
		{"missing elements", `{"type":"excalidraw","version":2}`, true},
		{"empty type", `{"type":"","version":2,"elements":[]}`, true},
		{"elements not array", `{"type":"excalidraw","version":2,"elements":{}}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExcalidraw([]byte(tt.doc))
			if (err != nil) != tt.wantErr {
				t.Errorf("validateExcalidraw(%s) err = %v, wantErr = %v", tt.doc, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeSketchName(t *testing.T) {
	tests := []struct {
		nameFlag, want string
		wantErr        bool
	}{
		{"", "", false}, // empty → let AttachSketch derive the canonical name
		{"  ", "", false},
		{"custom.excalidraw", "custom.excalidraw", false},
		{"../escape.excalidraw", "", true},
		{"a/b.excalidraw", "", true},
		{"..", "", true},
	}
	for _, tt := range tests {
		got, err := sanitizeSketchName(tt.nameFlag)
		if (err != nil) != tt.wantErr {
			t.Errorf("sanitizeSketchName(%q) err = %v, wantErr = %v", tt.nameFlag, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("sanitizeSketchName(%q) = %q, want %q", tt.nameFlag, got, tt.want)
		}
	}
}

func TestRunSpecAttachSketch(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.CreateSpec(state.CreateSpecParams{ID: "add-ui", Title: "Add UI", Now: time.Now()}); err != nil {
		t.Fatalf("CreateSpec: %v", err)
	}

	writeTmp := func(name, content string) string {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}
	validFile := writeTmp("sketch.excalidraw", `{"type":"excalidraw","version":2,"elements":[]}`)
	badFile := writeTmp("bad.excalidraw", `{"nope":true}`)

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"missing id", []string{"--file", validFile, "--repo-root", root}, true},
		{"missing file", []string{"add-ui", "--repo-root", root}, true},
		{"unreadable file", []string{"add-ui", "--file", filepath.Join(root, "nope.excalidraw"), "--repo-root", root}, true},
		{"invalid json shape", []string{"add-ui", "--file", badFile, "--repo-root", root}, true},
		{"unknown spec", []string{"ghost", "--file", validFile, "--repo-root", root}, true},
		{"valid", []string{"add-ui", "--file", validFile, "--repo-root", root}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runSpecAttachSketch(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("runSpecAttachSketch(%v) err = %v, wantErr = %v", tt.args, err, tt.wantErr)
			}
		})
	}

	// The valid attach persisted the sketch on state.
	spec, err := store.ReadSpec("add-ui")
	if err != nil {
		t.Fatalf("ReadSpec: %v", err)
	}
	// No --name was passed, so the binary named it canonically from the spec id.
	if len(spec.Sketches) != 1 || spec.Sketches[0].Name != "add-ui-sketch.excalidraw" {
		t.Fatalf("Sketches = %+v", spec.Sketches)
	}
}
