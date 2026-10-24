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
| `kaiten instances`         | `list`, `get`, `create`, `update`, `delete`, `audit-trails`, `usage list\|get\|report\|history\|export` |
| `kaiten licenses`          | `list`, `get`, `create`, `update`, `delete`, `entitlements list\|get\|associate\|update\|delete` |
| `kaiten entitlements`      | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten entitlement-groups`| `list`, `get`, `create`, `update`, `delete`, `add-entitlement`, `remove-entitlement`, `usage`   |
| `kaiten feature-flags`     | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten components`        | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten releases`          | `list`, `get`, `create`, `delete`                                                               |
| `kaiten service-accounts`  | `list`, `get`, `create`, `update`, `tokens list\|create\|delete`                                |
| `kaiten deployment-zones`  | `list`, `get`, `create`, `update`, `delete`                                                     |
| `kaiten billing`           | `settings`, `capabilities`, `health`, `sync`, `close-periods`, `handoff`, `invoices`, `subscriptions`, `prices`, `addons`, `instance-addons`, `vouchers`, `publishable-keys`, `customer-sessions`, `customers` |
| `kaiten ai`                | `models list\|get\|create\|update\|delete`, `conversion-rules list\|get\|create\|update\|archive`, `report`, `inferences list`, `reservations list` |
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

`--output` (or `KAITEN_OUTPUT`) selects `table` (the default), `json`, `yaml` or
`ndjson` (one compact JSON document per line: one per row of a list).

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

A report without a key is sent once and never retried: if it fails in flight, it may
or may not have been counted. Give it a `--transaction-id` and Kaiten applies it at
most once per key, so the CLI retries it on network errors and on 500, 502, 503 and
504, and running the same command again is safe. A report the server had already
counted is answered from the first time, with a note on stderr; stdout is unchanged.

```shell
kaiten instances usage report acme-production tokens --value 1200 \
  --transaction-id llm-call:9f2c:tokens
```

A key already used for a different report exits `5`: send a correction as a new
report under a new key.

The usage history lists every accepted report, with the counter before and after it
and the limit it was gated on. The range defaults to the last 30 days.

```shell
kaiten instances usage history acme-production tokens --from 2026-10-01T00:00:00Z

# Stream it as CSV (default) or NDJSON: one pair, up to 366 days per run...
kaiten instances usage export acme-production tokens > tokens.csv
# ...or the whole organization, up to 31 days per run, narrowed if needed
kaiten instances usage export --from 2026-09-01T00:00:00Z --to 2026-10-01T00:00:00Z \
  --format json --output-file usage-september.ndjson
```

`--instance-id` and `--entitlement-id` reach deleted instances and entitlements,
whose reports are kept. A `--from` before the start of the organization's history
exits `5`. Exports are written as they arrive and are not bound by the CLI's request
timeout; `--output-file` only takes its final name once the export is complete.

### Billing

```shell
# Invoices: filter, export, settle
kaiten billing invoices list --overdue --customer acme
kaiten billing invoices list --updated-since 2027-01-01T00:00:00Z --output json
kaiten billing invoices export --from 2027-01-01T00:00:00Z --to 2027-02-01T00:00:00Z \
  --granularity invoice --output-file january.csv
kaiten billing invoices mark-paid inv_123 --external-reference WIRE-9
kaiten billing invoices void inv_123 --reason "duplicate" --yes

# The accounting handoff: lease a batch, book it, acknowledge each invoice
kaiten billing handoff claim --limit 50 --lease 30m --output ndjson > batch.ndjson
kaiten billing handoff ack inv_123 --lease-id lease_456 --external-reference ACC-2027-0001

# Subscriptions, prices, add-ons and vouchers take their body from --file or --payload
kaiten billing subscriptions subscribe acme-prod --payload '{"basePriceId":"price_1"}'
kaiten billing prices preview team-v1 --file scenario.yaml
kaiten billing instance-addons set-quantity acme-prod extra-seats --quantity 5
```

Amounts are printed in the currency's minor units, as the API carries them. An
acknowledgement with `--lease-id` for an invoice another claim took since exits `5`.

### AI credit control plane and accounting

The AI commands manage the model catalogue and credit conversion rules, and
record or inspect usage accounting. They do not configure provider credentials or
execute inference. Conversion rules belong to a **specific license version slug**
(for example `team-v2`), not its license family.

```shell
kaiten ai models list --vendor acme --modality TEXT --output json
kaiten ai models get assistant-v1
kaiten ai models create --payload \
  '{"slug":"assistant-v1","name":"Assistant","vendor":"acme","modality":"TEXT","status":"ACTIVE","vendorCosts":{"currency":"USD","perInputToken":"0.000000000001"}}'
kaiten ai models update assistant-v1 --payload '{"status":"DEPRECATED"}'
kaiten ai models delete assistant-v1 --yes
kaiten ai conversion-rules list team-v2 --entitlement ai-credits
kaiten ai conversion-rules create team-v2 --payload \
  '{"entitlementSlug":"ai-credits","modelSlug":"assistant-v1","rates":{"perInputToken":"0.000001","perOutputToken":"0.000002"}}'
kaiten ai conversion-rules update team-v2 rule-123 --file rule.yaml
kaiten ai conversion-rules archive team-v2 rule-123 --yes

# Reserve estimated quantities, then commit actual quantities under the SAME identity.
# These only record accounting; invoke your provider separately.
kaiten ai report acme-prod ai-credits --payload \
  '{"value":{"type":"ai_usage","phase":"reserve","transactionId":"call-123","model":"assistant-v1","requestType":"COMPLETION","usage":{"inputTokens":100,"outputTokens":50}}}'
kaiten ai report acme-prod ai-credits --payload \
  '{"value":{"type":"ai_usage","phase":"commit","transactionId":"call-123","model":"assistant-v1","requestType":"COMPLETION","requestStatus":"SUCCESS","usage":{"inputTokens":120,"outputTokens":60}}}'
kaiten ai inferences list acme-prod ai-credits --transaction-id call-123 --output json
kaiten ai reservations list acme-prod ai-credits
```

Writes take exactly one `--file` (JSON/YAML, `-` for stdin) or `--payload`.
Model slugs are immutable; an update may omit `slug` or repeat the targeted slug.
Vendor costs, conversion rates, credit bounds and fractional `usage.units` must be
**quoted decimal strings**, including in YAML; they are never converted to
floating-point numbers.

Update commands first GET the current resource, then merge the authored patch
into a writable-field-only PUT. Model updates preserve omitted fields and merge
individual `vendorCosts` dimensions and metadata keys. Slug and modality are
immutable. Rule updates preserve omitted selectors, credit caps and error policy;
`rates` is required and replaces the entire rates map. Entitlement, model and
request-type selectors are immutable: omit them or echo their current values.
An explicit `maximumCreditsPerRequest: null` clears the maximum cap; it does not
permit clearing selectors.

The API has no compare-and-swap/conditional update contract. Another writer
between the GET and PUT can be overwritten; coordinate concurrent changes to the
same model or conversion rule.

AI usage read values remain numeric credit counters, with an optional `ai` outcome
on report responses; `ai_usage` is calculation input, not a read-value type.

AI reports require `value.transactionId` and `phase` (`reserve` or `commit`), with
`behavior` omitted or `append`. No `Idempotency-Key` header is sent. Each command
sends once: after an uncertain failure, rerun the **identical phase and body** with
the same transaction identity. Do not start the provider call after a refused or
uncertain reserve. Numeric adjustments/refunds use `instances usage report` with
`--behavior append`, not `ai report`.

Model and inference lists return **one page**. JSON/YAML (also the default/table
fallback) preserve the page envelope and cursor; use `--cursor` and `--limit` to
continue. NDJSON emits one record per item, without the page envelope. Inference
lists accept `--from`, `--to`, `--status` and `--transaction-id`; reservations show
open holds only.

### Service account tokens

```shell
kaiten service-accounts tokens create deploy-bot --name "zone-eu-west-1" \
  --scope write:instances --scope read:feature_flags \
  --expires-at 2026-12-31T00:00:00Z
```

The plaintext token is printed once; the API stores a hash.

### Destructive commands

`delete`, `remove-entitlement`, `config unset` and the billing commands that
cannot be undone (`invoices void`, `invoices write-off`, `subscriptions cancel`,
`instance-addons detach`, the `revoke`s, ...) echo their target and ask for
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
