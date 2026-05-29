# Changelog

<!-- loosely based on https://keepachangelog.com/en/1.0.0/ -->

## 0.1.5 - Unreleased

- Use platform-standard config and cache directories for LocalCI state and artifacts.
- Use shallow local fetches for per-commit clones.
- Serve task artifacts through the web UI with browser-compatible raw URLs and downloads.
- Use Cobra for CLI parsing, help, and shell completions.
- Generate the CLI reference from Cobra command metadata.
- Add `localci docs` with bundled narrative documentation.
- Add first-run setup guidance to the web home page.
- Add a Defining Tasks docs guide for TOML and file-based Mise tasks.
- Remove legacy `run.json` history import and fallback code now that run history is SQLite-backed.
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
