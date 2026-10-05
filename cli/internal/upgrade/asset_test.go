package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildArchive returns a release archive (tar.gz, or zip for windows) holding
// the vector binary with content, plus a README like GoReleaser ships.
func buildArchive(t *testing.T, goos string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	name := binaryName(goos)
	if goos == "windows" {
		writer := zip.NewWriter(&buffer)
		for entryName, entryContent := range map[string][]byte{"README.md": []byte("readme"), name: content} {
			entry, err := writer.Create(entryName)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(entryContent); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range []struct {
		name    string
		content []byte
	}{{"README.md", []byte("readme")}, {name, content}} {
		if err := tarWriter.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(entry.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestResolveAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, version string
		want                  string
		wantErr               bool
	}{
		{"darwin", "arm64", "v1.2.3", "vector_1.2.3_darwin_arm64.tar.gz", false},
		{"linux", "amd64", "1.2.3", "vector_1.2.3_linux_amd64.tar.gz", false},
		{"windows", "arm64", "v1.2.3", "vector_1.2.3_windows_arm64.zip", false},
		{"freebsd", "amd64", "v1.2.3", "", true},
		{"linux", "386", "v1.2.3", "", true},
		{"linux", "amd64", "v", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch+"/"+tc.version, func(t *testing.T) {
			got, err := resolveAssetName(tc.goos, tc.goarch, tc.version)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("resolveAssetName = (%q, %v), want (%q, err=%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	content := []byte("archive-bytes")
	assetPath := filepath.Join(dir, "vector_1.2.3_linux_amd64.tar.gz")
	if err := os.WriteFile(assetPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		checksums string
		wantErr   string
	}{
		{"match", fmt.Sprintf("%s  other.tar.gz\n%s  vector_1.2.3_linux_amd64.tar.gz\n", sha256Hex([]byte("x")), sha256Hex(content)), ""},
		{"uppercase digest", fmt.Sprintf("%s  vector_1.2.3_linux_amd64.tar.gz\n", strings.ToUpper(sha256Hex(content))), ""},
		{"mismatch", fmt.Sprintf("%s  vector_1.2.3_linux_amd64.tar.gz\n", sha256Hex([]byte("tampered"))), "checksum mismatch"},
		{"missing entry", fmt.Sprintf("%s  other.tar.gz\n", sha256Hex(content)), "no checksum entry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checksumsPath := filepath.Join(dir, tc.name+"-checksums.txt")
			if err := os.WriteFile(checksumsPath, []byte(tc.checksums), 0o644); err != nil {
				t.Fatal(err)
			}
			err := verifyChecksum(assetPath, checksumsPath, filepath.Base(assetPath))
			if tc.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestExtractArchive(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			dir := t.TempDir()
			content := []byte("new-binary-" + goos)
			extension := ".tar.gz"
			if goos == "windows" {
				extension = ".zip"
			}
			archivePath := filepath.Join(dir, "vector_1.2.3_"+goos+"_amd64"+extension)
			if err := os.WriteFile(archivePath, buildArchive(t, goos, content), 0o644); err != nil {
				t.Fatal(err)
			}
			extractDir := filepath.Join(dir, "out")
			if err := os.Mkdir(extractDir, 0o755); err != nil {
				t.Fatal(err)
			}
			binaryPath, err := extractArchive(archivePath, extractDir, goos)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(binaryPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, content) {
				t.Errorf("extracted %q, want %q", got, content)
			}
		})
	}
}

func TestExtractArchiveMissingBinary(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "vector.tar.gz")
	// A unix archive (holding `vector`) searched for `vector.exe`.
	if err := os.WriteFile(archivePath, buildArchive(t, "linux", []byte("x")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := extractArchive(archivePath, dir, "windows"); err == nil || !strings.Contains(err.Error(), "does not contain vector.exe") {
		t.Fatalf("err = %v, want missing vector.exe", err)
	}
	if err := os.WriteFile(archivePath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := extractArchive(archivePath, dir, "linux"); err == nil {
		t.Fatal("expected an error for an empty archive")
	}
	if _, err := extractArchive(filepath.Join(dir, "vector.rar"), dir, "linux"); err == nil {
		t.Fatal("expected an error for an unsupported format")
	}
}

func TestDownloadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/ok" {
			_, _ = writer.Write([]byte("payload"))
			return
		}
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	dir := t.TempDir()
	destPath := filepath.Join(dir, "file")
	if err := downloadFile(context.Background(), server.Client(), server.URL+"/ok", destPath); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(destPath); string(got) != "payload" {
		t.Errorf("downloaded %q", got)
	}
	if err := downloadFile(context.Background(), server.Client(), server.URL+"/missing", destPath); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("err = %v, want HTTP 404", err)
	}
}
