# Changelog


## 0.2.16 - 2026-08-29

- Show compact, second-granularity durations throughout the web UI, using at most two units such as `2m1s` or `1h2m`.

## 0.2.15 - 2026-08-29

- Show rolled-up start time, end time, and live duration on web commit pages, with relative timestamps and exact details on hover.

## 0.2.14 - 2026-06-10

- Fix the web top navigation Repos menu so it stays inside the viewport.

## 0.2.13 - 2026-06-10

- Fix terminal dashboard socket status flicker during websocket reconnect handling.

## 0.2.12 - 2026-06-08

- Add `localci cat` for printing selected task artifacts directly to stdout.
- Add a compact CLI cheatsheet via `localci docs --cheatsheet`.
- Document that `localci install-hooks` is idempotent for the LocalCI-managed hook entry.
- Fix generated changelog output so it preserves a final newline.

## 0.2.11 - 2026-06-04

- Fix docs deployment for release builds that publish artifact metadata.
- Document Homebrew as the canonical install path.
- Fix terminal dashboard websocket reconnect loops on large run snapshots.
- Improve web and terminal dashboard run summaries so high-level artifact links avoid unnecessary output-directory scans.

## 0.2.10 - 2026-06-02

- Add `localci --version`.
- Add the MIT license.

## 0.2.9 - 2026-05-31

- Improve appearance of boosted artifacts

## 0.2.8 - 2026-05-31

- Fix GitHub CI and release checks for tasks that declare artifact metadata.

## 0.2.7 - 2026-05-31

- Add task history across the web UI, CLI, and terminal dashboard.
- Add task-declared artifact links so important outputs are surfaced directly across web, CLI, and TUI views.
- Improve web run navigation with a merged active/queued panel, repo navigation in the top bar, Menubar-based controls, canonical breadcrumb icons, and clearer task attempt handling.
- Fix web retry flows so newly queued attempts open and execute correctly.

## 0.2.6 - 2026-05-31

- Show commit subjects in run lists and commit headers so runs are easier to tell apart.

## 0.2.5 - 2026-05-31

- Fix terminal dashboard artifact tabs so tasks with many artifacts remain browsable.

## 0.2.4 - 2026-05-31

- Serve raw artifact directory URLs with trailing slashes as `index.html` when present.
- Copy the built documentation site into the LocalCI docs task output so it is available from the web UI.

## 0.2.3 - 2026-05-30

- Fix the terminal dashboard failing to load when API responses include pagination fields.

## 0.2.2 - 2026-05-29

- Add changelog to the documentation and make docs deployment manually runnable.
- Add system-aware dark mode to the web UI.

## 0.2.1 - 2026-05-29

- Remove vestigial repo root setting.

## 0.2.0 - 2026-05-29

- Store durable history and temporary artifacts in platform-standard directories.
- Make per-commit runs faster and smaller with shallow local clones.
- Let the web UI open browser-compatible artifacts directly and download binary artifacts.
- Improve command help, shell completions, and CLI reference accuracy.
- Add `localci docs` for quick terminal access to the core documentation.
- Improve first-run setup guidance and task-definition documentation.
- Visual improvements.

## 0.1.4 - 2026-05-29

- Add expandable package summaries for high-volume run lists.
- Add a reusable web top bar with the LocalCI logo and documentation link.
- Fix SPA navigation and daemon restart recovery issues that could leave repo pages loading or show stale load errors.
- Fix web route tests for the SPA shell.

## 0.1.3 - 2026-05-27

- Improve high-volume run lists with compact summaries, linked task names, and clearer package grouping.
- Fix commit pages so long task lists scroll correctly.
- Add lightweight package-level LocalCI demo tasks for exercising monorepo run summaries.

## 0.1.2 - 2026-05-27

- Fix repo pages with a single run stretching the run row to fill the viewport.

## 0.1.1 - 2026-05-24

- Deploy documentation only from release tags.
- Fix a websocket event race that could hang CI tests.
- Add GitHub repository metadata to the documentation site.

## 0.1.0 - 2026-05-24

- Initial release.
