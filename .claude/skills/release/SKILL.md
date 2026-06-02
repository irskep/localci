# Release LocalCI

Use this skill when preparing and publishing a LocalCI release.

LocalCI releases are driven by Git tags. A push of `vX.Y.Z` starts `.github/workflows/release.yml`, which validates release-relevant mise `localci:*` tasks, builds Go binaries, extracts notes from `CHANGELOG.md`, and creates the GitHub release object.

## Release Checklist

1. Verify the workspace is clean and `main` is up to date.

   ```sh
   git status --short --branch
   git pull --ff-only origin main
   ```

2. Choose the release version from the top `CHANGELOG.md` entry. It must be plain semver without a leading `v`, for example `0.1.0`.

3. Update `CHANGELOG.md`.

   - Change `## X.Y.Z - Unreleased` to `## X.Y.Z - YYYY-MM-DD`.
   - Keep only sections that have entries.
   - Release notes should be useful to users, not a raw commit log.

4. Update the CLI version constant.

   - Set `internal/cli/version.go` to the plain release version, for example
     `0.1.0`.
   - `localci --version` must print the same version that will be tagged.

5. Check install references.

   - Homebrew via `irskep/tap` is the canonical install path, so docs should
     not need release-by-release install version updates.
   - If any legacy pinned LocalCI install snippets remain, update them or
     intentionally remove them.

6. Run local validation.

   ```sh
   mise run check
   mise run //docs:build
   ```

7. Commit the release notes, CLI version, and any install doc changes.

   ```sh
   git add CHANGELOG.md README.md docs/src/getting-started.md internal/cli/version.go
   git commit -m "Release vX.Y.Z"
   ```

8. Create and push the tag.

   ```sh
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin main
   git push origin vX.Y.Z
   ```

9. Watch the release workflow.

   ```sh
   gh run list --repo irskep/localci --workflow Release --limit 3
   gh run watch <run_id> --repo irskep/localci --exit-status
   ```

10. Verify the GitHub release exists and contains binaries plus `checksums.txt`.

   ```sh
   gh release view vX.Y.Z --repo irskep/localci
   ```

11. Start the next changelog section after the release succeeds unless the maintainer asks not to.

   ```md
   ## X.Y.(Z+1) - Unreleased

   ### Added

   ### Changed

   ### Fixed

   ### Removed
   ```

   Commit and push that changelog preparation separately.

## Notes

- Do not tag a changelog section that is still marked `Unreleased`; the release workflow intentionally fails in that case.
- Do not use `localci invoke` as the release validation path. Use the mise tasks directly so local and GitHub validation match.
- The release workflow only accepts tags matching `vX.Y.Z`, such as `v0.1.0`.
