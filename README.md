# dv

A beautiful, snappy, and highly interactive tool for exploring diffs without leaving your terminal.

<img width="1746" height="992" alt="image" src="https://github.com/user-attachments/assets/32629381-3c30-4353-a9fe-f368d4330761" />

Run `dv` with no arguments to see changes currently tracked by git, or pipe a diff directly into it.

> [!NOTE] 
> `dv` is currently in beta, but I'd love your early feedback! Feel free to open a Discussion to chat about ideas or an Issue to raise a bug.

## Quick demo

https://github.com/user-attachments/assets/e9beadc9-0e77-47e2-86aa-bb869c657863

## Highlights

- Unified and split diffs
- Snappy native terminal app
- Lots of pretty themes
- Keyboard centric
- Mouse support
- Command palette
- Synchronised scrolling
- Intraline diff highlighting
- Supports piped input

## Installation

Install it using Homebrew:

```bash
brew install darrenburns/homebrew/dv
```

Alternatively install using Go (ensure `$GOPATH/bin` or `$HOME/go/bin` is in your `PATH`):

```bash
go install github.com/darrenburns/dv@latest
```

Then run it like this:

```bash
dv
```

`dv` displays your current staged/unstaged files from `git`.

## Piped Input

You can also pipe a diff directly into `dv`:

```bash
git diff | dv
gh pr diff <number> | dv
```

## Things you can do

Most keybinds are documented either in the footer, command palette (`ctrl+p`), or in the UI itself.

Some things that aren't as clear at the moment:

* You can click and drag the sidebar divider to resize it.
* You can click and drag the central divider when in side-by-side/split view to adjust the ratio.
  * As a shortcut you can use `ctrl+h`/`ctrl+l` to shift it left/right.
* Tab and shift-tab move focus
* Press `/` from the file tree or diff view to filter files (if the sidebar is hidden, this opens it). While the filter input is focused, `up`/`down` move through matching files, `tab` moves focus back to the tree, and `esc` clears the filter.
* Press `y` to copy the active file or directory path to your clipboard (also available via the command palette).
* Press `m` to toggle seen on the active file, or `M` to clear all seen marks (both are also available in the command palette).
* Press `ctrl+j`/`ctrl+k` to move to the next/previous file (same as `n`/`p`).

## Startup options

There are a few CLI options available for customising `dv`.

| Flag | Values | Default |
| --- | --- | --- |
| `--view` | `unified`, `split` | `unified` |
| `--sidebar` | `true`, `false` | `true` |
| `--theme` | any built-in theme name (for example `catppuccin`, `dracula`, `nord`) | `obsidian-tide` |
| `--intraline-style` | `background`, `underline`, `off` | `background` |
| `--show-symbols` | `true`, `false` | `false` |
| `--ignore-whitespace` | `true`, `false` | `false` |
| `--config` | path to a YAML config file | auto-discover via XDG |
| `--no-config` | `true`, `false` | `false` |

Example using all options:

```bash
dv --view split --sidebar=false --theme catppuccin --intraline-style underline --show-symbols
```

## Config file

`dv` can load startup defaults from a YAML config file.

Default path:

```text
$XDG_CONFIG_HOME/dv/config.yaml
```

This path is resolved using `xdg.ConfigHome` via the `github.com/adrg/xdg` package.

Precedence:

1. CLI flags
2. Config file
3. Built-in defaults

You can also use:

- `--config /path/to/config.yaml` to load an explicit file path.
- `--no-config` to disable config loading for a run.

Example config file:

```yaml
view: split
sidebar: true
theme: catppuccin
intraline-style: underline
show-symbols: false
ignore-whitespace: true
```

Notes:
- For string flags, both `--flag value` and `--flag=value` work.
- For booleans, prefer `--flag=false` when disabling.
- `ignore-whitespace` is unavailable in piped mode (`git diff | dv`); apply whitespace flags before piping.
