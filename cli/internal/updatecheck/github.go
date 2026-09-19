package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultAPIBaseURL is the GitHub REST API root; Client.BaseURL overrides it in
// tests (httptest.Server).
const DefaultAPIBaseURL = "https://api.github.com"

// Repository is the GitHub owner/name Vector releases are published under
// (.goreleaser.yml release.github).
const Repository = "mcampbellr/vector"

// requestTimeout bounds a single releases/latest call, independently of the
// caller's context.
const requestTimeout = 5 * time.Second

// maxReleaseBody caps how much of the API response is read (a release payload is
// a few KiB; anything far larger is not a release).
const maxReleaseBody = 4 << 20

// GitHubRelease is the subset of the releases/latest payload Vector consumes.
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

// GitHubAsset is one downloadable file attached to a release.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ErrRateLimited is returned for 403/429 responses (unauthenticated GitHub API
// limit). Callers treat it like any other error: silently.
var ErrRateLimited = errors.New("github api rate limit reached")

// Client fetches release metadata from the GitHub Releases API. The zero value
// is usable and targets api.github.com.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// FetchLatestRelease queries releases/latest on the public repository (no
// authentication, no retries).
func FetchLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	return (&Client{}).FetchLatestRelease(ctx)
}

// FetchLatestRelease queries releases/latest. Every failure — network, timeout,
// non-200 status, malformed JSON, empty tag — is returned as an error; it never
// panics.
func (client *Client) FetchLatestRelease(ctx context.Context) (*GitHubRelease, error) {
	baseURL := client.BaseURL
	if baseURL == "" {
		baseURL = DefaultAPIBaseURL
	}
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	url := fmt.Sprintf("%s/repos/%s/releases/latest", baseURL, Repository)
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("query latest release: %w", err)
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusForbidden, response.StatusCode == http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case response.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("query latest release: unexpected HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseBody))
	if err != nil {
		return nil, fmt.Errorf("read latest release: %w", err)
	}
	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("decode latest release: %w", err)
	}
	if release.TagName == "" {
		return nil, errors.New("decode latest release: empty tag_name")
	}
	return &release, nil
}
