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

**Pre-built binaries** for Linux, macOS and Windows (amd64 and arm64) are attached
to every release on the [releases page](https://github.com/kaitencloud/cli/releases),
with a `checksums.txt`.

**With Go 1.25 or newer:**

```shell
go install github.com/kaitencloud/cli/cmd/kaiten@latest
```

**From source**, with [Task](https://taskfile.dev) installed:

```shell
git clone https://github.com/kaitencloud/cli.git && cd cli
task build   # bin/kaiten
```

**Shell completion** is available for bash, zsh, fish and PowerShell:

```shell
kaiten completion zsh > "${fpath[1]}/_kaiten"
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
kaiten instances audit-trails acme-production --event-name INSTANCE_UPDATED --after 2026-01-01T00:00:00Z --limit 100
```

`--output` (or `KAITEN_OUTPUT`) selects `table`, `json` or `yaml`.

Without it, the format follows what the command returns. A list prints a table. A single
resource (`get`, `create`, a usage report, a new token) prints YAML, and so does
`--output table` there, a single resource having no columns. A command the API answers
with no body (`delete`, most `update`s) prints one line, or a document with a `message`
key under `--output json` and `--output yaml`.

JSON and YAML output carry the same document: the API's field names and values, in the
same order.

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

A file or a payload uses the API's field names, in JSON or in YAML, and both make the
same request:

```yaml
# customer.yaml
name: Acme Corp
externalCustomerId: crm-4711
```

A YAML document is read as the JSON it stands for. A value written bare for a text field
stays text (`version: 1.0` is the version `1.0`), and a YAML key the request has no field
for is an error that names the key, before anything is sent. JSON skips such a key
instead, so a resource printed with `--output json` can be edited and written back with
the fields only the API sets (`id`, `createdAt`, ...) still in it; printed as YAML, it has
to lose them first.

With inline flags, `instances update` keeps what its optional flags leave out: the
deployment zone, the slug, and the metadata, which the command reads from the instance and
sends back unless `--metadata-json` gives another (`--metadata-json '{}'` clears it).

### Licenses and entitlements

```shell
kaiten licenses entitlements associate team-v1 seats --number 25
kaiten licenses entitlements associate team-v1 sso --boolean=true
kaiten licenses entitlements associate team-v1 limits --object-payload '{"max": 10}'

# A soft limit: 1000 orders a month, refused past 1200
kaiten entitlements create --name "Monthly orders" --slug monthly-orders --type NUMBER \
  --aggregation-method SUM --reset-period MONTH --warning-threshold-percent 80
kaiten licenses entitlements associate team-v1 monthly-orders --number 1000 --overage-percent 20

# Change the value; the grant keeps its 20 % allowance
kaiten licenses entitlements update team-v1 monthly-orders --number 1500
```

The API replaces an entitlement or a grant with what it is sent, resetting what the request
leaves out. So both update commands read the resource first and send back what the
invocation does not change, each in its own way:

- `entitlements update` does it with inline flags: every field the flags do not name is
  sent back as it was read, so a periodic entitlement keeps its window, which the API
  refuses to see dropped. `--file` and `--payload` are sent as written there, and carry
  every field the entitlement keeps.
- `licenses entitlements update` does it whenever `--overage-percent` is absent, whatever
  carries the value: `--number`, `--boolean`, an object, or `--file` and `--payload`, which
  hold the value and nothing else. The grant keeps its allowance when the new value is a
  finite number. `--overage-percent` sets another, and the grant is then not read.

The API numbers the versions of a license itself. `licenses create` has no `--version`,
and on `licenses update` the flag does not set the version: it states the one the license
is expected to have, and the API refuses the update when the license has another.

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

Without `--yes` (`-y`), a run that cannot prompt changes nothing and exits with code `2`.
At the prompt, any answer but `y` or `yes` cancels the command, which exits with code `1`.

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
task vuln           # govulncheck over reachable code
task release:check  # validate .goreleaser.yaml without building
```

The API client is [`github.com/kaitencloud/sdk-go`](https://github.com/kaitencloud/sdk-go);
a change to a request or response shape belongs there. This repository holds the
command surface: flags, input sources, output formatting, confirmation prompts and
exit codes.

Releases are cut by pushing a `vX.Y.Z` tag: GoReleaser builds the six binaries,
writes the checksums and publishes the GitHub release.

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
