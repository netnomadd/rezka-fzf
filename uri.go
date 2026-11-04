package main

import (
	"net/url"
	"strings"
)

// normalizeLaunchArg разбирает аргумент запуска.
//
// Схема rezka: (её регистрирует rezka-fzf.desktop):
//
//	rezka://434365
//	rezka://434365-pantheon.html
//	rezka://films/drama/123-title.html
//	rezka://hdrezka.ag/123-title.html
//	rezka://https://hdrezka.ag/123-title.html
//	rezka://favorites
//	rezka://search/хищник
//	rezka://search?q=хищник+1987
//
// Без схемы строка возвращается как есть: её уже понимает -url
// (полный http(s), slug или ID).
func normalizeLaunchArg(raw string) (direct string, favorites bool, search string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, ""
	}

	const scheme = "rezka:"
	if len(raw) < len(scheme) || !strings.EqualFold(raw[:len(scheme)], scheme) {
		return raw, false, ""
	}

	rest := raw[len(scheme):]
	if strings.HasPrefix(rest, "//") {
		rest = rest[2:]
	}
	if dec, err := url.PathUnescape(rest); err == nil {
		rest = dec
	}
	// Qt/KDE разбирает rezka://https://host/path как host=https и path=//host/path,
	// затем отдаёт приложению rezka://https//host/path. Возвращаем нормальный URL.
	rest = repairEmbeddedHTTP(rest)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", false, ""
	}

	base, query, _ := strings.Cut(rest, "?")
	if before, _, ok := strings.Cut(base, "#"); ok {
		base = before
	}
	base = strings.TrimRight(base, "/")
	key := strings.Trim(base, "/")
	if strings.EqualFold(key, "favorites") || strings.EqualFold(key, "favs") {
		return "", true, ""
	}
	if q, ok := searchQuery(key, query); ok {
		return "", false, q
	}

	low := strings.ToLower(base)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		if query != "" {
			return base + "?" + query, false, ""
		}
		return base, false, ""
	}

	if host, path, ok := splitHostPath(base); ok {
		u := "https://" + host + "/" + strings.TrimLeft(path, "/")
		if query != "" {
			u += "?" + query
		}
		return u, false, ""
	}

	return strings.TrimLeft(base, "/"), false, ""
}

// searchQuery читает rezka://search/текст и rezka://search?q=текст.
// Плюс в запросе — пробел, как у url.QueryEscape, которым Gofer подставляет %s.
func searchQuery(key, rawQuery string) (string, bool) {
	head, tail, _ := strings.Cut(strings.Trim(key, "/"), "/")
	if !strings.EqualFold(head, "search") {
		return "", false
	}
	q := tail
	if q == "" {
		q = queryValue(rawQuery, "q")
	}
	if dec, err := url.QueryUnescape(q); err == nil {
		q = dec
	}
	return strings.TrimSpace(q), true
}

func queryValue(raw, key string) string {
	for _, part := range strings.Split(raw, "&") {
		k, v, ok := strings.Cut(part, "=")
		if ok && strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// repairEmbeddedHTTP чинит URL, который Qt склеил, выкинув двоеточие из https://.
// https//host/path и https:/host/path становятся https://host/path. http — так же.
func repairEmbeddedHTTP(rest string) string {
	low := strings.ToLower(rest)
	for _, scheme := range []string{"https", "http"} {
		if !strings.HasPrefix(low, scheme) {
			continue
		}
		tail := rest[len(scheme):]
		switch {
		case strings.HasPrefix(tail, "://"):
			return scheme + tail
		case strings.HasPrefix(tail, "//"):
			return scheme + "://" + tail[2:]
		case strings.HasPrefix(tail, ":/"):
			return scheme + "://" + tail[2:]
		case strings.HasPrefix(tail, ":"):
			return scheme + "://" + tail[1:]
		}
	}
	return rest
}

// splitHostPath отличает rezka://hdrezka.ag/slug от rezka://123-title.html.
// Хост — первый сегмент с точкой, и после него есть путь.
func splitHostPath(base string) (host, path string, ok bool) {
	base = strings.TrimLeft(base, "/")
	host, path, found := strings.Cut(base, "/")
	if !found || host == "" || path == "" {
		return "", "", false
	}
	if strings.ContainsAny(host, " @:") || !strings.Contains(host, ".") {
		return "", "", false
	}
	return host, path, true
}

// launchTarget собирает цель из -url и позиционных аргументов (их передаёт desktop-файл, %u).
func launchTarget(flagURL string, favorites bool, args []string) (direct string, openFavorites bool, search string) {
	openFavorites = favorites
	take := func(raw string) {
		if strings.TrimSpace(raw) == "" {
			return
		}
		d, fav, q := normalizeLaunchArg(raw)
		if fav {
			openFavorites = true
		}
		if q != "" && search == "" {
			search = q
		}
		if d != "" && direct == "" {
			direct = d
		}
	}
	take(flagURL)
	for _, arg := range args {
		take(arg)
	}
	return direct, openFavorites, search
}
