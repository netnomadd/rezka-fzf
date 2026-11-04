package main

import "testing"

func TestNormalizeLaunchArg(t *testing.T) {
	cases := []struct {
		in, direct, search string
		fav                bool
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
		{in: "rezka://search/хищник", search: "хищник"},
		{in: "rezka://search?q=%D1%85%D0%B8%D1%89%D0%BD%D0%B8%D0%BA+1987", search: "хищник 1987"},
		{in: "rezka://search/", direct: ""},
	}
	for _, tc := range cases {
		direct, fav, search := normalizeLaunchArg(tc.in)
		if direct != tc.direct || fav != tc.fav || search != tc.search {
			t.Errorf("normalizeLaunchArg(%q) = (%q, %v, %q), want (%q, %v, %q)", tc.in, direct, fav, search, tc.direct, tc.fav, tc.search)
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
	direct, fav, search := launchTarget("", false, []string{"rezka://434365"})
	if direct != "434365" || fav || search != "" {
		t.Fatalf("positional uri: got (%q, %v, %q)", direct, fav, search)
	}

	direct, fav, search = launchTarget("10", true, []string{"rezka://favorites"})
	if direct != "10" || !fav || search != "" {
		t.Fatalf("flag wins over favorites uri: got (%q, %v, %q)", direct, fav, search)
	}

	direct, fav, search = launchTarget("rezka://favorites", false, nil)
	if direct != "" || !fav || search != "" {
		t.Fatalf("-url favorites: got (%q, %v, %q)", direct, fav, search)
	}

	direct, fav, search = launchTarget("", false, []string{"rezka://111", "rezka://222"})
	if direct != "111" || fav || search != "" {
		t.Fatalf("first positional: got (%q, %v, %q)", direct, fav, search)
	}

	_, _, search = launchTarget("", false, []string{"rezka://search?q=хищник+1987"})
	if search != "хищник 1987" {
		t.Fatalf("search uri: %q", search)
	}
}
