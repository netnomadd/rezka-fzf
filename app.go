package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/n0madic/go-hdrezka"
)

type epsEntry struct {
	eps hdrezka.Episodes
	err error
}

type epRef struct{ season, episode int }

type App struct {
	cfg       *Config
	menu      Menu
	player    *Player
	r         *hdrezka.HDRezka
	preferHLS bool

	// текущий тайтл
	video    *hdrezka.Video
	itemID   int
	slug     string
	tr       *hdrezka.Translation // выбранная озвучка (nil — ещё не выбрана)
	isSeries bool
	epsCache map[string]epsEntry
}

func NewApp(cfg *Config, m Menu, preferHLS bool) *App {
	return &App{cfg: cfg, menu: m, player: NewPlayer(cfg.Player, cfg.PlayerArgs), preferHLS: preferHLS}
}

// msg печатает сообщение в stderr, а в режиме gofer (обычно без терминала) ещё и уведомление.
func (a *App) msg(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, s)
	if a.menu.Name() == "gofer" && have("notify-send") {
		_ = exec.Command("notify-send", "rezka-fzf", s).Run()
	}
}

func (a *App) base() string {
	if a.r != nil && a.r.URL != nil {
		return strings.TrimRight(a.r.URL.String(), "/")
	}
	return a.cfg.Mirror()
}

func (a *App) setup() error {
	r := hdrezka.New().WithMirrors(a.cfg.Mirror())
	if a.cfg.Proxy != "" {
		r.WithProxy(a.cfg.Proxy)
	}
	if a.cfg.Cookies != "" {
		r.WithCookies(a.cfg.Cookies)
	}
	if err := r.Init(); err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	if a.cfg.Username != "" && a.cfg.Password != "" {
		if err := r.Login(a.cfg.Username, a.cfg.Password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	}
	a.r = r
	a.cfg.UpdateFavoritesMirror(a.base())
	return nil
}

// ---------- вспомогательное ----------

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

func lastSegment(u string) string {
	u = strings.TrimRight(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}

func extractID(u string) int {
	part, _, _ := strings.Cut(lastSegment(u), "-")
	part = strings.TrimSuffix(part, ".html")
	if n, err := strconv.Atoi(part); err == nil {
		return n
	}
	return 0
}

func safeName(s string) string {
	return strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(s)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func translationLabel(t *hdrezka.Translation) string {
	l := t.Name
	if t.IsPremium {
		l += " [PREMIUM]"
	}
	if t.IsAds {
		l += " [ADS]"
	}
	if t.IsCamRip {
		l += " [CAMRIP]"
	}
	return l
}

func watchedString(w map[string][]int) string {
	keys := make([]string, 0, len(w))
	for k := range w {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		strs := make([]string, len(w[k]))
		for i, e := range w[k] {
			strs[i] = strconv.Itoa(e)
		}
		parts = append(parts, k+":"+strings.Join(strs, ","))
	}
	return strings.Join(parts, " ")
}

// streamURLs возвращает ссылки выбранного качества: MP4 и HLS (если есть).
func streamURLs(s *hdrezka.Stream, quality string, preferHLS bool) []string {
	f := s.Formats[quality]
	urls := []string{f.MP4, f.HLS}
	if preferHLS {
		urls = []string{f.HLS, f.MP4}
	}
	var out []string
	for _, u := range urls {
		if u != "" {
			out = append(out, u)
		}
	}
	return out
}

// ---------- загрузка тайтла ----------

func (a *App) load(url string, id int) bool {
	v, err := a.r.GetVideo(url)
	if err != nil {
		a.msg("Error loading content: %v (%s)", err, url)
		return false
	}
	if len(v.Translation) == 0 {
		a.msg("No translations available")
		return false
	}
	if id == 0 {
		id = extractID(url)
	}
	if id == 0 {
		id, _ = strconv.Atoi(v.ID)
	}
	a.video, a.itemID, a.slug = v, id, lastSegment(url)
	a.tr = nil
	a.epsCache = map[string]epsEntry{}

	// Сериал определяем по наличию эпизодов у озвучки по умолчанию.
	a.isSeries = false
	if eps, err := a.episodesFor(a.defaultTranslation()); err == nil && len(eps) > 0 {
		a.isSeries = true
	}
	return true
}

func (a *App) defaultTranslation() *hdrezka.Translation {
	for _, t := range a.video.Translation {
		if t.IsDefault {
			return t
		}
	}
	return a.video.Translation[0]
}

func (a *App) episodesFor(tr *hdrezka.Translation) (hdrezka.Episodes, error) {
	if e, ok := a.epsCache[tr.ID]; ok {
		return e.eps, e.err
	}
	eps, err := tr.GetEpisodes()
	a.epsCache[tr.ID] = epsEntry{eps, err}
	return eps, err
}

// ensureTranslation выбирает озвучку, если ещё не выбрана.
func (a *App) ensureTranslation() bool {
	if a.tr != nil {
		return true
	}
	return a.chooseTranslation()
}

func (a *App) chooseTranslation() bool {
	trs := a.video.Translation
	if len(trs) == 1 {
		a.tr = trs[0]
		return true
	}
	items := make([]Item, len(trs))
	for i, t := range trs {
		items[i] = Item{Display: translationLabel(t), Value: t}
	}
	idx, ok := a.menu.Select("Select Translation", items)
	if !ok {
		return false
	}
	a.tr = trs[idx]
	return true
}

// ---------- поиск и избранное ----------

func (a *App) searchAndSelect() (url string, id int, ok bool) {
	query, ok := a.menu.Input("Search")
	if !ok {
		return "", 0, false
	}
	return a.searchQuery(query)
}

func (a *App) searchQuery(query string) (url string, id int, ok bool) {
	results, err := a.r.Search(query, 50)
	if err != nil {
		a.msg("Search error: %v", err)
		return "", 0, false
	}
	if len(results) == 0 {
		a.msg("No results found")
		return "", 0, false
	}

	favIDs := map[int]bool{}
	for _, f := range a.cfg.Favorites() {
		favIDs[f.ID] = true
	}
	items := make([]Item, len(results))
	for i, it := range results {
		rid := extractID(it.URL)
		mark := ""
		if favIDs[rid] {
			mark = "★ "
		}
		idStr := ""
		if rid != 0 {
			idStr = fmt.Sprintf("[%d] ", rid)
		}
		extra := ""
		if info := clean(it.Info); info != "" {
			extra = " (" + info + ")"
		}
		items[i] = Item{Display: mark + idStr + clean(it.Title) + extra, Value: it}
	}
	idx, ok := a.menu.Select("Select Content", items)
	if !ok {
		return "", 0, false
	}
	return results[idx].URL, extractID(results[idx].URL), true
}

func (a *App) browseFavorites() (url string, id int, ok bool) {
	favs := a.cfg.Favorites()
	if len(favs) == 0 {
		a.msg("No favorites yet...")
		return "", 0, false
	}
	items := make([]Item, 0, len(favs)+1)
	for _, f := range favs {
		d := fmt.Sprintf("ID: %d", f.ID)
		if f.Type != "" {
			d += fmt.Sprintf(" [%s]", f.Type)
		}
		if f.URLShort != "" {
			d += fmt.Sprintf(" (%s)", f.URLShort)
		}
		if len(f.Watched) > 0 {
			d += "  ✓ " + watchedString(f.Watched)
		}
		items = append(items, Item{Display: d, Value: f})
	}
	items = append(items, Item{Display: "── Remove from favorites ──", Value: "__remove__"})

	idx, ok := a.menu.Select("Favorites", items)
	if !ok {
		return "", 0, false
	}
	if idx == len(favs) {
		a.removeFavoriteMenu(favs)
		return "", 0, false
	}
	f := favs[idx]
	switch {
	case f.URLShort != "":
		url = contentURL(a.base(), f.URLShort)
	default:
		url = contentURL(a.base(), strconv.Itoa(f.ID))
	}
	return url, f.ID, true
}

func (a *App) removeFavoriteMenu(favs []Favorite) {
	items := make([]Item, len(favs))
	for i, f := range favs {
		t := f.Type
		if t == "" {
			t = "unknown"
		}
		items[i] = Item{Display: fmt.Sprintf("ID: %d [%s]", f.ID, t), Value: f}
	}
	idx, ok := a.menu.Select("Remove", items)
	if !ok {
		return
	}
	if err := a.cfg.RemoveFavorite(favs[idx].ID); err != nil {
		a.msg("Error: %v", err)
		return
	}
	a.msg("✗ Removed ID %d from favorites", favs[idx].ID)
}

// ---------- меню тайтла ----------

func (a *App) contentMenu() (string, bool) {
	v := a.video
	kind := "Movie"
	if a.isSeries {
		kind = "Series"
	}
	inFav := a.itemID != 0 && a.cfg.FindFavorite(a.itemID) != nil

	items := []Item{{Display: "Watch", Value: "watch"}}
	if len(v.Translation) > 1 {
		cur := "not chosen"
		if a.tr != nil {
			cur = a.tr.Name
		}
		items = append(items, Item{Display: "Translation: " + cur + " (change)", Value: "translation"})
	}
	if inFav {
		items = append(items, Item{Display: "Remove from favorites", Value: "fav_remove"})
	} else {
		items = append(items, Item{Display: "Add to favorites", Value: "fav_add"})
	}
	if a.isSeries {
		items = append(items, Item{Display: "Save playlist (.m3u)", Value: "export_series"})
	} else {
		items = append(items, Item{Display: "Save playlist (.m3u)", Value: "export_movie"})
	}

	idStr := ""
	if a.itemID != 0 {
		idStr = fmt.Sprintf(" [%d]", a.itemID)
	}
	items = append(items,
		Item{Display: "━━━ Info ━━━", Value: ""},
		Item{Display: "Title: " + clean(v.Title) + idStr, Value: ""},
		Item{Display: "Original: " + clean(v.TitleOriginal), Value: ""},
	)
	if d := clean(v.Description); d != "" {
		items = append(items, Item{Display: "Description: " + truncate(d, 150), Value: ""})
	}
	year := v.Year
	if year == "" {
		year = v.ReleaseDate
	}
	items = append(items,
		Item{Display: "Year: " + year, Value: ""},
		Item{Display: "Type: " + kind, Value: ""},
	)
	if r := ratingString(v); r != "" {
		items = append(items, Item{Display: "Rating: " + r, Value: ""})
	}
	if inFav && a.isSeries {
		w := a.cfg.Watched(a.itemID)
		keys := make([]string, 0, len(w))
		for k := range w {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			strs := make([]string, len(w[k]))
			for i, e := range w[k] {
				strs[i] = strconv.Itoa(e)
			}
			items = append(items, Item{
				Display: fmt.Sprintf("Watched %s: episodes %s", k, strings.Join(strs, ", ")),
				Value:   "",
			})
		}
	}

	idx, ok := a.menu.Select(kind+" Menu", items)
	if !ok {
		return "", false
	}
	return items[idx].Value.(string), true
}

func ratingString(v *hdrezka.Video) string {
	var parts []string
	if v.Rating.Score > 0 {
		parts = append(parts, fmt.Sprintf("%.1f", v.Rating.Score))
	}
	if v.RatingIMDB.Score > 0 {
		parts = append(parts, fmt.Sprintf("IMDb %.1f", v.RatingIMDB.Score))
	}
	if v.RatingKinopoisk.Score > 0 {
		parts = append(parts, fmt.Sprintf("KP %.1f", v.RatingKinopoisk.Score))
	}
	return strings.Join(parts, ", ")
}

func (a *App) contentLoop() {
	for {
		action, ok := a.contentMenu()
		if !ok {
			return
		}
		switch action {
		case "watch":
			a.watch()
		case "translation":
			if a.chooseTranslation() {
				// у другой озвучки может быть другой набор серий
				if eps, err := a.episodesFor(a.tr); err == nil && len(eps) > 0 {
					a.isSeries = true
				}
			}
		case "fav_add":
			if a.itemID != 0 {
				typ := "movie"
				if a.isSeries {
					typ = "series"
				}
				if err := a.cfg.AddFavorite(a.itemID, typ, a.slug, a.base()); err != nil {
					a.msg("Error: %v", err)
				} else {
					a.msg("★ Added to favorites")
				}
			}
		case "fav_remove":
			if a.itemID != 0 {
				if err := a.cfg.RemoveFavorite(a.itemID); err != nil {
					a.msg("Error: %v", err)
				} else {
					a.msg("✗ Removed from favorites")
				}
			}
		case "export_series":
			a.exportSeries()
		case "export_movie":
			a.exportMovie()
		}
	}
}

// ---------- просмотр ----------

func (a *App) watch() {
	if !a.ensureTranslation() {
		return
	}
	if !a.isSeries {
		a.play(0, 0)
		return
	}
	if season, episode, ok := a.selectEpisode(); ok {
		a.play(season, episode)
	}
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func (a *App) selectEpisode() (season, episode int, ok bool) {
	eps, err := a.episodesFor(a.tr)
	if err != nil || len(eps) == 0 {
		a.msg("This translation has no episodes")
		return 0, 0, false
	}
	var watched map[string][]int
	if a.itemID != 0 {
		watched = a.cfg.Watched(a.itemID)
	}

	seasons := sortedKeys(eps)
	switch len(seasons) {
	case 1:
		season = seasons[0]
	default:
		items := make([]Item, len(seasons))
		for i, s := range seasons {
			total := len(eps[s])
			d := fmt.Sprintf("Season %d %d ep", s, total)
			if w := watched[fmt.Sprintf("s%d", s)]; len(w) > 0 {
				d = fmt.Sprintf("Season %d ✓%d/%d", s, len(w), total)
			}
			items[i] = Item{Display: d, Value: s}
		}
		idx, ok := a.menu.Select("Season", items)
		if !ok {
			return 0, 0, false
		}
		season = seasons[idx]
	}

	done := map[int]bool{}
	for _, e := range watched[fmt.Sprintf("s%d", season)] {
		done[e] = true
	}
	episodes := sortedKeys(eps[season])
	items := make([]Item, len(episodes))
	for i, e := range episodes {
		d := fmt.Sprintf("Episode %d", e)
		if done[e] {
			d += " ✓"
		}
		items[i] = Item{Display: d, Value: e}
	}
	idx, ok2 := a.menu.Select(fmt.Sprintf("S%d Episode", season), items)
	if !ok2 {
		return 0, 0, false
	}
	return season, episodes[idx], true
}

func (a *App) getStream(season, episode int) (*hdrezka.Stream, error) {
	if a.isSeries {
		return a.tr.GetStream(season, episode)
	}
	return a.tr.GetStream()
}

func (a *App) play(season, episode int) {
	stream, err := a.getStream(season, episode)
	if err != nil {
		a.msg("Error getting stream: %v", err)
		return
	}
	quals := stream.Qualities()
	if len(quals) == 0 {
		a.msg("No qualities available")
		return
	}
	items := make([]Item, len(quals))
	for i, q := range quals {
		d := q
		if stream.Formats[q].IsPremium {
			d += " [PREMIUM]"
		}
		items[i] = Item{Display: d, Value: q}
	}
	idx, ok := a.menu.Select("Quality", items)
	if !ok {
		return
	}
	urls := streamURLs(stream, quals[idx], a.preferHLS)
	if len(urls) == 0 {
		a.msg("No URLs available for selected quality")
		return
	}

	title := clean(a.video.Title)
	if a.isSeries {
		title += fmt.Sprintf(" S%dE%d", season, episode)
	}
	if err := a.player.Play(urls[0], title); err != nil {
		a.msg("Playback error: %v", err)
		return
	}
	if a.isSeries && a.itemID != 0 && a.cfg.FindFavorite(a.itemID) != nil {
		a.cfg.MarkWatched(a.itemID, season, episode)
	}
}

// ---------- плейлист ----------

func (a *App) allEpisodes() []epRef {
	eps, err := a.episodesFor(a.tr)
	if err != nil {
		return nil
	}
	var refs []epRef
	for _, s := range sortedKeys(eps) {
		for _, e := range sortedKeys(eps[s]) {
			refs = append(refs, epRef{s, e})
		}
	}
	return refs
}

func (a *App) chooseExportQuality(s *hdrezka.Stream) (string, bool) {
	quals := s.Qualities()
	if len(quals) == 0 {
		a.msg("No qualities available")
		return "", false
	}
	items := make([]Item, len(quals))
	for i, q := range quals {
		items[i] = Item{Display: q, Value: q}
	}
	idx, ok := a.menu.Select("Quality", items)
	if !ok {
		return "", false
	}
	return quals[idx], true
}

func (a *App) exportFileName(quality string) string {
	return fmt.Sprintf("%s_%s_%s.m3u",
		safeName(clean(a.video.Title)), safeName(quality), safeName(a.tr.Name))
}

func (a *App) exportSeries() {
	if !a.ensureTranslation() {
		return
	}
	refs := a.allEpisodes()
	if len(refs) == 0 {
		a.msg("No episodes to export")
		return
	}
	first, err := a.tr.GetStream(refs[0].season, refs[0].episode)
	if err != nil {
		a.msg("Error: %v", err)
		return
	}
	quality, ok := a.chooseExportQuality(first)
	if !ok {
		return
	}

	results := make([]string, len(refs))
	errs := make([]error, len(refs))
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done int
	)
	sem := make(chan struct{}, 3)
	for i, ref := range refs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, ref epRef) {
			defer func() { <-sem; wg.Done() }()
			st, err := a.tr.GetStream(ref.season, ref.episode)
			urls := []string(nil)
			if err == nil {
				urls = streamURLs(st, quality, a.preferHLS)
			}
			switch {
			case err != nil:
				errs[i] = err
			case len(urls) == 0:
				errs[i] = fmt.Errorf("quality %s unavailable", quality)
			default:
				results[i] = urls[0]
			}
			mu.Lock()
			done++
			fmt.Fprintf(os.Stderr, "\rexporting %d/%d", done, len(refs))
			mu.Unlock()
		}(i, ref)
	}
	wg.Wait()
	fmt.Fprintln(os.Stderr)

	title := clean(a.video.Title)
	var entries []m3uEntry
	failed := 0
	for i, ref := range refs {
		if errs[i] != nil {
			failed++
			fmt.Fprintf(os.Stderr, "S%02dE%02d: %v\n", ref.season, ref.episode, errs[i])
			continue
		}
		entries = append(entries, m3uEntry{
			title: fmt.Sprintf("%s S%02dE%02d", title, ref.season, ref.episode),
			url:   results[i],
		})
	}
	if len(entries) == 0 {
		a.msg("No episodes to export")
		return
	}

	name := a.exportFileName(quality)
	if err := os.WriteFile(name, []byte(m3uPlaylist(entries)), 0o644); err != nil {
		a.msg("Error: %v", err)
		return
	}
	if failed > 0 {
		a.msg("✓ %d episodes, %d failed (%s) → %s", len(entries), failed, quality, name)
		return
	}
	a.msg("✓ %d episodes (%s) → %s", len(entries), quality, name)
}

func (a *App) exportMovie() {
	if !a.ensureTranslation() {
		return
	}
	stream, err := a.tr.GetStream()
	if err != nil {
		a.msg("Error: %v", err)
		return
	}
	quality, ok := a.chooseExportQuality(stream)
	if !ok {
		return
	}
	urls := streamURLs(stream, quality, a.preferHLS)
	if len(urls) == 0 {
		a.msg("No URLs for %s", quality)
		return
	}

	title := clean(a.video.Title)
	name := a.exportFileName(quality)
	body := m3uPlaylist([]m3uEntry{{title: title, url: urls[0]}})
	if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
		a.msg("Error: %v", err)
		return
	}
	a.msg("✓ %s (%s) → %s", title, quality, name)
}

type m3uEntry struct {
	title string
	url   string
}

// m3uPlaylist пишет extended M3U: плеер открывает файл как очередь.
func m3uPlaylist(entries []m3uEntry) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, e := range entries {
		title := clean(strings.ReplaceAll(e.title, ",", " "))
		fmt.Fprintf(&b, "#EXTINF:-1,%s\n%s\n", title, e.url)
	}
	return b.String()
}

// ---------- точки входа ----------

// contentURL собирает адрес тайтла. Голый числовой ID на зеркале открывает
// главную (ответ 200 без карточки), поэтому он идёт как DLE-параметр newsid:
// /?newsid=1452 редиректит на страницу фильма.
func contentURL(base, ref string) string {
	base = strings.TrimRight(base, "/")
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	if n, err := strconv.Atoi(ref); err == nil && n > 0 {
		return fmt.Sprintf("%s/?newsid=%d", base, n)
	}
	return base + "/" + strings.TrimLeft(ref, "/")
}

// RunDirect открывает тайтл по URL / slug / ID.
func (a *App) RunDirect(input string) {
	url := contentURL(a.base(), input)
	if a.load(url, extractID(url)) {
		a.contentLoop()
	}
}

func (a *App) Run(openFavorites bool, initialQuery string) {
	for {
		var (
			url string
			id  int
			ok  bool
		)
		switch {
		case openFavorites:
			openFavorites = false
			url, id, ok = a.browseFavorites()
			if !ok {
				continue
			}
		case initialQuery != "":
			q := initialQuery
			initialQuery = ""
			url, id, ok = a.searchQuery(q)
			if !ok {
				continue
			}
		default:
			url, id, ok = a.searchAndSelect()
			if !ok {
				items := []Item{
					{Display: "Open favorites", Value: "favs"},
					{Display: "Exit", Value: "exit"},
				}
				idx, sel := a.menu.Select("", items)
				if !sel || idx != 0 {
					return
				}
				url, id, ok = a.browseFavorites()
				if !ok {
					continue
				}
			}
		}
		if a.load(url, id) {
			a.contentLoop()
		}
	}
}
