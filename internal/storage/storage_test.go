package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/camiloserranor/network-mapper/internal/topology"
)

func TestSnapshotStore_SaveAndList(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSnapshotStore(dir)
	if err != nil {
		t.Fatalf("NewSnapshotStore: %v", err)
	}

	// Save two snapshots with different timestamps
	topo1 := &topology.Topology{
		CollectedAt: time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC),
		Devices:     []topology.Device{{ID: "sw1", Type: "switch"}},
	}
	topo2 := &topology.Topology{
		CollectedAt: time.Date(2026, 5, 11, 12, 30, 0, 0, time.UTC),
		Devices:     []topology.Device{{ID: "sw1", Type: "switch"}, {ID: "host1", Type: "host"}},
	}

	info1, err := store.Save(topo1)
	if err != nil {
		t.Fatalf("Save topo1: %v", err)
	}
	if info1.SizeBytes == 0 {
		t.Error("Expected non-zero size for saved snapshot")
	}

	_, err = store.Save(topo2)
	if err != nil {
		t.Fatalf("Save topo2: %v", err)
	}

	// List should return 2 snapshots sorted by timestamp
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("Expected 2 snapshots, got %d", len(list))
	}
	if !list[0].Timestamp.Before(list[1].Timestamp) {
		t.Error("Snapshots not sorted by timestamp")
	}
}

func TestSnapshotStore_Load(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSnapshotStore(dir)
	if err != nil {
		t.Fatalf("NewSnapshotStore: %v", err)
	}

	ts := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	topo := &topology.Topology{
		CollectedAt: ts,
		Devices:     []topology.Device{{ID: "sw1", Type: "switch", SystemName: "tor-1"}},
		Links:       []topology.Link{{LocalDevice: "sw1", RemoteDevice: "host1"}},
	}

	_, err = store.Save(topo)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Devices) != 1 || loaded.Devices[0].SystemName != "tor-1" {
		t.Errorf("Loaded topology mismatch: got %d devices", len(loaded.Devices))
	}
	if len(loaded.Links) != 1 {
		t.Errorf("Expected 1 link, got %d", len(loaded.Links))
	}
}

func TestSnapshotStore_PruneByAge(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSnapshotStore(dir)
	if err != nil {
		t.Fatalf("NewSnapshotStore: %v", err)
	}

	// Save an old snapshot (10 days ago) and a new one (now)
	oldTopo := &topology.Topology{CollectedAt: time.Now().UTC().Add(-10 * 24 * time.Hour)}
	newTopo := &topology.Topology{CollectedAt: time.Now().UTC()}

	store.Save(oldTopo)
	store.Save(newTopo)

	list, _ := store.List()
	if len(list) != 2 {
		t.Fatalf("Expected 2 snapshots before prune, got %d", len(list))
	}

	// Prune files older than 7 days
	deleted, err := store.PruneByAge(7 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("PruneByAge: %v", err)
	}
	if deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", deleted)
	}

	list, _ = store.List()
	if len(list) != 1 {
		t.Errorf("Expected 1 snapshot after prune, got %d", len(list))
	}
}

func TestSnapshotStore_PruneByCount(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSnapshotStore(dir)
	if err != nil {
		t.Fatalf("NewSnapshotStore: %v", err)
	}

	// Save 5 snapshots
	for i := 0; i < 5; i++ {
		topo := &topology.Topology{
			CollectedAt: time.Now().UTC().Add(time.Duration(i) * time.Hour),
		}
		store.Save(topo)
	}

	list, _ := store.List()
	if len(list) != 5 {
		t.Fatalf("Expected 5 snapshots, got %d", len(list))
	}

	// Keep only 3
	deleted, err := store.PruneByCount(3)
	if err != nil {
		t.Fatalf("PruneByCount: %v", err)
	}
	if deleted != 2 {
		t.Errorf("Expected 2 deleted, got %d", deleted)
	}

	list, _ = store.List()
	if len(list) != 3 {
		t.Errorf("Expected 3 snapshots after prune, got %d", len(list))
	}
}

func TestPruneLogFiles(t *testing.T) {
	dir := t.TempDir()

	// Create old and new log files
	oldDate := time.Now().Add(-10 * 24 * time.Hour).Format(logDateFmt)
	newDate := time.Now().Format(logDateFmt)

	os.WriteFile(filepath.Join(dir, logPrefix+oldDate+logSuffix), []byte("old log"), 0644)
	os.WriteFile(filepath.Join(dir, logPrefix+newDate+logSuffix), []byte("new log"), 0644)

	deleted, err := PruneLogFiles(dir, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("PruneLogFiles: %v", err)
	}
	if deleted != 1 {
		t.Errorf("Expected 1 log deleted, got %d", deleted)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("Expected 1 file remaining, got %d", len(entries))
	}
}

func TestLogWriter_WritesAndRotates(t *testing.T) {
	dir := t.TempDir()

	lw, err := NewLogWriter(dir, 1) // 1 MB max
	if err != nil {
		t.Fatalf("NewLogWriter: %v", err)
	}
	defer lw.Close()

	// Write some data
	msg := "test log message\n"
	n, err := lw.Write([]byte(msg))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Expected %d bytes written, got %d", len(msg), n)
	}

	// Verify log file exists
	entries, err := os.ReadDir(lw.Dir())
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("Expected at least one log file")
	}
}
