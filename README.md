# file_watcher_lib

Polls a directory at a configurable interval and reports files that were created, modified, or removed.

```go
package main

import (
	"fmt"
	"time"

	"file_watcher_lib"
)

func main() {
	w := file_watcher_lib.New("./data", time.Second, time.Now)
	stop := make(chan struct{})
	defer close(stop)

	for ev := range w.Run(stop) {
		switch ev.Type {
		case file_watcher_lib.Created:
			fmt.Println("created", ev.Path)
		case file_watcher_lib.Modified:
			fmt.Println("modified", ev.Path)
		case file_watcher_lib.Removed:
			fmt.Println("removed", ev.Path)
		}
	}
}
```

## Why this exists

Watching a directory for changes without pulling in a platform-specific library means
polling. The trade-off is latency versus portability: a poll can miss a file that is
created and removed between scans, and it costs a directory read every interval, but it
works the same on every operating system and needs nothing beyond the standard library.

## The awkward edge

The first `Scan` reports every file that already exists as `Created`, because the watcher
has no prior state to compare against. If you only care about changes after startup, call
`Scan` once and discard its events before you start reacting.

Modification detection compares both the modification time and the size. A file whose
contents change but whose size and modification time stay identical will not be reported;
this is a deliberate limit of a polling watcher that does not read file contents.

## API

- `New(dir string, interval time.Duration, now func() time.Time) *Watcher` — creates a
  watcher. The `now` function is used for timestamps; pass `time.Now` in production and a
  fake clock in tests.
- `(*Watcher) Scan() ([]Event, error)` — performs one poll and returns the events since
  the previous scan. Returns an error if the directory cannot be read.
- `(*Watcher) Run(stop <-chan struct{}) <-chan Event` — polls every interval and sends
  events until `stop` is closed, then closes the returned channel.
- `Event{Path string, Type EventType}` — a single change.
- `Created`, `Modified`, `Removed` — the `EventType` values.
