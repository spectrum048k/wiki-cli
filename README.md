# wiki-cli

Export Azure DevOps project wiki pages as Markdown while preserving the wiki hierarchy locally.

## Build

Go 1.22 or newer is required. The application uses only the Go standard library.

```sh
go build -o wiki-cli .
```

Set an Azure DevOps token in the environment. The token must have `vso.wiki` read access.

```sh
export AZDO_TOKEN="..."
```

## Usage

```sh
./wiki-cli \
  --organization my-org \
  --project my-project \
  --wiki "Project Wiki" \
  --start-page "/Guides/Setup" \
  --output ./exported-wiki
```

The app exports the selected start page and its subpages, preserving their wiki paths in the output. `--start-page` defaults to `/` to export the full wiki. The output directory defaults to `./exported-wiki`; use `--output` to choose a different location. Existing contents of that directory are deleted before each export, so choose a directory that contains no files you want to keep.

A page such as `/Guides/Setup` is written to `exported-wiki/Guides/Setup.md`. The root page is written to `_index.md`. Images referenced by absolute URLs are downloaded alongside their page, and `.wiki-cli.json` records source paths, ordering, ETags, images, and failures.

## Releases

Push a version tag such as `v1.0.0` to run the GitHub Actions release workflow. It publishes the Go test results in the workflow's Checks tab, builds standalone executables for Apple silicon Macs, Intel Macs, and 64-bit Windows, then attaches them to a GitHub Release. A failed test run prevents the release from being built.
