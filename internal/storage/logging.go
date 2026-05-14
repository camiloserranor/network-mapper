package storage

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	logDir       = "logs"
	logPrefix    = "network-mapper-"
	logSuffix    = ".log"
	logDateFmt   = "2006-01-02"
)

// LogWriter writes log output to both stdout and a daily-rotated file.
type LogWriter struct {
	mu          sync.Mutex
	dir         string
	maxSizeMB   int
	currentFile *os.File
	currentDate string
	currentSize int64
	multiWriter io.Writer
}

// NewLogWriter creates a LogWriter that writes to stdout and daily log files
// in the given data directory. maxSizeMB controls max file size before rotation.
func NewLogWriter(dataDir string, maxSizeMB int) (*LogWriter, error) {
	dir := filepath.Join(dataDir, logDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory %s: %w", dir, err)
	}

	lw := &LogWriter{
		dir:       dir,
		maxSizeMB: maxSizeMB,
	}

	if err := lw.rotate(); err != nil {
		return nil, fmt.Errorf("opening initial log file: %w", err)
	}

	return lw, nil
}

// Write implements io.Writer. It handles daily rotation and size-based rotation.
func (lw *LogWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	today := time.Now().Format(logDateFmt)
	if today != lw.currentDate || (lw.maxSizeMB > 0 && lw.currentSize+int64(len(p)) > int64(lw.maxSizeMB)*1024*1024) {
		if err := lw.rotate(); err != nil {
			// Fall back to stdout only
			return os.Stdout.Write(p)
		}
	}

	n, err = lw.multiWriter.Write(p)
	lw.currentSize += int64(n)
	return n, err
}

// Close closes the current log file.
func (lw *LogWriter) Close() error {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	if lw.currentFile != nil {
		return lw.currentFile.Close()
	}
	return nil
}

// Dir returns the log directory path.
func (lw *LogWriter) Dir() string {
	return lw.dir
}

// Install sets this LogWriter as the output for the standard logger.
func (lw *LogWriter) Install() {
	log.SetOutput(lw)
}

func (lw *LogWriter) rotate() error {
	if lw.currentFile != nil {
		lw.currentFile.Close()
	}

	today := time.Now().Format(logDateFmt)
	filename := logPrefix + today + logSuffix
	path := filepath.Join(lw.dir, filename)

	// If file exists and exceeds size limit, use a numbered suffix
	info, err := os.Stat(path)
	if err == nil && lw.maxSizeMB > 0 && info.Size() > int64(lw.maxSizeMB)*1024*1024 {
		for i := 1; i < 100; i++ {
			path = filepath.Join(lw.dir, fmt.Sprintf("%s%s-%d%s", logPrefix, today, i, logSuffix))
			info, err = os.Stat(path)
			if os.IsNotExist(err) {
				break
			}
			if err == nil && info.Size() < int64(lw.maxSizeMB)*1024*1024 {
				break
			}
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file %s: %w", path, err)
	}

	fInfo, _ := f.Stat()
	var size int64
	if fInfo != nil {
		size = fInfo.Size()
	}

	lw.currentFile = f
	lw.currentDate = today
	lw.currentSize = size
	lw.multiWriter = io.MultiWriter(os.Stdout, f)

	return nil
}

// PruneLogFiles deletes log files older than maxAge in the log directory.
// Returns the number of files deleted.
func PruneLogFiles(logDir string, maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading log directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), logPrefix) || !strings.HasSuffix(e.Name(), logSuffix) {
			continue
		}

		// Extract date from filename
		name := strings.TrimPrefix(e.Name(), logPrefix)
		name = strings.TrimSuffix(name, logSuffix)
		// Handle numbered suffix (e.g., "2026-05-11-1")
		if idx := strings.LastIndex(name, "-"); idx > 9 {
			name = name[:idx]
		}

		t, err := time.Parse(logDateFmt, name)
		if err != nil {
			continue
		}

		if t.Before(cutoff) {
			path := filepath.Join(logDir, e.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("[storage] Failed to delete log file %s: %v", e.Name(), err)
				continue
			}
			deleted++
		}
	}

	if deleted > 0 {
		log.Printf("[storage] Pruned %d log file(s) older than %s", deleted, cutoff.Format(time.RFC3339))
	}
	return deleted, nil
}
