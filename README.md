# vault-manager

[English](README.md) · [Русский](README.ru.md) · [Español](README.es.md) ·
[简体中文](README.zh-CN.md)

![vault-manager interface preview](docs/images/vault-manager.svg)

`vault-manager` is an independent, interactive terminal manager for a
HashiCorp Vault cluster. It talks only to the Vault HTTP API: external identity
sources, provisioning systems, and deployment-specific synchronization are not
required.

Current release: `0.3`.

Prebuilt archives and their checksums are available on the
[GitHub Releases page](https://github.com/shurrman/vault-manager/releases/latest).

The interface uses three panes:

1. sections such as Groups, Users, and Engines;
2. items in the selected section;
3. properties and actions for the selected item.

Human-readable names are primary. Vault IDs and accessors remain visible in
previews when they are useful for diagnostics.

## Current scope

The manager reads and operates Status, Groups, Users, Policies, Engines, SSH,
Auth Methods, Audit, and Tokens:

| Area | Available actions |
| --- | --- |
| Cluster | Seal, submit unseal keys, save and restore Raft snapshots |
| Internal groups | Create, edit policies/membership/metadata, delete |
| Users | Create userpass account, edit entity access, change password, revoke userpass access, delete entity |
| ACL policies | Create, edit multiline HCL/JSON body, delete |
| Secrets engines | Navigate and read the existing value hierarchy, enable, disable |
| SSH roles | Create, edit complete JSON configuration, delete |
| Auth methods | Tune, enable, disable |
| Audit devices | Enable, disable |
| Tokens | Revoke by accessor |

Multiline preview values preserve line breaks and indentation. Every editor has
a lower Help area with field-specific syntax, examples, and operational tips.
Comma-separated relationship fields offer filtered choices from current Vault
objects; already selected values are excluded and Up/Down plus Enter/Tab select a
match. HCL/JSON fields are validated before confirmation and again before an API
write, with line and column details for syntax errors.

The TUI language is selected with `ui.language`: `en`, `ru`, `es`, or `zh`. It
defaults to `en` when omitted.
Destructive actions open a confirmation screen with Cancel selected by default;
Vault still enforces the caller's ACL capabilities for every operation.

## Build

Go 1.25 or newer is required for development. The default build produces a
static Linux amd64 binary using the baseline x86-64 instruction set:

```sh
make build
```

The result is `bin/vault-manager`. The build uses `CGO_ENABLED=0` and
`GOAMD64=v1` for portability from CentOS 7 through Debian 13.

The complete release matrix is produced with:

```sh
make release
```

| OS | Architecture | Archive |
| --- | --- | --- |
| Linux | amd64 | `vault-manager_0.3_linux_amd64.tar.gz` |
| Linux | arm64 | `vault-manager_0.3_linux_arm64.tar.gz` |
| macOS | amd64 | `vault-manager_0.3_darwin_amd64.tar.gz` |
| macOS | arm64 | `vault-manager_0.3_darwin_arm64.tar.gz` |
| Windows | amd64 | `vault-manager_0.3_windows_amd64.zip` |

All binaries use `CGO_ENABLED=0`; amd64 builds use `GOAMD64=v1`, and arm64
builds use the baseline `GOARM64=v8.0`. `dist/checksums.txt` contains SHA-256
checksums for all five archives. Pushing a two-part `vX.Y` tag runs the tests,
builds this matrix, and creates or updates the GitHub Release automatically.

## Configuration

Copy `.vault-manager.example` to `$HOME/.vault-manager`, replace the address
and token, and restrict the file to its owner:

```sh
cp .vault-manager.example "$HOME/.vault-manager"
chmod 600 "$HOME/.vault-manager"
```

```toml
[vault]
address = "http://vault.wsoft:8400"
token = "replace-me"
request_timeout = "10s"

[ui]
color = "auto"
language = "en"
```

`ui.language` accepts the two-letter codes `en`, `ru`, `es`, and `zh`; the
default is `en`. Plain HTTP addresses are supported for trusted internal
reverse-proxy networks such as `http://vault.wsoft:8400`.

Without `--config`, the application loads `$HOME/.vault-manager`, regardless
of its current working directory. The configuration is rejected if it is
readable by group or others. Both `.vault-manager` and `.env` are excluded
from Git.

An installation that synchronizes membership from another system can add a
local warning without changing the application:

```toml
[deployment]
membership_notice = "Membership is managed externally and may change after synchronization."
```

This text is shown before editing user groups. It is informational; the core
does not call or interpret the external system.

## Run

```sh
bin/vault-manager
```

Print the application version with `bin/vault-manager --version`.
Use a different configuration path with `--config /path/to/config.toml`.
The read-only connectivity smoke check is:

```sh
bin/vault-manager --check
```

The check lists every section and previews its first item. It performs no
write operations.

## Keyboard controls

- `↑`/`↓`, `Home`, `End`: move in the active pane;
- `←`/`→`: move between panes;
- `Tab`/`Shift+Tab`: move to the pane on the right/left; `Tab` from the
  middle pane selects the first action in the preview;
- `Enter`: activate an item, action, or editor field;
- in comma-separated list fields, `↑`/`↓` choose a filtered suggestion and
  `Enter` or `Tab` inserts it;
- on an Engines `Path` row, `Enter` opens value browsing directly; inside it,
  `Enter` descends/opens and `Esc` or `←` returns to the parent path;
- `Space`: toggle a boolean editor field;
- `Ctrl+S`: review the current editor action;
- `Enter` inside a multiline field: insert a newline;
- `Esc`: accept a multiline field or review/leave an editor;
- `r`: refresh the active data;
- `?`: show a short key reminder;
- `q` or `Ctrl+C`: quit outside text input.

Color is used when supported. With `ui.color = "never"`, `NO_COLOR`, or a
terminal without color, `*` marks the selected path and `>` marks the active
item. The minimum terminal size is 80x20.

## Membership and save safety

Vault stores entity membership on internal group objects. Changing a user's
Groups field can therefore require several sequential writes and is not
atomic. Before writing, `vault-manager` checks the entity and every affected
group for concurrent changes. It then updates groups in deterministic name
order and verifies each result.

If a later write fails, successful earlier writes are not rolled back. The UI
reports applied, failed, and not-attempted operations, reloads the actual Vault
state, and requires a fresh edit before continuing.

Updates and deletes compare the current object with the version originally
loaded and refuse to overwrite concurrent changes whenever Vault exposes a
readable representation. Writes are followed by a read/list verification.
Commands without a stable readable fingerprint, such as unseal-key submission
and Raft restore, verify the response or completion status exposed by Vault.
Passwords, unseal keys, and token values are never displayed in full, logged,
or included in error messages.

## Development checks

```sh
make test
make test-race
make vet
```

API write tests use an in-process fake Vault HTTP server. The configured
cluster is used only by the explicit read-only `--check` command.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for design details.
