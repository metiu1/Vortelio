package server

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// ── Read-only filesystem browsing for the Developer view ─────────────────────
//
// These endpoints back the Developer file explorer. They are read-only and
// intended for the local single-user app (the agentic coding tools already
// have full filesystem access). Both take an absolute ?path= query.

type fsEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
}

// GET /api/fs/list?path=...
// With an empty path it returns the filesystem roots (home + drives) so the
// folder picker has a starting point.
func handleFsList(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		respond(w, 200, map[string]interface{}{"path": "", "roots": true, "entries": fsRoots()})
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		jsonError(w, 404, "path not found")
		return
	}
	if !info.IsDir() {
		jsonError(w, 400, "path is not a directory")
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	out := make([]fsEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		// Skip noisy hidden/vendor dirs to keep the tree readable.
		if name == ".git" || name == "node_modules" {
			continue
		}
		out = append(out, fsEntry{Name: name, Path: filepath.Join(path, name), Dir: e.IsDir()})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir // directories first
		}
		return out[i].Name < out[j].Name
	})
	respond(w, 200, map[string]interface{}{"path": path, "entries": out})
}

// fsRoots returns useful starting points: the user's home directory plus any
// available drives (Windows) or "/" (Unix).
func fsRoots() []fsEntry {
	var out []fsEntry
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		out = append(out, fsEntry{Name: "🏠 Home", Path: home, Dir: true})
	}
	if runtime.GOOS == "windows" {
		for c := 'C'; c <= 'Z'; c++ {
			d := string(c) + ":\\"
			if _, err := os.Stat(d); err == nil {
				out = append(out, fsEntry{Name: d, Path: d, Dir: true})
			}
		}
	} else {
		out = append(out, fsEntry{Name: "/", Path: "/", Dir: true})
	}
	return out
}

// GET /api/fs/read?path=...
func handleFsRead(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		jsonError(w, 400, "path is required")
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		jsonError(w, 404, "file not found")
		return
	}
	if info.IsDir() {
		jsonError(w, 400, "path is a directory")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	const maxLen = 200 * 1024
	truncated := false
	if len(data) > maxLen {
		data = data[:maxLen]
		truncated = true
	}
	respond(w, 200, map[string]interface{}{"path": path, "content": string(data), "truncated": truncated})
}
