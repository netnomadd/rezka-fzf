package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const post2mpvBin = "post2mpv-menu"

// Шаблоны плееров. Дополнительные аргументы (player_args) вставляются
// перед последним элементом — {url}.
var playerTemplates = map[string][]string{
	"mpv":         {"mpv", "--title={title}", "{url}"},
	"vlc":         {"vlc", "{url}"},
	"celluloid":   {"celluloid", "{url}"},
	post2mpvBin:   {post2mpvBin, "{url}"},
}

type Player struct {
	name  string
	extra []string
}

// NewPlayer: name — "auto" | mpv | vlc | celluloid | post2mpv-menu | любая команда.
// extraArgs — дополнительные аргументы плееру (например, "-a play" для post2mpv-menu).
func NewPlayer(name, extraArgs string) *Player {
	switch name {
	case "", "auto":
		name = "mpv"
		for _, p := range []string{"mpv", "vlc", "celluloid"} {
			if have(p) {
				name = p
				break
			}
		}
	case "post2mpv":
		name = post2mpvBin
	}
	return &Player{name: name, extra: strings.Fields(extraArgs)}
}

func (p *Player) args(url, title string) []string {
	tmpl, ok := playerTemplates[p.name]
	if !ok {
		tmpl = []string{p.name, "{url}"} // произвольный плеер из конфига
	}
	r := strings.NewReplacer("{title}", title, "{url}", url)
	last := len(tmpl) - 1
	var out []string
	for _, a := range tmpl[:last] {
		out = append(out, r.Replace(a))
	}
	out = append(out, p.extra...)
	return append(out, r.Replace(tmpl[last]))
}

func (p *Player) Play(url, title string) error {
	args := p.args(url, title)
	if p.name == post2mpvBin {
		return runPost2mpv(args)
	}

	// Обычный плеер: отдельный процесс в своей сессии, меню не блокируется.
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0); err == nil {
		defer devnull.Close()
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // не оставляем зомби, пока меню работает
	return nil
}

// runPost2mpv запускает post2mpv-menu и ждёт завершения: скрипт сам показывает
// меню действий (Gofer) и отправляет запрос на post2mpv-server, а ошибки
// (недоступен сервер, неверный токен) выводит в stderr — их стоит показать.
func runPost2mpv(args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s: %w: %s", post2mpvBin, err, msg)
		}
		return fmt.Errorf("%s: %w", post2mpvBin, err)
	}
	return nil
}
