# Changelog


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