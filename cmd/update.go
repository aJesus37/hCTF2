package cmd

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ajesus37/hCTF/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
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

// downloadAndExtract downloads a .tar.gz from url, finds the binary entry
// named "hctf" or "hctf.exe", and writes it to destPath with mode 0755.
func downloadAndExtract(url, destPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil) //nolint:gosec — URL comes from GitHub API
	if err != nil {
		return fmt.Errorf("creating download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != "hctf" && base != "hctf.exe" {
			continue
		}

		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("creating dest: %w", err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			os.Remove(destPath)
			return fmt.Errorf("writing: %w", err)
		}
		return f.Close()
	}
	return fmt.Errorf("binary not found in archive")
}

// canWriteExec checks whether the process can open execPath for writing.
func canWriteExec(execPath string) bool {
	f, err := os.OpenFile(execPath, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// atomicReplace moves newPath over targetPath using os.Rename (atomic on
// same filesystem). Cleans up newPath on failure.
func atomicReplace(newPath, targetPath string) error {
	if err := os.Rename(newPath, targetPath); err != nil {
		os.Remove(newPath)
		return fmt.Errorf("replacing binary: %w", err)
	}
	return nil
}

// buildSudoArgs constructs the argument list for a sudo re-exec.
// Returns ["sudo", execPath, arg1, arg2, ...]
func buildSudoArgs(execPath string, args []string) []string {
	return append([]string{"sudo", execPath}, args...)
}

// reexecWithSudo re-runs the current binary with sudo, forwarding os.Args[1:].
// Only called on Linux when canWriteExec returns false.
func reexecWithSudo(execPath string) error {
	args := buildSudoArgs(execPath, os.Args[1:])
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var (
	updateFlagBeta  bool
	updateFlagCheck bool
	updateFlagYes   bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install the latest hCTF release",
	Long: `Checks GitHub Releases for a newer version and replaces the running binary.

Channels:
  stable  tagged releases only (default)
  beta    includes pre-releases

Channel is resolved from: --beta flag > config update_channel > stable`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateFlagBeta, "beta", false, "Use beta channel (includes pre-releases)")
	updateCmd.Flags().BoolVar(&updateFlagCheck, "check", false, "Only check for updates, do not install")
	updateCmd.Flags().BoolVarP(&updateFlagYes, "yes", "y", false, "Skip confirmation prompt")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.UpdateChannel != "" && cfg.UpdateChannel != "stable" && cfg.UpdateChannel != "beta" {
		fmt.Fprintf(os.Stderr, "Warning: unrecognized update_channel %q in config, defaulting to stable\n", cfg.UpdateChannel)
	}

	beta := resolveChannel(updateFlagBeta, cfg.UpdateChannel)
	fmt.Fprintf(os.Stderr, "Checking for updates (channel: %s)...\n", channelName(beta))

	rel, err := fetchLatestRelease(ghReleasesURL, beta)
	if err != nil {
		return err
	}

	current := rootCmd.Version
	latestTag := rel.TagName
	// Normalize: compare with and without leading "v"
	if latestTag == current || latestTag == "v"+strings.TrimPrefix(current, "v") || "v"+latestTag == current {
		fmt.Println("Already up to date:", current)
		return nil
	}

	fmt.Printf("Current: %s\nLatest:  %s\n", current, rel.TagName)

	if updateFlagCheck {
		return nil
	}

	asset, err := findAsset(rel, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	if !updateFlagYes && term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Printf("Install %s? [y/N] ", rel.TagName)
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	if !canWriteExec(execPath) {
		if runtime.GOOS == "linux" {
			fmt.Fprintln(os.Stderr, "Insufficient permissions, re-running with sudo...")
			return reexecWithSudo(execPath)
		}
		return fmt.Errorf("cannot write to %s — re-run as administrator", execPath)
	}

	tmpPath := execPath + ".new"
	fmt.Fprintf(os.Stderr, "Downloading %s...\n", asset.Name)
	if err := downloadAndExtract(asset.DownloadURL, tmpPath); err != nil {
		return err
	}

	if err := atomicReplace(tmpPath, execPath); err != nil {
		return err
	}

	fmt.Printf("Updated to %s\n", rel.TagName)
	return nil
}

func channelName(beta bool) string {
	if beta {
		return "beta"
	}
	return "stable"
}
