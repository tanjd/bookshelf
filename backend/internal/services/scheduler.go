package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tanjd/bookshelf/internal/repository"
)

// defaultInterval is used when no valid interval is configured.
const defaultInterval = 24 * time.Hour

// JobStatus describes the current state of a background job.
type JobStatus struct {
	Name       string     `json:"name"`
	Running    bool       `json:"running"`
	Interval   string     `json:"interval"`
	LastRunAt  *time.Time `json:"last_run_at"`
	LastResult string     `json:"last_result"`
}

// Scheduler runs periodic background tasks such as refreshing book cover images.
type Scheduler struct {
	books     repository.BookRepository
	admin     repository.AdminRepository
	coversDir string
	fallback  time.Duration // interval used when setting is absent/invalid
	client    *http.Client
	trigger   chan struct{}

	mu         sync.RWMutex
	running    bool
	lastRunAt  *time.Time
	lastResult string
}

// NewScheduler creates a Scheduler.
// intervalStr is the fallback duration string (e.g. "24h") used when the
// admin setting "cover_refresh_interval" is absent. Defaults to 24h on error.
func NewScheduler(books repository.BookRepository, admin repository.AdminRepository, coversDir, intervalStr string) *Scheduler {
	fallback, err := time.ParseDuration(intervalStr)
	if err != nil || fallback <= 0 {
		fallback = defaultInterval
	}
	return &Scheduler{
		books:     books,
		admin:     admin,
		coversDir: coversDir,
		fallback:  fallback,
		client:    &http.Client{Timeout: 15 * time.Second},
		trigger:   make(chan struct{}, 1),
	}
}

// interval reads the configured interval from admin settings, falling back to s.fallback.
func (s *Scheduler) interval() time.Duration {
	if s.admin != nil {
		if val, err := s.admin.GetSetting("cover_refresh_interval"); err == nil && val != "" {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				return d
			}
		}
	}
	return s.fallback
}

// Status returns the current status of the cover-refresh job.
func (s *Scheduler) Status() JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return JobStatus{
		Name:       "cover-refresh",
		Running:    s.running,
		Interval:   s.interval().String(),
		LastRunAt:  s.lastRunAt,
		LastResult: s.lastResult,
	}
}

// TriggerNow requests an immediate run. Non-blocking; ignored if already queued.
func (s *Scheduler) TriggerNow() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// Start launches the scheduler goroutine. It runs until ctx is cancelled.
// It uses a 1-minute base tick and checks the configured interval on each tick,
// so interval changes take effect within a minute.
func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("scheduler started", "interval", s.interval())
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	// Run once on startup after a short delay to avoid blocking server startup.
	go func() {
		select {
		case <-time.After(30 * time.Second):
			s.refreshCovers(ctx)
		case <-ctx.Done():
		}
	}()

	for {
		select {
		case <-ticker.C:
			// Only run if enough time has elapsed since the last run.
			s.mu.RLock()
			last := s.lastRunAt
			s.mu.RUnlock()
			if last != nil && time.Since(*last) < s.interval() {
				continue
			}
			s.refreshCovers(ctx)
		case <-s.trigger:
			s.refreshCovers(ctx)
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		}
	}
}

// refreshCovers downloads and caches cover images for books that still have
// external URLs (i.e. not yet cached locally).
func (s *Scheduler) refreshCovers(ctx context.Context) {
	if s.coversDir == "" {
		return
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		slog.Info("scheduler: cover refresh already running, skipping")
		return
	}
	s.running = true
	now := time.Now()
	s.lastRunAt = &now
	s.lastResult = "running…"
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	slog.Info("scheduler: starting cover refresh")

	books, err := s.books.List("", "title", false)
	if err != nil {
		slog.Error("scheduler: failed to list books", "error", err)
		s.mu.Lock()
		s.lastResult = "failed: " + err.Error()
		s.mu.Unlock()
		return
	}

	refreshed := 0
	for _, book := range books {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Skip books with no cover or already a locally-cached path.
		if book.CoverURL == "" || strings.HasPrefix(book.CoverURL, "/") {
			continue
		}

		localPath, dlErr := s.downloadCover(book.CoverURL)
		if dlErr != nil {
			slog.Warn("scheduler: cover download failed", "book_id", book.ID, "error", dlErr)
			continue
		}
		if localPath == "" || localPath == book.CoverURL {
			continue
		}

		book.CoverURL = localPath
		if saveErr := s.books.Save(&book); saveErr != nil {
			slog.Warn("scheduler: failed to save cover path", "book_id", book.ID, "error", saveErr)
			continue
		}
		refreshed++
	}

	result := fmt.Sprintf("refreshed %d of %d books", refreshed, len(books))
	slog.Info("scheduler: cover refresh complete", "refreshed", refreshed, "total", len(books))
	s.mu.Lock()
	s.lastResult = result
	s.mu.Unlock()
}

// downloadCover fetches an external image URL and saves it to the coversDir.
// Returns the proxy-accessible path (/api/covers/<filename>) on success.
func (s *Scheduler) downloadCover(externalURL string) (string, error) {
	sum := sha256.Sum256([]byte(externalURL))
	baseName := fmt.Sprintf("%x", sum[:8])

	resp, err := s.client.Get(externalURL) //nolint:noctx
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
	destPath := filepath.Join(s.coversDir, filename)

	// Already cached — skip re-download.
	if _, statErr := os.Stat(destPath); statErr == nil {
		return "/api/covers/" + filename, nil
	}

	f, err := os.Create(destPath) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	const maxBytes = 10 << 20 // 10 MiB
	if _, err := io.Copy(f, io.LimitReader(resp.Body, maxBytes)); err != nil {
		_ = os.Remove(destPath)
		return "", err
	}

	slog.Info("scheduler: cover cached", "filename", filename)
	return "/api/covers/" + filename, nil
}
