// Package file_watcher_lib provides a polling file watcher that reports
// files created, modified, or removed in a directory.
package file_watcher_lib

import (
	"os"
	"path/filepath"
	"time"
)

// EventType describes what happened to a file.
type EventType int

const (
	// Created indicates a file that did not exist in the previous scan.
	Created EventType = iota
	// Modified indicates a file whose modification time or size changed.
	Modified
	// Removed indicates a file that was present in the previous scan but is now gone.
	Removed
)

// Event represents a single change detected by the watcher.
type Event struct {
	// Path is the absolute path to the file that changed.
	Path string
	// Type is the kind of change that occurred.
	Type EventType
}

// Watcher polls a directory for changes.
type Watcher struct {
	dir      string
	interval time.Duration
	now      func() time.Time
	files    map[string]fileState
}

type fileState struct {
	modTime time.Time
	size    int64
}

// New creates a Watcher for the given directory. It polls at the given interval.
// The now function is used to timestamp scans; pass time.Now in production and a
// fake clock in tests so behaviour is deterministic.
func New(dir string, interval time.Duration, now func() time.Time) *Watcher {
	return &Watcher{
		dir:      dir,
		interval: interval,
		now:      now,
		files:    make(map[string]fileState),
	}
}

// Scan performs one poll of the directory and returns the events since the
// previous scan. The first scan reports every existing file as Created.
func (w *Watcher) Scan() ([]Event, error) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]fileState, len(entries))
	var events []Event

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			// The file may have been removed between ReadDir and Info; skip it.
			continue
		}
		path := filepath.Join(w.dir, entry.Name())
		state := fileState{modTime: info.ModTime(), size: info.Size()}
		seen[path] = state

		prev, ok := w.files[path]
		if !ok {
			events = append(events, Event{Path: path, Type: Created})
			continue
		}
		if !prev.modTime.Equal(state.modTime) || prev.size != state.size {
			events = append(events, Event{Path: path, Type: Modified})
		}
	}

	for path := range w.files {
		if _, ok := seen[path]; !ok {
			events = append(events, Event{Path: path, Type: Removed})
		}
	}

	w.files = seen
	return events, nil
}

// Run polls the directory every interval and sends events on the returned
// channel until stop is closed. The channel is closed when Run returns.
// Callers must drain the channel; Run blocks until stop is closed.
func (w *Watcher) Run(stop <-chan struct{}) <-chan Event {
	ch := make(chan Event)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				events, err := w.Scan()
				if err != nil {
					continue
				}
				for _, ev := range events {
					select {
					case ch <- ev:
					case <-stop:
						return
					}
				}
			}
		}
	}()
	return ch
}
