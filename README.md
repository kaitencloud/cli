# Kaiten CLI

[![CI](https://github.com/kaitencloud/cli/actions/workflows/ci.yml/badge.svg)](https://github.com/kaitencloud/cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/tag/kaitencloud/cli?label=release)](https://github.com/kaitencloud/cli/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/kaitencloud/cli)](https://goreportcard.com/report/github.com/kaitencloud/cli)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

`kaiten` is the command-line interface for the [Kaiten](https://kaiten.sh) API. It
manages customers, instances, licenses, entitlements, feature flags, components,
releases and service accounts from a terminal or a script, with table, JSON and YAML
output and exit codes a script can branch on.

- [Installation](#installation)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Commands](#commands)
- [Usage](#usage)
- [Exit codes](#exit-codes)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Installation

Every method below installs the same binary from the
[GitHub release](https://github.com/kaitencloud/cli/releases). Check what you got with
`kaiten version`.

### macOS

```shell
brew install --cask kaitencloud/tap/kaiten
```

Or with the install script, which verifies the archive against the release's
`checksums.txt` and installs into `/usr/local/bin` (`KAITEN_INSTALL_DIR` changes that):

```shell
curl -fsSL https://raw.githubusercontent.com/kaitencloud/cli/main/install/install.sh | sh
```

### Linux

```shell
curl -fsSL https://raw.githubusercontent.com/kaitencloud/cli/main/install/install.sh | sh
```

Or a package for your distribution (replace `amd64` with `arm64` as needed). Each installs
the binary and shell completions:

```shell
# Debian, Ubuntu
curl -fsSLO https://github.com/kaitencloud/cli/releases/latest/download/kaiten_linux_amd64.deb
sudo dpkg -i kaiten_linux_amd64.deb

# Fedora, RHEL, openSUSE
sudo rpm -i https://github.com/kaitencloud/cli/releases/latest/download/kaiten_linux_amd64.rpm

# Alpine
curl -fsSLO https://github.com/kaitencloud/cli/releases/latest/download/kaiten_linux_amd64.apk
sudo apk add --allow-untrusted kaiten_linux_amd64.apk
```

Homebrew on Linux works too: `brew install --cask kaitencloud/tap/kaiten`.

### Windows

```powershell
irm https://raw.githubusercontent.com/kaitencloud/cli/main/install/install.ps1 | iex
```

The script verifies the archive against `checksums.txt`, installs `kaiten.exe` into
`%LOCALAPPDATA%\Programs\kaiten` (no administrator rights needed) and adds it to your
user `PATH`. Or with [Scoop](https://scoop.sh):

```powershell
scoop bucket add kaitencloud https://github.com/kaitencloud/scoop-bucket
scoop install kaiten
```

### Container

```shell
docker run --rm -e KAITEN_BASE_URL -e KAITEN_AUTH_TOKEN ghcr.io/kaitencloud/cli:latest instances list
```

`ghcr.io/kaitencloud/cli` is tagged `latest` and with each version, built for
`linux/amd64` and `linux/arm64`, and runs as a non-root user. Pass the configuration
through the environment: the image keeps no config file between runs.

### Go

With Go 1.25 or newer:

```shell
go install github.com/kaitencloud/cli/cmd/kaiten@latest
```

### Binaries

Every release attaches `kaiten_<os>_<arch>.tar.gz` (`.zip` on Windows) for Linux, macOS and
Windows on amd64 and arm64, with a `checksums.txt` of SHA-256 sums. Unpack it and put
`kaiten` on your `PATH`.

### A specific version

```shell
curl -fsSL https://raw.githubusercontent.com/kaitencloud/cli/main/install/install.sh | sh -s -- 1.2.3
```

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/kaitencloud/cli/main/install/install.ps1))) -Version 1.2.3
```

Homebrew and Scoop upgrade with `brew upgrade --cask kaiten` and `scoop update kaiten`;
`go install ...@v1.2.3` and the container tags pin a version directly.

### Shell completion

Homebrew, the Linux packages and the archives ship completions for bash, zsh and fish.
Any other install can generate them from the binary:

```shell
kaiten completion zsh > "${fpath[1]}/_kaiten"
kaiten completion bash > /etc/bash_completion.d/kaiten
kaiten completion fish > ~/.config/fish/completions/kaiten.fish
kaiten completion powershell | Out-String | Invoke-Expression
```

## Quick start

```shell
kaiten config set base-url https://kaiten.example.com/api
kaiten config set auth-token "$KAITEN_TOKEN"
kaiten doctor                      # checks the configuration and reaches the API
kaiten instances list
kaiten instances get acme-production --output json
```

The base URL includes the API's path prefix, and the token is an
organization-scoped Kaiten token. See the [Kaiten documentation](https://docs.kaiten.sh)
for how to obtain one.

## Configuration

Three settings are resolved for every invocation, in this order: flag, environment
variable, config file.

| Setting      | Flag           | Environment variable | Config key   |
| ------------ | -------------- | -------------------- | ------------ |
| API base URL | `--base-url`   | `KAITEN_BASE_URL`    | `base_url`   |
| Bearer token | `--auth-token` | `KAITEN_AUTH_TOKEN`  | `auth_token` |
| Output       | `--output`     | `KAITEN_OUTPUT`      | `output`     |

The config file lives in the user's configuration directory
(`~/.config/kaiten/config.yaml` on Linux, `~/Library/Application Support/kaiten/config.yaml`
on macOS, `%AppData%\kaiten\config.yaml` on Windows) and is written with `0600`
permissions. `kaiten config` manages it:

```shell
kaiten config set output json
kaiten config view
kaiten config unset auth-token
```

`kaiten doctor` shows the base URL and (masked) token the current invocation resolves
to, then lists the instances with them to prove the API is reachable and the token
accepted.

## Commands

| Command                    | Operations                                                                                      |
| -------------------------- | ----------------------------------------------------------------------------------------------- |
| `kaiten customers`         | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten instances`         | `list`, `get`, `create`, `update`, `delete`, `audit-trails`, `usage list\|get\|report`          |
| `kaiten licenses`          | `list`, `get`, `create`, `update`, `delete`, `entitlements list\|get\|associate\|update\|delete` |
| `kaiten entitlements`      | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten entitlement-groups`| `list`, `get`, `create`, `update`, `delete`, `add-entitlement`, `remove-entitlement`, `usage`   |
| `kaiten feature-flags`     | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten components`        | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten releases`          | `list`, `get`, `create`, `delete`                                                               |
| `kaiten service-accounts`  | `list`, `get`, `create`, `update`, `tokens list\|create\|delete`                                |
| `kaiten deployment-zones`  | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten config`            | `set`, `unset`, `view`                                                                          |
| `kaiten doctor`            | Validate configuration and API connectivity                                                     |
| `kaiten version`           | Print version, commit and build date                                                            |

`kaiten <command> --help` lists every flag.

## Usage

### Reading

```shell
kaiten customers list
kaiten licenses get team-v1 --output yaml
kaiten licenses entitlements list team-v1
kaiten instances audit-trails acme-production --event-name instance.updated --after 2026-01-01T00:00:00Z --limit 100
```

`--output` (or `KAITEN_OUTPUT`) selects `table` (the default), `json` or `yaml`.

### Writing

Every `create` and `update` accepts its input three ways. They are mutually exclusive.

```shell
# Inline flags, for the fields a resource has
kaiten customers create --name "Acme Corp" --external-customer-id crm-4711

# A JSON or YAML file; "-" reads stdin
kaiten instances create --file instance.yaml
cat instance.json | kaiten instances update acme-production --file -

# An inline JSON or YAML payload
kaiten components create --payload '{"name": "api", "version": "1.4.0"}'
```

Feature flags carry targeting rules and variants, so `feature-flags create` and
`update` take `--file` or `--payload` only.

### Licenses and entitlements

```shell
kaiten licenses entitlements associate team-v1 seats --number 25
kaiten licenses entitlements associate team-v1 sso --boolean=true
kaiten licenses entitlements associate team-v1 limits --object-payload '{"max": 10}'
```

### Usage metering

```shell
# Report a delta (the default behaviour) or set the absolute value
kaiten instances usage report acme-production api-calls --value 1
kaiten instances usage report acme-production seats --value 18 --behavior set

kaiten instances usage list acme-production
kaiten entitlement-groups usage compute acme-production
```

### Service account tokens

```shell
kaiten service-accounts tokens create deploy-bot --name "zone-eu-west-1" \
  --scope write:instances --scope read:feature_flags \
  --expires-at 2026-12-31T00:00:00Z
```

The plaintext token is printed once; the API stores a hash.

### Destructive commands

`delete`, `remove-entitlement` and `config unset` echo their target and ask for
confirmation. When stdin is not an interactive terminal they refuse to run instead of
prompting, so scripts and CI pipelines pass `--yes`:

```shell
kaiten customers delete acme --yes
```

## Exit codes

Every command reports the kind of failure through its exit code, so a script can
branch on it without parsing the error text.

| Code  | Meaning                                                                                                                                              |
| ----- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `0`   | The command succeeded.                                                                                                                               |
| `1`   | The command failed for a reason the CLI could not classify.                                                                                          |
| `2`   | Wrong invocation: an unknown command or flag, the wrong number of arguments, a nonsensical flag combination, or a configuration the CLI cannot use. |
| `3`   | The API refused the credentials or the operation (HTTP 401, 403).                                                                                    |
| `4`   | The addressed resource does not exist (HTTP 404).                                                                                                    |
| `5`   | The API understood the request and refused it: a validation failure, a conflict, a rate limit (any other HTTP 4xx). Retrying unchanged will not help. |
| `6`   | The operation did not happen for reasons outside your control: the API could not be reached, timed out, or answered HTTP 5xx. Retrying may help.     |
| `130` | The run was cancelled by `SIGINT` or `SIGTERM`.                                                                                                      |

```shell
kaiten customers get acme || case $? in
  4) echo "no such customer" ;;
  6) echo "kaiten is unreachable, retrying later" ;;
esac
```

## Development

Install [Task](https://taskfile.dev) and Go 1.25+. Tools run through
`go run <tool>@<version>` against the versions pinned in `Taskfile.yml`, so local runs
match CI.

```shell
task build          # bin/kaiten
task test           # go test -race -shuffle=on -cover ./...
task lint           # golangci-lint, the version CI runs
task fmt            # gofumpt + gci
task vuln              # govulncheck over reachable code
task release:check     # validate .goreleaser.yaml and the install scripts
task release:snapshot  # build every release artifact into dist/, publish nothing
```

The API client is [`github.com/kaitencloud/sdk-go`](https://github.com/kaitencloud/sdk-go);
a change to a request or response shape belongs there. This repository holds the
command surface: flags, input sources, output formatting, confirmation prompts and
exit codes.

### Releasing

Pushing a `vX.Y.Z` tag runs `.github/workflows/release.yml`, and GoReleaser
(`.goreleaser.yaml`) produces everything the Installation section points at: the six
binaries and their archives with completions, `checksums.txt`, the `.deb`/`.rpm`/`.apk`
packages, the `ghcr.io/kaitencloud/cli` image, the Homebrew cask and the Scoop manifest.
`task release:snapshot` builds all of it locally into `dist/` without a tag, and
`task release:check` validates the configuration and the install scripts; CI runs the
latter on every pull request.

The cask and the Scoop manifest are pushed to `kaitencloud/homebrew-tap` and
`kaitencloud/scoop-bucket` with the `HOMEBREW_TAP_GITHUB_TOKEN` repository secret, a
token that can write to both. Without it the release still succeeds and the two files are
left in `dist/` for a manual commit.

A pre-release tag such as `v1.2.0-rc.1` is published as a GitHub pre-release: the
install scripts' "latest", the `latest` image tag, the cask and the Scoop manifest all
keep pointing at the last stable version.

## Contributing

Issues and pull requests are welcome. Before opening one, run `task fmt`, `task lint`
and `task test`. A new command or flag should come with a test in `internal/cmd`, and
a change to what a command prints should keep `--output json` and `--output yaml`
stable, since scripts depend on them.

## License

The Kaiten CLI is open source and licensed under the
[Apache License, Version 2.0](./LICENSE).

By contributing, you agree to certify your contribution under the
[Developer Certificate of Origin 1.1](./DCO.md).

The Kaiten name and logos are not licensed under Apache-2.0. See the
[Kaiten trademark policy](https://github.com/kaitencloud/kaiten/blob/main/TRADEMARKS.md).
