package file_watcher_lib

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeClock returns a function that yields a fixed time, so tests never depend
// on wall-clock time.
func fakeClock() func() time.Time {
	t := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func TestScanReportsCreatedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := New(dir, time.Second, fakeClock())
	events, err := w.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	if events[0].Type != Created {
		t.Errorf("expected Created, got %v", events[0].Type)
	}
	if events[0].Path != filepath.Join(dir, "a.txt") {
		t.Errorf("unexpected path: %s", events[0].Path)
	}
}

func TestScanReportsModifiedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := New(dir, time.Second, fakeClock())
	if _, err := w.Scan(); err != nil {
		t.Fatal(err)
	}

	// Change size to guarantee a detectable difference without relying on mtime
	// resolution.
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := w.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	if events[0].Type != Modified {
		t.Errorf("expected Modified, got %v", events[0].Type)
	}
}

func TestScanReportsRemovedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := New(dir, time.Second, fakeClock())
	if _, err := w.Scan(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	events, err := w.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	if events[0].Type != Removed {
		t.Errorf("expected Removed, got %v", events[0].Type)
	}
}

func TestScanIgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := New(dir, time.Second, fakeClock())
	events, err := w.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected only the file event, got %+v", events)
	}
	if events[0].Path != filepath.Join(dir, "a.txt") {
		t.Errorf("unexpected path: %s", events[0].Path)
	}
}

func TestScanNoChangeProducesNoEvents(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := New(dir, time.Second, fakeClock())
	if _, err := w.Scan(); err != nil {
		t.Fatal(err)
	}
	events, err := w.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %+v", events)
	}
}

func TestScanMissingDirectoryReturnsError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	w := New(dir, time.Second, fakeClock())
	if _, err := w.Scan(); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestRunEmitsEventsUntilStopped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := New(dir, 10*time.Millisecond, fakeClock())
	stop := make(chan struct{})
	ch := w.Run(stop)

	select {
	case ev := <-ch:
		if ev.Type != Created {
			t.Errorf("expected Created, got %v", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	close(stop)
	// Drain until the channel closes; this proves Run exits cleanly.
	for range ch {
	}
}
