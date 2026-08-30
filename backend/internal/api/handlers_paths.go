package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

type pathEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// listServerPaths lets an admin browse directories on the server when adding a
// media library, instead of typing the absolute path by hand.
func (a *App) listServerPaths(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	home, _ := os.UserHomeDir()
	if path == "" {
		path = home
	}
	if path == "" {
		path = "/"
	}
	// Normalize user input into a clean, absolute path. Prepending "/" makes
	// relative input resolve against the filesystem root and lets filepath.Clean
	// clamp any ".." segments below the root, so the path can never escape it.
	path = filepath.Clean("/" + path)

	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		if home != "" && home != path {
			path = home
		} else {
			path = "/"
		}
	}

	entries, err := os.ReadDir(path)
	dirs := []pathEntry{}
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			dirs = append(dirs, pathEntry{
				Name: e.Name(),
				Path: filepath.Join(path, e.Name()),
			})
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	parent := ""
	if path != "/" {
		parent = filepath.Dir(path)
	}

	readErr := ""
	if err != nil {
		readErr = "cannot read directory: " + err.Error()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"current":    path,
		"parent":     parent,
		"home":       home,
		"dirs":       dirs,
		"free_bytes": freeBytes(path),
		"error":      readErr,
	})
}

func freeBytes(path string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
