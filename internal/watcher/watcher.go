package watcher

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchedExts are the file extensions that trigger a rebuild.
var WatchedExts = map[string]bool{
	".tex": true,
	".bib": true,
	".sty": true,
	".cls": true,
}

// BuildFn is called each time a watched file changes.
type BuildFn func() error

// Watch starts watching the current directory tree and calls build on changes.
// It blocks until ctx is done or a fatal watcher error occurs.
func Watch(root string, build BuildFn, log *slog.Logger) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer w.Close()

	if err := addDirs(w, root); err != nil {
		return err
	}

	log.Info("watching for changes", "root", root, "exts", ".tex .bib .sty .cls")

	// Run initial build.
	printDivider("initial build")
	if err := build(); err != nil {
		log.Error("build failed", "err", err)
	}

	var debounce *time.Timer
	debounceDelay := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !isRelevant(event) {
				continue
			}
			// Debounce: reset timer on each new event.
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(debounceDelay, func() {
				printDivider(fmt.Sprintf("changed: %s", event.Name))
				if err := build(); err != nil {
					log.Error("build failed", "err", err)
				}
			})

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			log.Warn("watcher error", "err", err)
		}
	}
}

func isRelevant(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return false
	}
	return WatchedExts[filepath.Ext(event.Name)]
}

func addDirs(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return w.Add(path)
		}
		return nil
	})
}

func printDivider(label string) {
	ts := time.Now().Format("15:04:05")
	noColor := os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
	line := strings.Repeat("─", 40)
	if noColor {
		fmt.Printf("\n[%s] %s %s\n\n", ts, label, line)
	} else {
		fmt.Printf("\n\033[36m[%s] %s %s\033[0m\n\n", ts, label, line)
	}
}
