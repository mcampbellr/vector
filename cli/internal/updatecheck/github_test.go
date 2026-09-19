package updatecheck

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchLatestRelease(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		wantTag     string
		wantErr     bool
		wantLimited bool
	}{
		{"ok", http.StatusOK, `{"tag_name":"v1.2.3","assets":[{"name":"checksums.txt","browser_download_url":"https://example.invalid/checksums.txt"}]}`, "v1.2.3", false, false},
		{"rate limited 403", http.StatusForbidden, `{"message":"API rate limit exceeded"}`, "", true, true},
		{"rate limited 429", http.StatusTooManyRequests, ``, "", true, true},
		{"not found", http.StatusNotFound, `{"message":"Not Found"}`, "", true, false},
		{"server error", http.StatusInternalServerError, ``, "", true, false},
		{"malformed json", http.StatusOK, `{"tag_name":`, "", true, false},
		{"empty tag", http.StatusOK, `{"tag_name":""}`, "", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/repos/"+Repository+"/releases/latest" {
					t.Errorf("unexpected path %q", request.URL.Path)
				}
				if got := request.Header.Get("Accept"); got != "application/vnd.github+json" {
					t.Errorf("Accept = %q", got)
				}
				if request.Header.Get("Authorization") != "" {
					t.Error("request must be unauthenticated")
				}
				writer.WriteHeader(tc.status)
				_, _ = writer.Write([]byte(tc.body))
			}))
			defer server.Close()

			client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
			release, err := client.FetchLatestRelease(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantLimited && !errors.Is(err, ErrRateLimited) {
				t.Errorf("err = %v, want ErrRateLimited", err)
			}
			if !tc.wantErr && release.TagName != tc.wantTag {
				t.Errorf("tag = %q, want %q", release.TagName, tc.wantTag)
			}
			if !tc.wantErr && len(release.Assets) != 1 {
				t.Errorf("assets = %d, want 1", len(release.Assets))
			}
		})
	}
}

func TestFetchLatestReleaseTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	started := time.Now()
	if _, err := client.FetchLatestRelease(ctx); err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Errorf("timeout not honored: took %v", elapsed)
	}
}
