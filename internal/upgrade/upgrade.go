package upgrade

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Run checks for a newer release and replaces the running binary if one exists.
func Run(current, repo string) error {
	fmt.Printf("current version: %s\n", current)
	fmt.Printf("checking %s for updates...\n", repo)

	latest, err := latestVersion(repo)
	if err != nil {
		return fmt.Errorf("fetch latest version: %w", err)
	}
	fmt.Printf("latest version:  %s\n", latest)

	if current == latest {
		fmt.Println("already up to date")
		return nil
	}
	if current != "dev" && !isNewer(latest, current) {
		fmt.Println("already up to date")
		return nil
	}

	platform, arch, err := detectPlatform()
	if err != nil {
		return err
	}

	tarball := fmt.Sprintf("tko-%s-%s.tar.gz", platform, arch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, tarball)

	fmt.Printf("downloading %s\n", url)

	tmp, err := os.MkdirTemp("", "tko-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	tarPath := filepath.Join(tmp, tarball)
	if err := download(url, tarPath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	binPath := filepath.Join(tmp, "tko")
	if err := extractBinary(tarPath, binPath); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	dest, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}
	// Follow symlinks to get the real path
	dest, err = filepath.EvalSymlinks(dest)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	if err := replaceFile(binPath, dest); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	fmt.Printf("upgraded to %s\n", latest)
	return nil
}

func latestVersion(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no releases found")
	}
	return release.TagName, nil
}

func detectPlatform() (platform, arch string, err error) {
	switch runtime.GOOS {
	case "darwin":
		platform = "macos"
	case "linux":
		platform = "linux"
	default:
		return "", "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		return "", "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
	return platform, arch, nil
}

func download(url, dest string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractBinary(tarPath, dest string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "tko" {
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			return err
		}
	}
	return fmt.Errorf("tko binary not found in archive")
}

// replaceFile atomically replaces dest with src by writing to a temp file in
// the same directory and renaming it, so the swap is atomic on the same fs.
func replaceFile(src, dest string) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".tko-upgrade-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	in, err := os.Open(src)
	if err != nil {
		tmp.Close()
		return err
	}
	defer in.Close()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	return os.Rename(tmpPath, dest)
}

// isNewer returns true if a is a strictly higher semver than b.
// Handles versions with or without a leading "v".
func isNewer(a, b string) bool {
	return strings.TrimPrefix(a, "v") > strings.TrimPrefix(b, "v")
}
