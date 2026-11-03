package main

import "testing"

func TestNormalizeLaunchArg(t *testing.T) {
	cases := []struct {
		in     string
		direct string
		fav    bool
	}{
		{in: "", direct: ""},
		{in: "434365", direct: "434365"},
		{in: "434365-pantheon.html", direct: "434365-pantheon.html"},
		{in: "https://hdrezka.ag/434365-pantheon.html", direct: "https://hdrezka.ag/434365-pantheon.html"},
		{in: "rezka://434365", direct: "434365"},
		{in: "rezka:434365-pantheon.html", direct: "434365-pantheon.html"},
		{in: "rezka:///films/drama/123-title.html", direct: "films/drama/123-title.html"},
		{in: "rezka://hdrezka.ag/123-title.html", direct: "https://hdrezka.ag/123-title.html"},
		{in: "rezka://hdrezka.ag/films/1-a.html?x=1", direct: "https://hdrezka.ag/films/1-a.html?x=1"},
		{in: "rezka://https://hdrezka.ag/434365-pantheon.html", direct: "https://hdrezka.ag/434365-pantheon.html"},
		{in: "rezka://https%3A%2F%2Fhdrezka.ag%2F1-a.html", direct: "https://hdrezka.ag/1-a.html"},
		{in: "rezka://https//hdrezka.name/films/action/1452-hischnik-1987.html", direct: "https://hdrezka.name/films/action/1452-hischnik-1987.html"},
		{in: "rezka://http//hdrezka.name/films/a.html", direct: "http://hdrezka.name/films/a.html"},
		{in: "rezka://https:/hdrezka.name/films/a.html", direct: "https://hdrezka.name/films/a.html"},
		{in: "REZKA://favorites", fav: true},
		{in: "rezka://favorites/", fav: true},
		{in: "rezka://favs", fav: true},
		{in: "rezka://", direct: ""},
		{in: "rezka:///", direct: ""},
		{in: "rezka://434365#t", direct: "434365"},
	}
	for _, tc := range cases {
		direct, fav := normalizeLaunchArg(tc.in)
		if direct != tc.direct || fav != tc.fav {
			t.Errorf("normalizeLaunchArg(%q) = (%q, %v), want (%q, %v)", tc.in, direct, fav, tc.direct, tc.fav)
		}
	}
}

func TestContentURL(t *testing.T) {
	base := "https://hdrezka.name"
	if got := contentURL(base, "1452"); got != "https://hdrezka.name/?newsid=1452" {
		t.Fatalf("id: %s", got)
	}
	if got := contentURL(base+"/", "films/action/1452-hischnik-1987.html"); got != base+"/films/action/1452-hischnik-1987.html" {
		t.Fatalf("slug: %s", got)
	}
	full := "https://hdrezka.name/films/action/1452-hischnik-1987.html"
	if got := contentURL(base, full); got != full {
		t.Fatalf("absolute: %s", got)
	}
}

func TestLaunchTarget(t *testing.T) {
	direct, fav := launchTarget("", false, []string{"rezka://434365"})
	if direct != "434365" || fav {
		t.Fatalf("positional uri: got (%q, %v)", direct, fav)
	}

	direct, fav = launchTarget("10", true, []string{"rezka://favorites"})
	if direct != "10" || !fav {
		t.Fatalf("flag wins over favorites uri: got (%q, %v)", direct, fav)
	}

	direct, fav = launchTarget("rezka://favorites", false, nil)
	if direct != "" || !fav {
		t.Fatalf("-url favorites: got (%q, %v)", direct, fav)
	}

	direct, fav = launchTarget("", false, []string{"rezka://111", "rezka://222"})
	if direct != "111" || fav {
		t.Fatalf("first positional: got (%q, %v)", direct, fav)
	}
}
