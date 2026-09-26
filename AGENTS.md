# Agent Guidelines

## Browser Verification and Evidence

Browser verification is one of the primary ways agents should verify their work on dv. Agents cannot use computer-use tools with native terminals; Terma's browser launcher exposes the real dv app through a browser. Use it both to check your own implementation and to give the user evidence that the changed behavior works.

For changes to rendering, layout, focus, keyboard navigation, or mouse interaction:

1. Build the browser launcher from the sibling Terma checkout, then launch dv from this repository:

   ```sh
   (cd ../terma && go build -o /tmp/terma-browser ./cmd/terma-browser)
   /tmp/terma-browser -- go run . --no-config
   ```

   Read [Terma's browser testing guide](../terma/docs/browser-testing.md) for sizing and computer-use details. Open the printed URL, including its token fragment, in the computer-use browser. The app uses the launcher's working directory as its Git repository.

2. Reproduce the affected workflow using keyboard and mouse input. Inspect screenshots and accessible terminal text after the app updates. Check the expected outcome, including relevant focus, selection, scrolling, and layout behavior. Use the default viewport size for layout verification; use fixed terminal dimensions for screenshot comparisons.

3. Capture evidence of the implemented behavior. Save screenshots under `snapshot-output/evidence/` with descriptive filenames, showing the state that demonstrates the result. For interaction changes, capture the relevant before/after states and record the actions taken.

4. Include the evidence in the final response: embed the relevant screenshots, explain what they demonstrate, and report the interactions and tests performed. State any verification blocker or remaining uncertainty explicitly. A successful launch alone does not establish that the implementation is correct.

5. Close test tabs and stop the launcher when finished.

Browser verification complements Go tests and snapshots. Run `go test ./...` for code changes and use the browser to check affected user-visible behavior. For changes with no observable UI effect, report the relevant test results as evidence.

### Test Repositories

Use a disposable Git repository for staging, unstaging, commit, or other operations that modify repository state. Browser sessions operate on real files and Git state. To run the current dv implementation against a fixture, build it here and launch it from the fixture directory:

```sh
go build -o /tmp/dv .
cd /path/to/test-repo
/tmp/terma-browser -- /tmp/dv --no-config
```

Choose fixture changes that exercise the behavior being verified. Restarting the browser session resets in-memory app state, but preserves changes made to the repository.
