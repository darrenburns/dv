# Aside

A terminal file and diff browser with inline conversations for humans and coding agents. Based on dv's `dvdev` branch, with staging, committing, pushing, and PR creation removed.

Aside reads your repository. Feedback, captured diffs, and agent findings live in a separate local review store. Nothing is posted to GitHub.

## Build and run

This development branch uses the sibling Terma checkout through `replace github.com/darrenburns/terma => ../terma`. Use Go 1.25.5 or newer with this layout:

```text
code/
  aside/     # this branch
  terma/     # ad4d7cd2de162b98a133bd0e727af279ae9ba254 or compatible
```

```sh
go build -o aside .
./aside --repo /path/to/project
./aside --repo /path/to/project --base main --head feature
./aside --repo /path/to/project --pr 123
./aside --review REVIEW_ID
```

PRs require an authenticated `gh`. Aside captures the diff without checking out a branch; it rejects a PR that changes while being read. Branch comparisons use the merge base. Working changes combine staged, unstaged, and untracked changes relative to HEAD.

The sidebar defaults to changed files; `a` includes all tracked and unignored working-tree files. Unchanged files show “No text changes”. PR and branch reviews list their changed files. `/` filters paths; Enter keeps the filter and Escape clears it. `n`/`p` visit visible files. `v` switches unified/split views. Drag the sidebar divider to resize it. `r` captures current working changes; saved branch/PR views stay pinned. Reopen the same source to capture a newer branch/PR snapshot.

## Browsing controls

`Ctrl+P` opens the searchable command palette. `t` opens the theme picker with live previews; Escape cancels the preview and Enter accepts it. The palette also exposes the viewing controls and review actions.

| Key | Action |
| --- | --- |
| `b` / `d` | Toggle the sidebar / resize its divider with the keyboard |
| `w` | Toggle line wrapping; left/right scroll horizontally when the diff has focus |
| `v` / `Ctrl+H` / `Ctrl+L` | Toggle unified/split view / shift the split left or right |
| `i` | Cycle intraline highlighting: background, underline, off |
| `y` | Copy selected code, or the active file path when no text is selected |
| `m` / `M` | Mark a file seen / clear seen marks |
| `n` / `p` or `Ctrl+J` / `Ctrl+K` | Next / previous file |
| `J` / `K` | Scroll ten lines down / up |

The sidebar shows coloured change totals and seen-file progress. Seen marks and per-file scroll positions last for the current session. A file's seen mark stops applying when its captured content changes. Scroll positions are remembered separately for unified/split and wrapped/unwrapped views. File filtering is fuzzy and highlights matching filename characters; Escape clears a retained filter.

See the [dvdev comparison and polish verification](docs/polish-verification.md) for what was restored and what still differs.

## Inline feedback

1. Click or drag across code, then press `c`. The editor opens immediately below the selected lines. In split view, select the old or new side. A selection spanning a unified replacement is rejected as ambiguous.
2. Save with Ctrl+S or **Save draft**. Escape or **Cancel** closes the editor. Drafts persist locally and remain hidden from the agent interfaces.
3. Press `f` or **Submit feedback** to submit drafts and copy the open conversations with their exact code references and quoted lines. Paste into your agent, or ask it to read Aside feedback.
4. Use **Reply**, or `[`/`]` then `e`, to reply inside a thread. Replies to submitted threads are immediately available to the agent. **Resolve** / `x` resolves or reopens a thread.

Native clipboard helpers are used where available. Remote terminals fall back to OSC 52 and report delivery as unconfirmed. You can always export Markdown:

```sh
./aside review feedback --repo /path/to/project --id REVIEW_ID --format markdown
```

Comments are attached to a captured snapshot, a repository-relative path, a side, and inclusive line numbers. Refreshing changed code marks earlier comments **OUTDATED** and retains their original code quote. Aside does not silently move comments to different lines. Binary files have no line comments. Select ranges within one uninterrupted code section; existing comment cards divide sections.

## Claude Code and other agents

The clipboard workflow needs no MCP setup. For a shared local review store, register Aside as a stdio MCP server from your target project:

```sh
claude mcp add --transport stdio aside -- /absolute/path/to/aside mcp --repo /absolute/path/to/project
```

Copy `skills/aside-review` into the target project's `.claude/skills/aside-review` (or your agent's skill directory). Then ask: “Use aside-review to review PR 123 and publish findings inline.” For feedback: “Read and address the submitted feedback in Aside review REVIEW_ID.” The skill also works through the CLI when MCP is unavailable.

This setup follows Claude Code's [local MCP server documentation](https://code.claude.com/docs/en/mcp#option-3-add-a-local-stdio-server) and [skills documentation](https://code.claude.com/docs/en/skills). The package does not change your global Claude configuration.

Tools: `aside_open`, `aside_list`, `aside_read`, `aside_publish`, `aside_feedback`, `aside_reply`, `aside_status`. The server is bound to one repository. Publishing validates the whole batch and uses stable annotation keys to make retries idempotent. Agents can mark threads **addressed**; MCP reserves **resolved** for the user. The UI notices stored updates automatically, deferring them while you compose or select code. Press Escape to clear a selection and receive pending updates.

This is a pull workflow: ordinary MCP tools do not wake an idle agent. Submit feedback, then prompt the agent to read it. Ordered walkthrough chapters and automatic agent wake-up are future features; inline explanations are supported now.

See [the agent skill](skills/aside-review/SKILL.md) for command examples and the publication schema.

## Storage and configuration

Reviews use the XDG data directory under `aside/reviews` (on macOS, usually `~/Library/Application Support/aside/reviews`). `ASIDE_DATA_DIR` overrides this location. The UI, CLI, and MCP process must use the same data directory. Review JSON is written atomically under an advisory lock; invalid batches and corrupt files are not overwritten. Draft privacy applies to the agent interfaces, not to another program with access to your account's files.

Configuration lives in the XDG config directory under `aside/config.yaml`. Supported keys are `view`, `theme`, `intraline-style`, `sidebar`, `show-symbols`, and `wrap`. The corresponding flags override these values. Use `--no-config` for repeatable tests. Run `./aside --help` for options.

## Verification

Run `go test ./...`, `go test -race ./...`, and `go vet ./...`. Visual goldens are under `testdata/`; intentional changes use `UPDATE_SNAPSHOTS=1 go test ./...` and require visual inspection.

Browser verification runs the real native app through a PTY:

```sh
(cd ../terma && go build -o /tmp/terma-browser ./cmd/terma-browser)
go build -o /tmp/aside .
ASIDE_DATA_DIR=/tmp/aside-test-reviews /tmp/terma-browser -- /tmp/aside --repo /path/to/disposable-fixture --no-config
```

Read [the verification report](docs/verification.md) for recorded results, screenshots, edge cases, and limits.
