package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// downloadCover fetches an external image URL and saves it to destDir.
// Returns the proxy-accessible path (/api/covers/<filename>) on success.
// Returns ("", nil) if externalURL is empty or already a local path (skip).
// On failure, returns ("", err); callers should log and keep the original URL.
func downloadCover(externalURL, destDir string) (string, error) {
	if externalURL == "" {
		return "", nil
	}
	// Already a local/proxy path — nothing to download.
	if strings.HasPrefix(externalURL, "/") {
		return externalURL, nil
	}

	// Deterministic filename: first 8 bytes of SHA-256(url) as hex.
	sum := sha256.Sum256([]byte(externalURL))
	baseName := fmt.Sprintf("%x", sum[:8])

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(externalURL) //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover fetch returned %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return "", fmt.Errorf("unexpected content-type %q", ct)
	}

	ext := ".jpg"
	if strings.Contains(ct, "png") {
		ext = ".png"
	} else if strings.Contains(ct, "webp") {
		ext = ".webp"
	}

	filename := baseName + ext
	destPath := filepath.Join(destDir, filename)

	// Already cached — skip re-download.
	if _, err := os.Stat(destPath); err == nil {
		return "/api/covers/" + filename, nil
	}

	f, err := os.Create(destPath) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = os.Remove(destPath) // clean up partial file
		return "", err
	}

	slog.Info("cover cached", "filename", filename)
	return "/api/covers/" + filename, nil
}
