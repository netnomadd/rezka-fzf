package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	authFile := flag.String("authFile", "", "path to a file with username and password (plain lines or key=value)")
	favorites := flag.Bool("favorites", false, "open favorites list on startup")
	direct := flag.String("url", "", "open by URL, slug, ID, or rezka:// URI (e.g. 'https://hdrezka.ag/434365-pantheon.html', '434365', 'rezka://favorites')")
	search := flag.String("search", "", "open the result list for this query")
	menuKind := flag.String("menu", "", "menu backend: auto | fzf | gofer (overrides config)")
	playerName := flag.String("player", "", "player: auto | mpv | vlc | celluloid | post2mpv-menu | command (overrides config)")
	playerArgs := flag.String("player-args", "", "extra player arguments, e.g. \"-a play\" for post2mpv-menu")
	hls := flag.Bool("hls", false, "prefer HLS stream over MP4")
	playlistDir := flag.String("playlist-dir", "", "directory for saved .m3u playlists (default ~/tmp/rezka-fzf)")
	flag.Parse()

	directURL, openFavorites, query := launchTarget(*direct, *favorites, flag.Args())
	if query == "" {
		query = *search
	}

	cfg, err := LoadConfig(*authFile)
	if err != nil {
		die(err)
	}

	if *playerName != "" {
		cfg.Player = *playerName
	}
	if *playerArgs != "" {
		cfg.PlayerArgs = *playerArgs
	}
	if *playlistDir != "" {
		cfg.PlaylistDir = *playlistDir
	}

	kind := cfg.Menu
	if *menuKind != "" {
		kind = *menuKind
	}
	m, err := newMenu(kind, cfg.GoferCmd)
	if err != nil {
		die(err)
	}

	app := NewApp(cfg, m, *hls)
	if err := app.setup(); err != nil {
		app.msg("%v", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\nGoodbye!")
		os.Exit(0)
	}()

	if directURL != "" {
		app.RunDirect(directURL)
		return
	}
	app.Run(openFavorites, query)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
