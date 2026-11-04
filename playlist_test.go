package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolvePlaylistDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got := resolvePlaylistDir("", "rezka-fzf"); got != filepath.Join(home, "tmp", "rezka-fzf") {
		t.Fatalf("default: %s", got)
	}
	if got := resolvePlaylistDir("~/Videos/lists", "rezka-fzf"); got != filepath.Join(home, "Videos", "lists") {
		t.Fatalf("tilde: %s", got)
	}
	if got := resolvePlaylistDir("/var/playlists", "rezka-fzf"); got != "/var/playlists" {
		t.Fatalf("abs: %s", got)
	}
}

func TestWriteAndListPlaylists(t *testing.T) {
	dir := t.TempDir()
	older, err := writePlaylist(dir, "old.m3u", []byte("#EXTM3U\n"))
	if err != nil {
		t.Fatal(err)
	}
	newer, err := writePlaylist(dir, "new.m3u", []byte("#EXTM3U\n"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(older, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := listPlaylists(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != newer || got[1] != older {
		t.Fatalf("got %v", got)
	}
}
