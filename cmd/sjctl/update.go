package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/spf13/cobra"
)

// Self-update keeps an installed sjctl current without re-running the bootstrap
// installer (which only fires when the binary is missing). It mirrors the
// install-sjctl scripts end to end: resolve the latest GitHub release, download
// the os/arch asset and checksums.txt, verify the keyless cosign signature over
// checksums.txt, verify the asset's sha256 against it, then atomically replace
// the running executable.
//
// The cosign step is the actual supply-chain boundary and must run on every
// update, not just first install: each update fetches a fresh checksums.txt +
// archive, so a matching sha256 alone is self-referential (whoever serves a
// malicious archive serves a matching checksums.txt with it). The signature
// proves checksums.txt came from this repo's release workflow. Cosign at
// first-install does NOT transitively cover later self-updates. Because
// auto-update runs unattended and daily, the explicit and automatic paths apply
// different cosign policies — see verifyReleaseSignature.

// canonicalRepo is the trusted GitHub owner/repo releases are pulled from. The
// automatic updater is pinned to this and never honors SJCTL_REPO: a daily,
// background self-update that downloads and runs a binary from an env-var-chosen
// repo would let anything that can set an env var in the session gain persistent
// code execution on the next sjctl run.
const canonicalRepo = "solid-company/solid-jobs-skills"

// updateRepo is the repo the *explicit* `sjctl update` pulls from. SJCTL_REPO
// overrides it to keep parity with the installer scripts — that override is an
// explicit, interactive user action, unlike the automatic path.
func updateRepo() string {
	if v := os.Getenv("SJCTL_REPO"); v != "" {
		return v
	}
	return canonicalRepo
}

func newUpdateCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update sjctl to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return selfUpdate(cmd.OutOrStdout(), updateRepo(), force, false)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "reinstall even if already on the latest version")
	return cmd
}

// httpGet issues a GET with the User-Agent the GitHub API requires and returns
// the body bytes on a 2xx response.
func httpGet(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sjctl-updater")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// latestReleaseTag returns the tag_name of the repo's latest release, e.g. v0.2.0.
func latestReleaseTag(repo string) (string, error) {
	body, err := httpGet("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", err
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("could not resolve latest release for %s", repo)
	}
	return rel.TagName, nil
}

// assetName is the archive published for this os/arch: tar.gz everywhere except
// Windows, which ships a zip. Matches .goreleaser.yaml's name_template. Note
// .goreleaser.yaml skips windows/arm64, so on that platform this names an asset
// that does not exist and the download 404s — harmless, as auto-update treats it
// as a best-effort failure (the installer has the same gap).
func assetName() string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("sjctl_%s_%s.%s", runtime.GOOS, runtime.GOARCH, ext)
}

// selfUpdate resolves the latest release and, unless already current, downloads,
// verifies and installs it over the running binary. requireSignature controls
// the cosign-absent policy (see verifyReleaseSignature): the automatic path sets
// it so a missing cosign aborts the unattended update rather than installing
// unverified code; the explicit `sjctl update` leaves it off to match the
// installer's verify-when-present behavior.
func selfUpdate(out io.Writer, repo string, force, requireSignature bool) error {
	if version == "dev" && !force {
		return fmt.Errorf("this is a dev build; refusing to self-update (use --force to override)")
	}

	tag, err := latestReleaseTag(repo)
	if err != nil {
		return fail("resolve latest release", err)
	}
	// `sjctl version` prints main.version, which GoReleaser stamps without the
	// leading v; compare against the tag with its v stripped.
	if !force && version == strings.TrimPrefix(tag, "v") {
		fmt.Fprintf(out, "sjctl is already up to date (%s)\n", version)
		return nil
	}
	return installRelease(out, repo, tag, requireSignature)
}

// installRelease downloads repo's release tag for this os/arch, verifies it
// (cosign over checksums.txt, then the asset's sha256) and atomically swaps the
// running binary. The tag is passed in so the auto path can reuse the one it
// already resolved instead of re-querying the releases API.
func installRelease(out io.Writer, repo, tag string, requireSignature bool) error {
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s", repo, tag)
	asset := assetName()

	archive, err := httpGet(base + "/" + asset)
	if err != nil {
		return fail("download "+asset, err)
	}
	sums, err := httpGet(base + "/checksums.txt")
	if err != nil {
		return fail("download checksums.txt", err)
	}
	// cosign first (proves checksums.txt is authentic), then sha256 (proves the
	// archive matches it) — same order as install-sjctl.sh.
	if err := verifyReleaseSignature(out, base, repo, sums, requireSignature); err != nil {
		return fail("verify signature", err)
	}
	if err := verifyChecksum(archive, sums, asset); err != nil {
		return fail("verify "+asset, err)
	}

	binData, err := extractBinary(archive)
	if err != nil {
		return fail("extract sjctl", err)
	}
	if err := replaceRunningBinary(binData); err != nil {
		return fail("install update", err)
	}

	fmt.Fprintf(out, "updated sjctl to %s\n", tag)
	return nil
}

// verifyReleaseSignature gates installation on the keyless cosign signature over
// checksums.txt, mirroring install-sjctl.sh. SJCTL_SKIP_COSIGN=1 bypasses it
// (explicit user opt-out). When cosign is not on PATH it cannot verify: the
// installer warns and proceeds, and the explicit `sjctl update` does the same
// (require=false) since the user is present to judge. The automatic path passes
// require=true so an unattended daily update never installs unverified code —
// it aborts instead, leaving the running binary untouched.
func verifyReleaseSignature(out io.Writer, base, repo string, sums []byte, require bool) error {
	if os.Getenv("SJCTL_SKIP_COSIGN") == "1" {
		fmt.Fprintln(out, "sjctl: skipping cosign verification (SJCTL_SKIP_COSIGN=1)")
		return nil
	}
	if _, err := exec.LookPath("cosign"); err != nil {
		if require {
			return fmt.Errorf("cosign not on PATH; refusing to auto-update unverified " +
				"(install cosign, run `sjctl update` manually, or set SJCTL_SKIP_COSIGN=1)")
		}
		fmt.Fprintln(out, "sjctl: cosign not found; skipping signature verification "+
			"(install cosign or set SJCTL_SKIP_COSIGN=1 to silence)")
		return nil
	}
	return verifyCosign(base, repo, sums)
}

// verifyCosign runs `cosign verify-blob` over checksums.txt, binding the keyless
// signature to repo's release workflow identity (the GitHub Actions OIDC issuer),
// exactly as install-sjctl.sh does.
func verifyCosign(base, repo string, sums []byte) error {
	sig, err := httpGet(base + "/checksums.txt.sig")
	if err != nil {
		return fmt.Errorf("download checksums.txt.sig: %w", err)
	}
	pem, err := httpGet(base + "/checksums.txt.pem")
	if err != nil {
		return fmt.Errorf("download checksums.txt.pem: %w", err)
	}

	dir, err := os.MkdirTemp("", "sjctl-cosign")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	sumsPath := filepath.Join(dir, "checksums.txt")
	sigPath := filepath.Join(dir, "checksums.txt.sig")
	pemPath := filepath.Join(dir, "checksums.txt.pem")
	for _, f := range []struct {
		path string
		data []byte
	}{{sumsPath, sums}, {sigPath, sig}, {pemPath, pem}} {
		if err := os.WriteFile(f.path, f.data, 0o644); err != nil {
			return err
		}
	}

	cmd := exec.Command("cosign", "verify-blob",
		"--certificate", pemPath,
		"--signature", sigPath,
		"--certificate-identity-regexp", "^https://github.com/"+repo+"/",
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		sumsPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cosign signature verification failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// verifyChecksum confirms sha256(archive) matches the entry for asset in a
// checksums.txt body ("<hex>  <name>" per line).
func verifyChecksum(archive, sums []byte, asset string) error {
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			if fields[0] == got {
				return nil
			}
			return fmt.Errorf("checksum mismatch: have %s, want %s", got, fields[0])
		}
	}
	return fmt.Errorf("no checksum entry for %s", asset)
}

// extractBinary pulls the sjctl executable out of a release archive: a gzipped
// tar on Unix, a zip on Windows (where the entry is sjctl.exe).
func extractBinary(archive []byte) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return extractFromZip(archive, "sjctl.exe")
	}
	return extractFromTarGz(archive, "sjctl")
}

func extractFromTarGz(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == name {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", name)
}

func extractFromZip(archive []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", name)
}

// replaceRunningBinary writes binData over the currently running executable.
// A running binary cannot be overwritten in place on Windows, but it can be
// renamed; so on every platform we stage the new file beside the target (same
// filesystem, so the rename is atomic) and swap it in, moving the old binary
// aside first on Windows.
func replaceRunningBinary(binData []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}

	dir := filepath.Dir(exe)
	staged := filepath.Join(dir, ".sjctl.new")
	if err := os.WriteFile(staged, binData, 0o755); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			os.Remove(staged)
			return err
		}
		if err := os.Rename(staged, exe); err != nil {
			os.Rename(old, exe) // best-effort rollback
			return err
		}
		_ = os.Remove(old) // usually fails while running; cleaned up next time
		return nil
	}

	if err := os.Rename(staged, exe); err != nil {
		os.Remove(staged)
		return err
	}
	return os.Chmod(exe, 0o755)
}

// maybeAutoUpdate runs at most once per day: it checks for a newer release and,
// if found, installs it. The update takes effect on the next invocation, so the
// current command continues on the running binary. Best-effort — any failure is
// reported to stderr and otherwise ignored so it never blocks normal use.
// Disable with SJCTL_NO_AUTO_UPDATE=1.
func maybeAutoUpdate() {
	if version == "dev" || os.Getenv("SJCTL_NO_AUTO_UPDATE") == "1" {
		return
	}
	stamp := updateStampPath()
	if recentlyChecked(stamp) {
		return
	}
	touchStamp(stamp) // record the attempt up front so a hang can't loop

	// Always the canonical repo here — never SJCTL_REPO. See canonicalRepo.
	// The pre-check failing (network blip, rate limit) returns silently: it's
	// indistinguishable from "no update available" and not worth a daily line of
	// noise. Once we know an update exists, a failure mid-install (download,
	// signature, swap) is surfaced to stderr — that's an actionable problem
	// (notably a cosign abort), not background noise. installRelease reuses the
	// tag resolved here rather than querying the releases API a second time.
	tag, err := latestReleaseTag(canonicalRepo)
	if err != nil || version == strings.TrimPrefix(tag, "v") {
		return
	}
	var buf bytes.Buffer
	if err := installRelease(&buf, canonicalRepo, tag, true); err != nil {
		fmt.Fprintln(os.Stderr, "sjctl: auto-update failed:", err)
		return
	}
	fmt.Fprintf(os.Stderr, "sjctl: updated to %s (takes effect on next run)\n", tag)
}

func updateStampPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".solid-jobs-skills", ".update-check")
	}
	return filepath.Join(os.TempDir(), "sjctl-update-check")
}

// recentlyChecked reports whether the last update check was under 24h ago.
func recentlyChecked(stamp string) bool {
	info, err := os.Stat(stamp)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < 24*time.Hour
}

func touchStamp(stamp string) {
	_ = os.MkdirAll(filepath.Dir(stamp), 0o755)
	now := time.Now()
	if err := os.Chtimes(stamp, now, now); err != nil {
		_ = os.WriteFile(stamp, []byte(now.Format(time.RFC3339)), 0o644)
	}
}
