# rezka-fzf

Терминальный клиент для HDRezka на Go. Меню работает через [fzf](https://github.com/junegunn/fzf)
или [Gofer](https://codeberg.org/JakeAtLinux/Gofer), воспроизведение — через mpv, vlc, celluloid
или [post2mpv-menu](https://github.com/netnomadd/post2mpv-menu). Сайт разбирается библиотекой
[go-hdrezka](https://pkg.go.dev/github.com/n0madic/go-hdrezka).

Это переписанная на Go версия Python-скрипта rezka-fzf.

## Возможности

- поиск и просмотр фильмов и сериалов, выбор озвучки, сезона, серии и качества;
- избранное с отметками просмотренных серий (✓);
- сохранение плейлиста `.m3u` в текущий каталог (`mpv файл.m3u` или `vlc файл.m3u`);
- два бэкенда меню: fzf (в терминале) и Gofer (с хоткея, без терминала);
- плееры: mpv, vlc, celluloid, post2mpv-menu или любая своя команда;
- вход в аккаунт, прокси, cookies для зеркал за Cloudflare (Anubis проходится автоматически).

## Требования

- Go 1.25+
- `fzf` и/или `gofer` в `PATH`
- плеер (`mpv` по умолчанию); для уведомлений в режиме Gofer — `notify-send` (необязательно)

## Установка

```bash
go mod tidy
go build -o rezka-fzf .
cp rezka-fzf ~/.local/bin/
```

Пакет для Arch лежит в `~/git/pkgbuilds/rezka-fzf-git` (`makepkg -si`): бинарник в `/usr/bin` и desktop-файл со схемой `rezka:`.

## Использование

```bash
rezka-fzf                          # поиск
rezka-fzf -favorites               # сразу открыть избранное
rezka-fzf -url 434365              # открыть по ID, slug или полной ссылке
rezka-fzf 'rezka://434365'         # то же через URI-схему (так вызывает desktop-файл)
rezka-fzf -authFile ~/.rezka-auth  # логин и пароль из файла
rezka-fzf -menu gofer              # принудительно выбрать меню
rezka-fzf -player mpv -hls         # плеер и предпочтение HLS
```

| Флаг | Описание |
|---|---|
| `-authFile PATH` | файл с логином и паролем (`username=...` / `password=...` или две строки) |
| `-favorites` | открыть избранное при запуске |
| `-url URL_OR_ID` | открыть тайтл напрямую |
| `-menu auto\|fzf\|gofer` | бэкенд меню (по умолчанию `auto`) |
| `-player NAME` | `auto`, `mpv`, `vlc`, `celluloid`, `post2mpv-menu` или своя команда |
| `-player-args "..."` | дополнительные аргументы плееру |
| `-hls` | предпочитать HLS вместо MP4 |

`-menu auto`: из терминала берётся fzf, без терминала (например, хоткей Sway/Hyprland/Niri или `rezka://`) — Gofer.

### URI-схема `rezka:`

`rezka-fzf.desktop` регистрирует обработчик `x-scheme-handler/rezka`.
`Terminal=false`: ссылка из браузера открывается без терминала, меню — Gofer.

```
rezka://434365
rezka://434365-pantheon.html
rezka://films/drama/123-title.html
rezka://hdrezka.ag/123-title.html
rezka://https://hdrezka.ag/123-title.html
rezka://favorites
```

Тот же аргумент можно передать в `-url` или позиционно. Из нескольких ссылок берётся первая.

Браузер при первом открытии `rezka://` спрашивает подтверждение.
Ручная установка обработчика (пакет ставит desktop-файл сам):

```bash
install -Dm644 rezka-fzf.desktop ~/.local/share/applications/rezka-fzf.desktop
update-desktop-database ~/.local/share/applications
xdg-mime default rezka-fzf.desktop x-scheme-handler/rezka
xdg-open 'rezka://434365'
```

## Конфигурация

Файл `~/.config/rezka-fzf/config.yaml` (создаётся при первом запуске; формат совместим с Python-версией):

```yaml
url: https://hdrezka.ag
username: ""
password: ""
player: auto            # auto | mpv | vlc | celluloid | post2mpv-menu | команда
player_args: ""         # например "-a play" для post2mpv-menu
menu: auto              # auto | fzf | gofer
gofer_cmd: "gofer --prompt={prompt}" # шаблон dmenu; {prompt} — заголовок списка
proxy: ""               # http(s):// или socks5://
cookies: ""             # "cf_clearance=...;..." для зеркал за Cloudflare
```

Избранное хранится в `~/.config/rezka-fzf/favorites.yaml` (формат тоже совместим).

### Gofer

Список подаётся в Gofer через stdin (dmenu-режим), выбранная строка читается из stdout.
Заголовок списка — `--prompt={prompt}` одним аргументом: Gofer не принимает `--prompt` отдельно.
Поисковый запрос открывается через `gofer --input` (поле «Search», Enter отправляет текст).
Команду dmenu можно изменить ключом `gofer_cmd`. В режиме Gofer ошибки дублируются через `notify-send`.

### post2mpv-menu

```yaml
player: post2mpv-menu
player_args: "-i local_play"   # профиль сразу, без меню; можно оставить пустым
```

Программа ждёт завершения `post2mpv-menu`, поэтому ошибки (недоступен сервер, неверный токен)
показываются в rezka-fzf. Закрытие меню `post2mpv-menu` по Esc от успеха не отличается,
и серия всё равно помечается просмотренной.

## Отличия от Python-версии

- озвучка выбирается один раз на тайтл (меняется пунктом «Translation» в меню): `go-hdrezka`
  получает серии отдельно для каждой озвучки;
- пункта «Other Parts» нет — в библиотеке этого нет;
- по умолчанию используется MP4, HLS — только если MP4 нет или указан `-hls`;
  в плейлист попадает та же ссылка, что и в «Watch».

## Структура

| Файл | Назначение |
|---|---|
| `main.go` | флаги и запуск |
| `uri.go` | разбор `rezka://` и позиционных аргументов |
| `rezka-fzf.desktop` | обработчик схемы `rezka:` |
| `app.go` | поиск, меню тайтла, просмотр, избранное, плейлист |
| `menu.go` | бэкенды меню (fzf, Gofer) |
| `player.go` | запуск плееров и post2mpv-menu |
| `config.go` | конфиг и избранное |
