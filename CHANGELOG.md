# keyboardctl

## 1.1.0

### Minor Changes

- 36c36f0: Add automated releases: changesets drives versioning and the changelog, and CI
  builds, signs and publishes a ready-to-run Windows zip (CLI + driver + test
  certificate) to GitHub Releases. Installation no longer requires a compiler or
  the WDK - download the zip, run `install.bat`, done.
