package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
)

const ghReleasesURL = "https://api.github.com/repos/ajesus37/hCTF/releases"

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

// fetchLatestRelease calls the given URL (overridable in tests) and returns
// the newest release matching the channel. beta=true includes pre-releases.
func fetchLatestRelease(apiURL string, beta bool) (*ghRelease, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "hctf-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding releases: %w", err)
	}

	for _, r := range releases {
		r := r
		if !beta && r.Prerelease {
			continue
		}
		return &r, nil
	}
	return nil, fmt.Errorf("no releases found for channel (beta=%v)", beta)
}

// findAsset picks the correct binary asset for the current OS/arch.
// Asset names follow: hctf_{os}_{arch}.tar.gz (or .zip on windows)
func findAsset(rel *ghRelease, goos, goarch string) (*ghAsset, error) {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	want := fmt.Sprintf("hctf_%s_%s.%s", goos, goarch, ext)
	for _, a := range rel.Assets {
		a := a
		if a.Name == want {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("no asset %q in release %s", want, rel.TagName)
}

// resolveChannel returns true if the beta channel should be used.
// Priority: --beta flag > config.update_channel > stable
func resolveChannel(flagBeta bool, cfgChannel string) bool {
	if flagBeta {
		return true
	}
	return cfgChannel == "beta"
}

// assetName returns the expected asset name for the running platform.
func assetName() string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("hctf_%s_%s.%s", runtime.GOOS, runtime.GOARCH, ext)
}
