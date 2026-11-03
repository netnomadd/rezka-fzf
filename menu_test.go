package main

import (
	"strings"
	"testing"
)

func TestM3UPlaylist(t *testing.T) {
	got := m3uPlaylist([]m3uEntry{
		{title: "Show, S01E01", url: "https://cdn.example/1.mp4"},
		{title: "Show S01E02", url: "https://cdn.example/2.mp4"},
	})
	want := "#EXTM3U\n#EXTINF:-1,Show S01E01\nhttps://cdn.example/1.mp4\n#EXTINF:-1,Show S01E02\nhttps://cdn.example/2.mp4\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestGoferExec(t *testing.T) {
	g := newGoferMenu("gofer --theme=nord --prompt {prompt}")

	in := g.exec("--input", "Search")
	if got, want := strings.Join(in.Args[1:], " "), "--theme=nord --input=Search"; got != want {
		t.Fatalf("input: %s", got)
	}

	sel := g.exec("--prompt", "Select Translation")
	if got, want := strings.Join(sel.Args[1:], " "), "--theme=nord --prompt=Select Translation"; got != want {
		t.Fatalf("select: %s", got)
	}

	eq := newGoferMenu("gofer --prompt={prompt}")
	sel = eq.exec("--prompt", "Season")
	if got, want := strings.Join(sel.Args[1:], " "), "--prompt=Season"; got != want {
		t.Fatalf("equals form: %s", got)
	}
}
