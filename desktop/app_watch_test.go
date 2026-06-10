package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSnapshotMtimesTracksNewestFile verifies snapshotMtimes records the newest
// file mtime beneath a root and that writing a newer file is detected as a
// change by mtimesChanged — the core of the live-watch loop, exercised without
// any Wails runtime or display.
func TestSnapshotMtimesTracksNewestFile(t *testing.T) {
	root := t.TempDir()
	older := filepath.Join(root, "nested", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(older), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(older, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, base, base); err != nil {
		t.Fatal(err)
	}

	prev := snapshotMtimes([]string{root})
	if got := prev[root]; !got.Equal(base) {
		t.Fatalf("snapshot newest = %v, want %v", got, base)
	}

	// A newer file lands; the snapshot's newest mtime must advance.
	newer := filepath.Join(root, "nested", "b.jsonl")
	if err := os.WriteFile(newer, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newerTime := base.Add(30 * time.Minute)
	if err := os.Chtimes(newer, newerTime, newerTime); err != nil {
		t.Fatal(err)
	}

	cur := snapshotMtimes([]string{root})
	if got := cur[root]; !got.Equal(newerTime) {
		t.Fatalf("snapshot after new file = %v, want %v", got, newerTime)
	}
	if !mtimesChanged(prev, cur) {
		t.Fatal("mtimesChanged should report a change when a newer file appears")
	}
}

func TestSnapshotMtimesSkipsCoreIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	baseFile := filepath.Join(root, "sessions", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(baseFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baseFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(baseFile, base, base); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{".git", "node_modules", "memory"} {
		ignored := filepath.Join(root, name, "newer.jsonl")
		if err := os.MkdirAll(filepath.Dir(ignored), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ignored, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		newer := base.Add(time.Hour)
		if err := os.Chtimes(ignored, newer, newer); err != nil {
			t.Fatal(err)
		}
	}

	got := snapshotMtimes([]string{root})[root]
	if !got.Equal(base) {
		t.Fatalf("snapshot newest = %v, want %v; ignored dirs should not advance mtime", got, base)
	}
}

// TestMtimesChanged covers the comparison logic in isolation: identical
// snapshots are unchanged; added/removed roots and advanced mtimes are changes.
func TestMtimesChanged(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	t1 := t0.Add(time.Minute)

	base := map[string]time.Time{"/a": t0, "/b": t0}

	if mtimesChanged(base, map[string]time.Time{"/a": t0, "/b": t0}) {
		t.Fatal("identical snapshots must not be a change")
	}
	if !mtimesChanged(base, map[string]time.Time{"/a": t1, "/b": t0}) {
		t.Fatal("advanced mtime must be a change")
	}
	if !mtimesChanged(base, map[string]time.Time{"/a": t0}) {
		t.Fatal("removed root must be a change")
	}
	if !mtimesChanged(base, map[string]time.Time{"/a": t0, "/b": t0, "/c": t0}) {
		t.Fatal("added root must be a change")
	}
}
