package watch

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type Event struct {
	Path string
	Op   string
}

type Recursive struct {
	root    string
	watcher *fsnotify.Watcher
	mu      sync.Mutex
	paths   map[string]struct{}
}

func NewRecursive(root string) (*Recursive, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	r := &Recursive{
		root:    absRoot,
		watcher: watcher,
		paths:   make(map[string]struct{}),
	}
	if err := r.addRecursive(absRoot); err != nil {
		_ = watcher.Close()
		return nil, err
	}
	return r, nil
}

func (r *Recursive) Close() error {
	return r.watcher.Close()
}

func (r *Recursive) Run(ctx context.Context, onEvent func(Event)) error {
	defer r.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return nil
			}
			return err
		case event, ok := <-r.watcher.Events:
			if !ok {
				return nil
			}
			if err := r.handleEvent(event, onEvent); err != nil {
				return err
			}
		}
	}
}

func (r *Recursive) handleEvent(event fsnotify.Event, onEvent func(Event)) error {
	path := NormalizePath(event.Name)

	if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		r.removePath(path)
	}

	if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			if err := r.addRecursive(path); err != nil {
				return err
			}
			return nil
		}
	}

	if !IsSQLFile(path) {
		return nil
	}
	if !IsWithinRoot(r.root, path) {
		return nil
	}
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Chmod|fsnotify.Rename) == 0 {
		return nil
	}
	onEvent(Event{Path: path, Op: opName(event.Op)})
	return nil
}

func (r *Recursive) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return r.addPath(path)
	})
}

func (r *Recursive) addPath(path string) error {
	clean := NormalizePath(path)
	r.mu.Lock()
	if _, ok := r.paths[clean]; ok {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	if err := r.watcher.Add(clean); err != nil {
		return err
	}

	r.mu.Lock()
	r.paths[clean] = struct{}{}
	r.mu.Unlock()
	return nil
}

func (r *Recursive) removePath(path string) {
	clean := NormalizePath(path)
	r.mu.Lock()
	if _, ok := r.paths[clean]; !ok {
		r.mu.Unlock()
		return
	}
	delete(r.paths, clean)
	r.mu.Unlock()
	_ = r.watcher.Remove(clean)
}

func opName(op fsnotify.Op) string {
	parts := make([]string, 0, 4)
	if op&fsnotify.Create != 0 {
		parts = append(parts, "CREATE")
	}
	if op&fsnotify.Write != 0 {
		parts = append(parts, "WRITE")
	}
	if op&fsnotify.Chmod != 0 {
		parts = append(parts, "CHMOD")
	}
	if op&fsnotify.Rename != 0 {
		parts = append(parts, "RENAME")
	}
	if op&fsnotify.Remove != 0 {
		parts = append(parts, "REMOVE")
	}
	if len(parts) == 0 {
		return op.String()
	}
	return strings.Join(parts, "+")
}
