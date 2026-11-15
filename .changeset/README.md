# Changesets

This folder holds [changesets](https://github.com/changesets/changesets) — small
Markdown files that describe a change and the version bump it deserves.

## Adding a changeset

When your PR changes behaviour, add a changeset:

```sh
npm run changeset
```

Commit the generated `.changeset/*.md` file with your change.

## Release flow

1. PRs with changesets merge into `main`.
2. CI opens/updates a **"chore: version packages"** PR that consumes the
   changesets, bumps `package.json`, and writes `CHANGELOG.md`.
3. Merging that PR triggers the release job, which builds the driver + CLI,
   packages a zip, and publishes a GitHub Release.

See `.github/workflows/release.yml`.
