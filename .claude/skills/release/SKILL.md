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

4. Update pinned install versions everywhere they appear.

   - First find the old version in the whole repo so no install snippet is missed:

     ```sh
     rg 'github:irskep/localci|0\.1\.0|v0\.1\.0'
     ```

     Replace `0.1.0` and `v0.1.0` with the previous release version you are
     replacing.
   - Update every intentional pinned LocalCI install version, including
     `README.md` and `docs/src/getting-started.md`.
   - Use the plain version, for example `0.1.0`, not `v0.1.0`.

5. Run local validation.

   ```sh
   mise run check
   mise run //docs:build
   ```

6. Commit the release notes and install docs.

   ```sh
   git add CHANGELOG.md README.md docs/src/getting-started.md
   git commit -m "Release vX.Y.Z"
   ```

7. Create and push the tag.

   ```sh
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin main
   git push origin vX.Y.Z
   ```

8. Watch the release workflow.

   ```sh
   gh run list --repo irskep/localci --workflow Release --limit 3
   gh run watch <run_id> --repo irskep/localci --exit-status
   ```

9. Verify the GitHub release exists and contains binaries plus `checksums.txt`.

   ```sh
   gh release view vX.Y.Z --repo irskep/localci
   ```

10. Start the next changelog section after the release succeeds unless the maintainer asks not to.

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
