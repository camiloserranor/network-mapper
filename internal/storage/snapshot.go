// Package storage manages topology snapshot persistence, log file rotation,
// and retention-based cleanup for the network-mapper.
package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

const (
	snapshotDir    = "snapshots"
	snapshotPrefix = "topology-"
	snapshotSuffix = ".json"
	// TimestampFormat is the format used in snapshot filenames (filesystem-safe).
	TimestampFormat = "2006-01-02T15-04-05Z"
)

// SnapshotInfo describes a stored topology snapshot.
type SnapshotInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Filename  string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
}

// SnapshotStore manages topology snapshot files on disk.
type SnapshotStore struct {
	dir string // full path to snapshots directory
}

// NewSnapshotStore creates a SnapshotStore rooted at the given data directory.
// It creates the snapshots subdirectory if it does not exist.
func NewSnapshotStore(dataDir string) (*SnapshotStore, error) {
	dir := filepath.Join(dataDir, snapshotDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating snapshot directory %s: %w", dir, err)
	}
	return &SnapshotStore{dir: dir}, nil
}

// Save writes a topology snapshot to disk with the collection timestamp as the filename.
// Returns the SnapshotInfo for the saved file.
func (s *SnapshotStore) Save(topo *topology.Topology) (*SnapshotInfo, error) {
	ts := topo.CollectedAt.UTC()
	filename := snapshotPrefix + ts.Format(TimestampFormat) + snapshotSuffix
	path := filepath.Join(s.dir, filename)

	data, err := json.MarshalIndent(topo, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling topology: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("writing snapshot %s: %w", path, err)
	}

	log.Printf("[storage] Snapshot saved: %s (%d bytes)", filename, len(data))
	return &SnapshotInfo{
		Timestamp: ts,
		Filename:  filename,
		SizeBytes: int64(len(data)),
	}, nil
}

// List returns all snapshots sorted by timestamp (oldest first).
func (s *SnapshotStore) List() ([]SnapshotInfo, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot directory: %w", err)
	}

	var snapshots []SnapshotInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), snapshotPrefix) || !strings.HasSuffix(e.Name(), snapshotSuffix) {
			continue
		}

		ts, err := parseSnapshotTimestamp(e.Name())
		if err != nil {
			continue // skip malformed files
		}

		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}

		snapshots = append(snapshots, SnapshotInfo{
			Timestamp: ts,
			Filename:  e.Name(),
			SizeBytes: size,
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.Before(snapshots[j].Timestamp)
	})

	return snapshots, nil
}

// Load reads and parses a topology snapshot by its timestamp.
func (s *SnapshotStore) Load(ts time.Time) (*topology.Topology, error) {
	filename := snapshotPrefix + ts.UTC().Format(TimestampFormat) + snapshotSuffix
	return s.LoadByFilename(filename)
}

// LoadByFilename reads and parses a topology snapshot by filename.
func (s *SnapshotStore) LoadByFilename(filename string) (*topology.Topology, error) {
	path := filepath.Join(s.dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot %s: %w", filename, err)
	}

	var topo topology.Topology
	if err := json.Unmarshal(data, &topo); err != nil {
		return nil, fmt.Errorf("parsing snapshot %s: %w", filename, err)
	}

	return &topo, nil
}

// PruneByAge deletes snapshots older than the given duration.
// Returns the number of files deleted.
func (s *SnapshotStore) PruneByAge(maxAge time.Duration) (int, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	return s.pruneOlderThan(cutoff)
}

// PruneByCount deletes the oldest snapshots if the total count exceeds maxCount.
// Returns the number of files deleted.
func (s *SnapshotStore) PruneByCount(maxCount int) (int, error) {
	snapshots, err := s.List()
	if err != nil {
		return 0, err
	}

	if len(snapshots) <= maxCount {
		return 0, nil
	}

	toDelete := len(snapshots) - maxCount
	deleted := 0
	for i := 0; i < toDelete; i++ {
		path := filepath.Join(s.dir, snapshots[i].Filename)
		if err := os.Remove(path); err != nil {
			log.Printf("[storage] Failed to delete snapshot %s: %v", snapshots[i].Filename, err)
			continue
		}
		deleted++
	}

	if deleted > 0 {
		log.Printf("[storage] Pruned %d snapshot(s) by count (max %d)", deleted, maxCount)
	}
	return deleted, nil
}

func (s *SnapshotStore) pruneOlderThan(cutoff time.Time) (int, error) {
	snapshots, err := s.List()
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, snap := range snapshots {
		if snap.Timestamp.Before(cutoff) {
			path := filepath.Join(s.dir, snap.Filename)
			if err := os.Remove(path); err != nil {
				log.Printf("[storage] Failed to delete snapshot %s: %v", snap.Filename, err)
				continue
			}
			deleted++
		}
	}

	if deleted > 0 {
		log.Printf("[storage] Pruned %d snapshot(s) older than %s", deleted, cutoff.Format(time.RFC3339))
	}
	return deleted, nil
}

// Dir returns the snapshot directory path.
func (s *SnapshotStore) Dir() string {
	return s.dir
}

func parseSnapshotTimestamp(filename string) (time.Time, error) {
	name := strings.TrimPrefix(filename, snapshotPrefix)
	name = strings.TrimSuffix(name, snapshotSuffix)
	return time.Parse(TimestampFormat, name)
}
