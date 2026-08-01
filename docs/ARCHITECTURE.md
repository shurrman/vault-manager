# Architecture

## Goal

`vault-manager` provides a human-oriented terminal view of one Vault cluster.
Lists use names such as `alice`, `bob`, group names, policy names,
mount paths, and role names. Vault IDs and accessors remain available in the
preview but are never the primary label.

The target runtime range is CentOS 7 through Debian 13. Releases are static Go
binaries built with `CGO_ENABLED=0` and `GOAMD64=v1`.

## Stack

- Go
- Bubble Tea for the event loop and state machine
- Bubbles for reusable text inputs
- Lip Gloss for layout and styling
- the official HashiCorp Vault Go client

The UI follows a reducer-style flow: input messages update explicit state and
return effects. Vault calls, terminal rendering, and configuration are adapters
around that state. Stale asynchronous responses carry request and resource IDs
and are ignored after the selection changes.

## Three-pane model

The left pane contains `Status`, `Groups`, `Users`, `Policies`, `Engines`,
`SSH`, `Auth Methods`, `Audit`, and `Tokens`. The middle pane contains the
selected section's items. The right pane contains the selected item's
properties, relationships, and editor.

Two states are always distinguishable:

- selected path: the section, item, and property that form the current chain;
- active item: the row that currently receives keyboard input.

Color is preferred. The active item uses a separate color and reverse video.
When color is unavailable or `NO_COLOR` is set, `*` marks the selected path and
`>` marks the active item. Terminals smaller than 80x20 receive a clear size
error; no single-pane mode is planned for the MVP.

## Human-oriented identity

A Vault user is an aggregate, not a single record:

```text
userpass account -> identity alias -> identity entity -> groups and policies
```

Aliases are joined through their auth mount accessor and alias name. Entity
names are not assumed to equal usernames. The UI resolves entity and group IDs
to names and searches those names. Ambiguous, auth-only, and entity-only states
are shown explicitly.

Token values are never listed. The Tokens section shows token accessors only
when permitted, labelled with display name, policy and identity context, and a
shortened accessor. Passwords are write-only.

## Independence and deployment integration

The application core depends only on Vault concepts and the official Vault
API. It has no external identity client, synchronization protocol, or
assumptions about how users and groups were provisioned. An operator without
an external identity source gets the complete editing workflow.

Deployment-specific integration is deliberately informational. The optional
`deployment.membership_notice` configuration value is displayed in the user
membership editor. For example, a site can warn that automation may later
restore managed memberships. The text is opaque to the application and does
not alter save behavior.

## Action model

Every published preview action is active. Actions still use the same
`LoadEditor` / `SaveEditor` boundary, including commands that do not resemble a
traditional field editor. This gives the reducer one lifecycle for loading,
editing, review, execution, stale-response rejection, and error recovery.

Action editors fall into three groups:

- structured forms for create, update, tune, enable, and Raft restore;
- confirmation-only forms for delete, disable, seal, and revoke;
- read/list forms for browsing a path inside a selected secrets engine.

ACL policy bodies and SSH role JSON use a multiline textarea. Preview rows keep
embedded newlines and indentation instead of flattening them. The lower part of
every form reserves a bounded Help area for the selected field's syntax,
examples, and operational tips.

The auth mount model keeps the mount-table path (`userpass/`) separate from the
runtime API path (`auth/userpass/`) so display/navigation paths cannot leak into
userpass requests.

## Safe save flow

The editor stays in the right pane. `Ctrl+S` or `Esc` reviews a dirty draft.
Confirmation-only actions open review immediately. Cancel is selected by
default, and no action autosaves.

Before a single-object write, the application:

1. reloads the object;
2. compares its fingerprint with the editor's original value;
3. refuses to overwrite an externally changed object;
4. displays a redacted or compact diff;
5. sends a minimal update request;
6. reloads and verifies the saved result.

Delete/disable operations use the same precheck and verify absence by a
follow-up read or list. Commands without a stable readable representation use
the strongest response/status verification exposed by Vault.

Tokens, passwords, unseal keys, and real secret values are excluded from logs,
errors, snapshots, and test fixtures.

### User membership

Vault represents direct entity membership on internal group objects, so a
single user edit may affect multiple groups. The manager:

1. snapshots the entity and all selectable groups when opening the editor;
2. validates every affected group before the first write;
3. writes affected groups sequentially in deterministic name order;
4. verifies each successful write;
5. updates direct entity properties after group writes;
6. reloads the actual user preview.

This sequence is intentionally non-atomic. If a write may have partially
applied, the application does not attempt an unsafe compensating rollback. It
reports applied, failed, and pending operations, closes the stale draft, and
requires the operator to reload before continuing.

## Testing boundary

State transitions, fallback markers, cancel-by-default confirmations, conflict
detection, every action family, and partial membership writes use automated
tests. API contracts use an in-process fake HTTP Vault server. A disposable development
Vault remains the integration target for live write testing; the configured
real cluster is used only for an explicit read-only smoke check.

Daily Raft snapshot backup and restore tooling is a separate future task.
