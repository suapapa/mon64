package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch monitors the config file and calls reload on changes.
// The directory is watched so atomic editor renames are detected.
func Watch(ctx context.Context, path string, reload func() error, log *slog.Logger) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(dir); err != nil {
		return err
	}

	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	trigger := func(reason string) {
		debounce.Reset(200 * time.Millisecond)
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-debounce.C:
				if err := reload(); err != nil {
					log.Error("config reload failed", "reason", reason, "err", err)
					return
				}
				log.Info("config reloaded", "reason", reason, "path", abs)
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-watcher.Errors:
			if err != nil {
				log.Error("config watcher error", "err", err)
			}
		case ev := <-watcher.Events:
			if ev.Name != "" && filepath.Base(ev.Name) != base {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				trigger(ev.Op.String())
			}
		}
	}
}

// FileExists reports whether path is readable.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
