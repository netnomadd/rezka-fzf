package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Item — пункт меню: Display показывается, Value возвращается вызывающему коду.
type Item struct {
	Display string
	Value   any
}

// Menu — абстракция над fzf / gofer.
type Menu interface {
	// Select возвращает индекс выбранного пункта; false — отмена.
	Select(prompt string, items []Item) (int, bool)
	// Input спрашивает произвольную строку; false — отмена/пусто.
	Input(prompt string) (string, bool)
	Name() string
}

var stdin = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	line, _ := stdin.ReadString('\n')
	return strings.TrimSpace(line)
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func have(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func newMenu(kind, goferCmd string) (Menu, error) {
	switch kind {
	case "fzf":
		if !have("fzf") {
			return nil, fmt.Errorf("fzf not found. Please install fzf")
		}
		return fzfMenu{}, nil
	case "gofer":
		if !have("gofer") {
			return nil, fmt.Errorf("gofer not found in PATH")
		}
		return newGoferMenu(goferCmd), nil
	case "", "auto":
		// Из терминала — fzf; без терминала (хоткей в композиторе) — gofer.
		interactive := isTTY(os.Stdin) && isTTY(os.Stdout)
		order := []string{"fzf", "gofer"}
		if !interactive {
			order = []string{"gofer", "fzf"}
		}
		for _, k := range order {
			if have(k) {
				return newMenu(k, goferCmd)
			}
		}
		return nil, fmt.Errorf("neither fzf nor gofer found in PATH")
	}
	return nil, fmt.Errorf("unknown menu %q (auto|fzf|gofer)", kind)
}

// labelsOf чистит переводы строк/табы и делает дубликаты уникальными,
// чтобы выбранную строку можно было однозначно сопоставить с пунктом.
func labelsOf(items []Item) []string {
	seen := map[string]int{}
	out := make([]string, len(items))
	for i, it := range items {
		l := strings.Join(strings.Fields(it.Display), " ")
		if l == "" {
			l = "·"
		}
		seen[l]++
		if n := seen[l]; n > 1 {
			l = fmt.Sprintf("%s (%d)", l, n)
		}
		out[i] = l
	}
	return out
}

// runMenu запускает внешнюю программу, подаёт на stdin список и читает выбор из stdout.
func runMenu(cmd *exec.Cmd, items []Item) (int, bool) {
	labels := labelsOf(items)
	cmd.Stdin = strings.NewReader(strings.Join(labels, "\n") + "\n")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return -1, false // Esc / закрытие окна
	}
	sel := strings.TrimRight(strings.SplitN(string(out), "\n", 2)[0], "\r")
	for i, l := range labels {
		if l == sel {
			return i, true
		}
	}
	return -1, false
}

// ---------- fzf ----------

type fzfMenu struct{}

func (fzfMenu) Name() string { return "fzf" }

func (fzfMenu) Select(prompt string, items []Item) (int, bool) {
	if len(items) == 0 {
		return -1, false
	}
	cmd := exec.Command("fzf",
		"--height=40%",
		"--layout=reverse",
		"--border=rounded",
		"--prompt="+prompt+" > ",
		"--pointer=>",
		"--ansi",
		"--bind=esc:abort",
	)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	return runMenu(cmd, items)
}

func (fzfMenu) Input(prompt string) (string, bool) {
	q := readLine(prompt + " (Ctrl-C to exit): ")
	return q, q != ""
}

// ---------- gofer ----------

// Gofer — dmenu-совместимый лаунчер: список подаётся через stdin,
// выбранная строка печатается в stdout. Шаблон команды настраивается
// ключом gofer_cmd в config.yaml, {prompt} заменяется на заголовок.
type goferMenu struct{ tmpl []string }

func newGoferMenu(tmpl string) goferMenu {
	if strings.TrimSpace(tmpl) == "" {
		// --prompt= одним аргументом: Gofer не читает «--prompt текст».
		tmpl = "gofer --prompt={prompt}"
	}
	return goferMenu{tmpl: strings.Fields(tmpl)}
}

func (goferMenu) Name() string { return "gofer" }

// exec собирает команду. kind — «--input» для свободного ввода или «--prompt»
// для заголовка dmenu. Из шаблона выкидываются старые --prompt/--input,
// чтобы не перебить режим.
func (g goferMenu) exec(kind, prompt string) *exec.Cmd {
	bin := "gofer"
	var args []string
	if len(g.tmpl) > 0 {
		bin = g.tmpl[0]
	}
	for i := 1; i < len(g.tmpl); i++ {
		a := g.tmpl[i]
		switch {
		case a == "--prompt" || a == "--input":
			i++ // значение следующим аргументом
		case strings.HasPrefix(a, "--prompt=") || strings.HasPrefix(a, "--input="):
		default:
			args = append(args, strings.ReplaceAll(a, "{prompt}", prompt))
		}
	}
	if prompt != "" {
		args = append(args, kind+"="+prompt)
	}
	return exec.Command(bin, args...)
}

func (g goferMenu) Select(prompt string, items []Item) (int, bool) {
	if len(items) == 0 {
		return -1, false
	}
	return runMenu(g.exec("--prompt", prompt), items)
}

// Input открывает поле ввода Gofer (--input). dmenu с пустым списком
// запрос не возвращает: Enter выбирает строку из stdin, а не набранный текст.
func (g goferMenu) Input(prompt string) (string, bool) {
	if prompt == "" {
		prompt = "Search"
	}
	cmd := g.exec("--input", prompt)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	q := strings.TrimSpace(string(out))
	return q, q != ""
}
