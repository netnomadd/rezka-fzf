package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Settings — содержимое ~/.config/rezka-fzf/config.yaml
// (формат совместим с Python-версией; новые ключи необязательны).
type Settings struct {
	URL         string `yaml:"url"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	Player      string `yaml:"player"`                // auto | mpv | vlc | celluloid | post2mpv-menu | команда
	PlayerArgs  string `yaml:"player_args,omitempty"` // доп. аргументы плееру, напр. "-a play" для post2mpv-menu
	Menu        string `yaml:"menu,omitempty"`        // auto | fzf | gofer
	GoferCmd    string `yaml:"gofer_cmd,omitempty"`   // шаблон запуска gofer, {prompt} подставляется
	Proxy       string `yaml:"proxy,omitempty"`
	Cookies     string `yaml:"cookies,omitempty"`      // "name=value;name=value" (Cloudflare и т.п.)
	PlaylistDir string `yaml:"playlist_dir,omitempty"` // пусто = ~/tmp/rezka-fzf
}

// Favorite — запись в favorites.yaml (формат совместим с Python-версией).
type Favorite struct {
	ID       int              `yaml:"id"`
	Type     string           `yaml:"type,omitempty"`
	URL      string           `yaml:"url,omitempty"`
	URLShort string           `yaml:"url-short,omitempty"`
	Watched  map[string][]int `yaml:"watched,omitempty"`
}

type Config struct {
	Settings
	dir     string
	path    string
	favPath string
}

func LoadConfig(authFile string) (*Config, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	c := &Config{dir: filepath.Join(base, "rezka-fzf")}
	c.path = filepath.Join(c.dir, "config.yaml")
	c.favPath = filepath.Join(c.dir, "favorites.yaml")

	if _, err := os.Stat(c.path); os.IsNotExist(err) {
		c.Settings = Settings{URL: "https://hdrezka.ag", Player: "auto", Menu: "auto"}
		if isTTY(os.Stdin) {
			if err := c.firstRun(); err != nil {
				return nil, err
			}
		}
	} else {
		data, err := os.ReadFile(c.path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, &c.Settings); err != nil {
			return nil, fmt.Errorf("config.yaml: %w", err)
		}
	}
	if c.URL == "" {
		c.URL = "https://hdrezka.ag"
	}
	if authFile != "" {
		if err := c.loadAuthFile(authFile); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Config) playlistDir() string {
	return resolvePlaylistDir(c.PlaylistDir, "rezka-fzf")
}

func (c *Config) firstRun() error {
	fmt.Println("First run setup")
	fmt.Println()
	if v := readLine("Enter HDRezka URL [https://hdrezka.ag]: "); v != "" {
		c.URL = v
	}
	c.Username = readLine("Username (optional): ")
	c.Password = readLine("Password (optional): ")

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c.Settings)
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.path, data, 0o600); err != nil { // в файле пароль
		return err
	}
	fmt.Printf("\nConfiguration saved to: %s\n\n", c.path)
	return nil
}

func (c *Config) loadAuthFile(path string) error {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("auth file not found: %s", path)
	}
	var user, pass string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			v = strings.TrimSpace(v)
			switch strings.ToLower(strings.TrimSpace(k)) {
			case "username", "login", "email", "user":
				user = v
			case "password", "pass", "pwd":
				pass = v
			}
		} else if user == "" {
			user = line
		} else if pass == "" {
			pass = line
		}
	}
	if user == "" || pass == "" {
		return fmt.Errorf("auth file must contain username and password.\nExpected format:\n  username=...\n  password=...\nor two plain lines")
	}
	c.Username, c.Password = user, pass
	return nil
}

// ---------- favorites ----------

func (c *Config) Favorites() []Favorite {
	data, err := os.ReadFile(c.favPath)
	if err != nil {
		return nil
	}
	var favs []Favorite
	if err := yaml.Unmarshal(data, &favs); err != nil {
		return nil
	}
	return favs
}

func (c *Config) saveFavorites(favs []Favorite) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(favs)
	if err != nil {
		return err
	}
	return os.WriteFile(c.favPath, data, 0o644)
}

func (c *Config) FindFavorite(id int) *Favorite {
	for _, f := range c.Favorites() {
		if f.ID == id {
			f := f
			return &f
		}
	}
	return nil
}

func (c *Config) AddFavorite(id int, typ, slug, mirror string) error {
	favs := c.Favorites()
	for _, f := range favs {
		if f.ID == id {
			return nil
		}
	}
	fav := Favorite{ID: id, Type: typ, URLShort: slug}
	if slug != "" {
		fav.URL = mirror + "/" + slug
	}
	return c.saveFavorites(append(favs, fav))
}

func (c *Config) RemoveFavorite(id int) error {
	var out []Favorite
	for _, f := range c.Favorites() {
		if f.ID != id {
			out = append(out, f)
		}
	}
	return c.saveFavorites(out)
}

func (c *Config) MarkWatched(id, season, episode int) {
	favs := c.Favorites()
	for i := range favs {
		if favs[i].ID != id {
			continue
		}
		if favs[i].Watched == nil {
			favs[i].Watched = map[string][]int{}
		}
		key := fmt.Sprintf("s%d", season)
		eps := favs[i].Watched[key]
		for _, e := range eps {
			if e == episode {
				return
			}
		}
		eps = append(eps, episode)
		sort.Ints(eps)
		favs[i].Watched[key] = eps
		_ = c.saveFavorites(favs)
		return
	}
}

func (c *Config) Watched(id int) map[string][]int {
	if f := c.FindFavorite(id); f != nil {
		return f.Watched
	}
	return nil
}

// UpdateFavoritesMirror переписывает URL избранного при смене зеркала.
func (c *Config) UpdateFavoritesMirror(mirror string) {
	favs := c.Favorites()
	changed := false
	for i := range favs {
		if favs[i].URLShort == "" {
			continue
		}
		if u := mirror + "/" + favs[i].URLShort; favs[i].URL != u {
			favs[i].URL = u
			changed = true
		}
	}
	if changed {
		_ = c.saveFavorites(favs)
	}
}

func (c *Config) Mirror() string {
	return strings.TrimRight(c.URL, "/")
}
