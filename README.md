# dv

A beautiful, snappy, and highly interactive tool for exploring diffs without leaving your terminal.

<img width="1576" height="980" alt="image" src="https://github.com/user-attachments/assets/de6de789-e739-41fc-9952-5bf63488b391" />

## Highlights

- Unified and split diffs
- Snappy native terminal app
- Lots of pretty themes
- Keyboard centric
- Mouse support
- Command palette
- Synchronised scrolling
- Intraline diff highlighting

## Installation

Install it using Homebrew:

```bash
brew install darrenburns/homebrew/dv
```

Then run it like this:

```bash
dv
```

`dv` displays your current staged/unstaged files from `git`.

## Things you can do

Most keybinds are documented either in the footer, command palette (`ctrl+p`), or in the UI itself.

Some things that aren't as clear at the moment:

* You can click and drag the sidebar divider to resize it.
* You can click and drag the central divider when in side-by-side/split view to adjust the ratio.
  * As a shortcut you can use `ctrl+h`/`ctrl+l` to shift it left/right.
* Tab and shift-tab move focus
* When the file tree is focused, press `/` to filter the files, `tab` to move focus back to the tree, and `esc` to clear the filter.

## Startup options

There are a few CLI options available for customising `dv`.

| Flag | Values | Default |
| --- | --- | --- |
| `--view` | `unified`, `split` | `unified` |
| `--sidebar` | `true`, `false` | `true` |
| `--theme` | any built-in theme name (for example `catppuccin`, `dracula`, `nord`) | `catppuccin` |
| `--intraline-style` | `background`, `underline` | `background` |
| `--show-symbols` | `true`, `false` | `false` |

Example using all options:

```bash
dv --view split --sidebar=false --theme catppuccin --intraline-style underline --show-symbols
```

There's no config file yet, so I recommend creating an alias or something in your shell for now.

Notes:
- For string flags, both `--flag value` and `--flag=value` work.
- For booleans, prefer `--flag=false` when disabling.

