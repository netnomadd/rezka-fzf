package main

import (
	"strings"
	"testing"
)

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
