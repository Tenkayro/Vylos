package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultGitHubRepo = "Tenkayro/Vylos"
	LatestJSONURL     = "https://github.com/Tenkayro/Vylos/releases/latest/download/latest.json"
)

// AppVersion is the current semantic version of Vylos Family Control (overridden via -ldflags "-X ..." during release builds).
var AppVersion = "1.0.0"

type PlatformAsset struct {
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature,omitempty"`
}

type ReleaseManifest struct {
	Version   string                   `json:"version"`
	Notes     string                   `json:"notes"`
	PubDate   string                   `json:"pub_date"`
	URL       string                   `json:"url,omitempty"`
	SHA256    string                   `json:"sha256,omitempty"`
	Platforms map[string]PlatformAsset `json:"platforms"`
}

type UpdateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	DownloadURL     string `json:"downloadUrl"`
	SHA256          string `json:"sha256"`
	ReleaseNotes    string `json:"releaseNotes"`
	PublishedAt     string `json:"publishedAt"`
	CheckedAt       string `json:"checkedAt"`
	Repository      string `json:"repository"`
}

type Manager struct {
	manifestURL string
	client      *http.Client
}

func NewManager(customManifestURL string) *Manager {
	if strings.TrimSpace(customManifestURL) == "" {
		customManifestURL = LatestJSONURL
	}
	return &Manager{
		manifestURL: customManifestURL,
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

// CheckForUpdates fetches latest.json from GitHub Releases (Tenkayro/Vylos) and compares versions.
func (m *Manager) CheckForUpdates() (UpdateCheckResult, error) {
	res := UpdateCheckResult{
		CurrentVersion: AppVersion,
		LatestVersion:  AppVersion,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		Repository:     "https://github.com/" + DefaultGitHubRepo,
	}

	req, err := http.NewRequest(http.MethodGet, m.manifestURL, nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("User-Agent", "Vylos-Family-Control-Updater/"+AppVersion)
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return res, fmt.Errorf("fetch update manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return res, fmt.Errorf("update server returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return res, err
	}

	var manifest ReleaseManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return res, fmt.Errorf("parse latest.json: %w", err)
	}

	latestVer := strings.TrimPrefix(strings.TrimSpace(manifest.Version), "v")
	if latestVer == "" {
		return res, errors.New("manifest missing version")
	}

	res.LatestVersion = latestVer
	res.ReleaseNotes = manifest.Notes
	res.PublishedAt = manifest.PubDate
	res.DownloadURL = manifest.URL
	res.SHA256 = manifest.SHA256

	if asset, ok := manifest.Platforms["windows-x86_64"]; ok {
		if asset.URL != "" {
			res.DownloadURL = asset.URL
		}
		if asset.SHA256 != "" {
			res.SHA256 = asset.SHA256
		}
	}

	res.UpdateAvailable = IsNewerSemver(AppVersion, latestVer)
	return res, nil
}

// DownloadAndInstallUpdate downloads the verified MSI installer from GitHub Releases, validates its SHA-256 hash,
// and launches msiexec to upgrade Vylos Family Control.
func (m *Manager) DownloadAndInstallUpdate() (UpdateCheckResult, error) {
	check, err := m.CheckForUpdates()
	if err != nil {
		return check, err
	}
	if !check.UpdateAvailable {
		return check, errors.New("already running the latest version of Vylos")
	}
	if check.DownloadURL == "" || !strings.HasPrefix(check.DownloadURL, "https://github.com/") {
		return check, errors.New("invalid or untrusted update download URL")
	}

	dlClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := dlClient.Get(check.DownloadURL)
	if err != nil {
		return check, fmt.Errorf("download MSI update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return check, fmt.Errorf("download MSI returned HTTP %d", resp.StatusCode)
	}

	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("Vylos-Setup-%s.msi", check.LatestVersion))
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return check, err
	}

	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)
	_, copyErr := io.Copy(writer, io.LimitReader(resp.Body, 250*1024*1024))
	_ = f.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return check, fmt.Errorf("write MSI update: %w", copyErr)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if check.SHA256 != "" && !strings.EqualFold(actualHash, strings.TrimSpace(check.SHA256)) {
		_ = os.Remove(tmpPath)
		return check, fmt.Errorf("MSI SHA-256 integrity mismatch: expected %s, got %s", check.SHA256, actualHash)
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("msiexec.exe", "/i", tmpPath, "/passive", "NORESTART=1")
		if err := cmd.Start(); err != nil {
			return check, fmt.Errorf("launch msiexec installer: %w", err)
		}
	}

	return check, nil
}

// IsNewerSemver returns true if candidate (e.g. "1.2.0") is strictly newer than current (e.g. "1.1.0").
func IsNewerSemver(current, candidate string) bool {
	cParts := parseSemverParts(current)
	nParts := parseSemverParts(candidate)
	for i := 0; i < 3; i++ {
		if nParts[i] > cParts[i] {
			return true
		}
		if nParts[i] < cParts[i] {
			return false
		}
	}
	return false
}

func parseSemverParts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v = strings.Split(v, "-")[0]
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}
