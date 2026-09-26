# Aside contributor guide

Aside is a read-only repository browser with local inline conversations. Repository reads belong in `git.go`; review validation, identity, and persistence belong in `review.go`. UI, CLI, and MCP share that store. Agent setup is documented in `skills/aside-review/SKILL.md`.

Preserve these invariants:

- Aside never stages, commits, pushes, checks out, or publishes GitHub comments. Its writes go to the local review store.
- Anchors name an immutable snapshot, exact path, old/new side, and source line range. Rendered rows and current working files are not anchor identities.
- Comments and composers are real Terma widgets interleaved with code sections. Replies stay inside their thread; new editors stay beneath the selected lines. Keep review text out of the source renderer and selection model.
- Draft threads stay outside agent-facing responses. Incoming updates preserve active selections and compositions. Store mutations must retain validation, atomic replacement, and concurrent writer protection.
- Reactive updates happen in handlers, setup, or layout callbacks, never in `Build()`.

## Browser verification and evidence

Browser testing is a primary means for agents to verify their own work and give the user evidence that it is correct. Agents cannot use computer-use tools with native terminals. Terma's browser launcher exposes the real Aside process through a PTY.

For rendering, layout, focus, keyboard, or mouse changes:

1. Build Aside and the launcher, then run with a disposable repository and isolated data directory:

   ```sh
   go build -o /tmp/aside .
   (cd ../terma && go build -o /tmp/terma-browser ./cmd/terma-browser)
   ASIDE_DATA_DIR=/tmp/aside-test-reviews /tmp/terma-browser -- /tmp/aside --repo /path/to/fixture --no-config
   ```

2. Read [Terma's browser testing guide](../terma/docs/browser-testing.md) for sizing and computer-use details. Open the printed URL including its token fragment. Exercise the changed workflow with keyboard and mouse, including relevant selection, scrolling, focus, persistence, and narrow layouts. Inspect the resulting accessible text and screenshots. A launch alone is not verification.
3. Save key screenshots in `snapshot-output/evidence/`. Record actions, expected/observed results, automated checks, and remaining uncertainty in a digestible report. Embed relevant screenshots in the final response so the user can assess the implementation.
4. Run the full Go suite and relevant snapshot tests. Regenerate intentional goldens with `UPDATE_SNAPSHOTS=1`; visually inspect them before accepting. Run race tests when changing shared state or persistence.
5. Close temporary tabs and stop test launchers when finished.

Use fixture changes that cover the affected behavior. Restarting a browser session clears transient UI state but preserves review data. Never test repository modifications against the user's active checkout.
