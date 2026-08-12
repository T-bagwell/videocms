package api

import "context"

// StartFileWatcher runs the background incremental-indexing loop until ctx is
// cancelled. It is a no-op when watching is disabled (WATCH_INTERVAL=0).
func (a *App) StartFileWatcher(ctx context.Context) {
	go a.scanner.Watch(ctx, a.cfg.WatchInterval)
}
