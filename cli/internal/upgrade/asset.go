package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// checksumsFileName is the SHA256 manifest GoReleaser publishes next to the
// archives (.goreleaser.yml checksum.name_template).
const checksumsFileName = "checksums.txt"

// maxBinarySize bounds how many bytes are extracted for the binary, guarding
// against a decompression bomb (the real binary is a few tens of MiB).
const maxBinarySize = 512 << 20

// supportedPlatforms mirrors the goos × goarch matrix of .goreleaser.yml.
var supportedPlatforms = map[string]map[string]bool{
	"darwin":  {"amd64": true, "arm64": true},
	"linux":   {"amd64": true, "arm64": true},
	"windows": {"amd64": true, "arm64": true},
}

// resolveAssetName builds the release archive name for a platform, replicating
// the GoReleaser contract `vector_<VERSION>_<OS>_<ARCH>` with <VERSION> = tag
// without its leading "v" (same strip as scripts/install.sh): .tar.gz on unix,
// .zip on windows.
func resolveAssetName(goos, goarch, version string) (string, error) {
	arches, knownOS := supportedPlatforms[goos]
	if !knownOS || !arches[goarch] {
		return "", fmt.Errorf("no prebuilt vector binary for %s/%s — supported: darwin, linux, windows on amd64 and arm64", goos, goarch)
	}
	bareVersion := strings.TrimPrefix(version, "v")
	if bareVersion == "" {
		return "", errors.New("empty release version")
	}
	extension := ".tar.gz"
	if goos == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf("vector_%s_%s_%s%s", bareVersion, goos, goarch, extension), nil
}

// binaryName is the executable's file name inside the archive for goos.
func binaryName(goos string) string {
	if goos == "windows" {
		return "vector.exe"
	}
	return "vector"
}

// downloadFile GETs url into destPath. Any non-200 status is an error naming the
// URL so the user knows what could not be fetched.
func downloadFile(ctx context.Context, httpClient *http.Client, url, destPath string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, response.StatusCode)
	}
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(destPath), err)
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		return fmt.Errorf("write %s: %w", filepath.Base(destPath), err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", filepath.Base(destPath), err)
	}
	return nil
}

// verifyChecksum checks assetPath's SHA256 against the "<sha256>  <filename>"
// line for assetName in checksums.txt (same format scripts/install.sh verifies).
func verifyChecksum(assetPath, checksumsPath, assetName string) error {
	expected, err := expectedChecksum(checksumsPath, assetName)
	if err != nil {
		return err
	}
	file, err := os.Open(assetPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", assetName, err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash %s: %w", assetName, err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch for %s (expected %s, got %s) — the download may be corrupt, try again", assetName, expected, actual)
	}
	return nil
}

// expectedChecksum finds assetName's digest in checksums.txt.
func expectedChecksum(checksumsPath, assetName string) (string, error) {
	file, err := os.Open(checksumsPath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", checksumsFileName, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", checksumsFileName, err)
	}
	return "", fmt.Errorf("no checksum entry for %s in %s — cannot verify integrity", assetName, checksumsFileName)
}

// extractArchive pulls the vector binary out of a .tar.gz or .zip archive into
// destDir and returns its path. Only the entry whose base name is the binary is
// extracted (entry paths are never joined, so path traversal is impossible).
func extractArchive(archivePath, destDir, goos string) (string, error) {
	wanted := binaryName(goos)
	destPath := filepath.Join(destDir, wanted)
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"):
		return destPath, extractFromTarGz(archivePath, wanted, destPath)
	case strings.HasSuffix(archivePath, ".zip"):
		return destPath, extractFromZip(archivePath, wanted, destPath)
	default:
		return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
}

// extractFromTarGz copies the regular-file entry named wanted to destPath.
func extractFromTarGz(archivePath, wanted, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read gzip archive: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("archive does not contain %s", wanted)
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == wanted {
			return writeExecutable(tarReader, destPath)
		}
	}
}

// extractFromZip copies the file entry named wanted to destPath.
func extractFromZip(archivePath, wanted, destPath string) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("read zip archive: %w", err)
	}
	defer zipReader.Close()
	for _, entry := range zipReader.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != wanted {
			continue
		}
		entryReader, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open %s in archive: %w", wanted, err)
		}
		defer entryReader.Close()
		return writeExecutable(entryReader, destPath)
	}
	return fmt.Errorf("archive does not contain %s", wanted)
}

// writeExecutable streams source into destPath with mode 0755, refusing
// anything larger than maxBinarySize.
func writeExecutable(source io.Reader, destPath string) error {
	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create extracted binary: %w", err)
	}
	written, err := io.Copy(file, io.LimitReader(source, maxBinarySize+1))
	if err != nil {
		file.Close()
		return fmt.Errorf("extract binary: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close extracted binary: %w", err)
	}
	if written > maxBinarySize {
		return errors.New("extracted binary exceeds the size limit — refusing")
	}
	return nil
}
