package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func defaultPlaylistDir(app string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), app)
	}
	return filepath.Join(home, "tmp", app)
}

func expandHome(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func resolvePlaylistDir(configured, app string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return defaultPlaylistDir(app)
	}
	return expandHome(configured)
}

func writePlaylist(dir, name string, body []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func listPlaylists(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type file struct {
		path string
		mod  time.Time
	}
	var files []file
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".m3u") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, file{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.path
	}
	return out, nil
}

func (a *App) browsePlaylists() {
	dir := a.cfg.playlistDir()
	files, err := listPlaylists(dir)
	if err != nil {
		a.msg("Playlists: %v", err)
		return
	}
	if len(files) == 0 {
		a.msg("No playlists in %s", dir)
		return
	}
	items := make([]Item, 0, len(files)+1)
	for _, f := range files {
		items = append(items, Item{Display: filepath.Base(f), Value: f})
	}
	items = append(items, Item{Display: "── Remove playlist ──", Value: ""})
	idx, ok := a.menu.Select("Playlists", items)
	if !ok {
		return
	}
	if idx == len(files) {
		a.removePlaylist(files)
		return
	}
	path := files[idx]
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if err := a.player.Play(path, title); err != nil {
		a.msg("Playback error: %v", err)
	}
}

func (a *App) removePlaylist(files []string) {
	items := make([]Item, len(files))
	for i, f := range files {
		items[i] = Item{Display: filepath.Base(f), Value: f}
	}
	idx, ok := a.menu.Select("Remove playlist", items)
	if !ok {
		return
	}
	if err := os.Remove(files[idx]); err != nil {
		a.msg("Error: %v", err)
		return
	}
	a.msg("Removed %s", filepath.Base(files[idx]))
}
