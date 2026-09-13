<picture>
  <source media="(prefers-color-scheme: dark)" srcset=".github/banner.svg">
  <source media="(prefers-color-scheme: light)" srcset=".github/banner-light.svg">
  <img alt="actup — GitHub Actions version updates, with release tags rising from v4 to v5." src=".github/banner-light.svg">
</picture>

`actup` checks the GitHub Actions used in a repository and updates their version references.

## Installation

Download the release for your platform from [releases](https://github.com/coalaura/actup/releases), extract the executable and place it on your `PATH`.

## Usage

Run `actup` from the root of a repository:

```sh
actup
```

By default, `actup` reports available major-version updates without changing any files.

```sh
# Apply major-version updates.
actup --apply

# Report minor and patch updates as well.
actup --full

# Apply all available updates.
actup --full --apply
```

Use `actup --help` to view all command options.

## Behavior

- Workflow files are read from `.github/workflows`.
- Both `.yml` and `.yaml` files are supported.
- Versions are resolved from each action's latest GitHub release.
- The default mode updates references to major tags such as `v4`.
- `--full` uses the complete release version, such as `v4.2.1`.
- Local actions and non-semantic version references, such as branches and commit hashes, are left unchanged.
- Files are written only when `--apply` is provided.
