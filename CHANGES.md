# Changes

## 0.2

- Enabled all 26 preview actions, including cluster seal/unseal/Raft restore;
  group, user, policy, engine, SSH role, auth method, and audit lifecycle;
  secret-path browsing; and token revocation by accessor.
- Added structured forms, read/list views, and cancel-by-default confirmations
  for destructive operations.
- Added optimistic conflict checks and post-operation verification to the new
  write actions, with state-changing coverage through fake Vault API tests.
- Preserved line breaks and indentation in policy and JSON previews, with
  multiline editing for ACL policies and SSH role configuration.
- Added persistent per-field Help areas with examples and operational tips to
  editors.
- Separated display and API paths for auth mounts, fixing userpass operations.

## 0.1

- Three-pane terminal interface with Status, Groups, Users, Policies, Engines,
  SSH, Auth Methods, Audit, and Tokens.
- Human-readable identity, group, policy, mount, and role names, with Vault IDs
  retained in detailed previews.
- Arrow-key and Tab/Shift+Tab pane navigation. Tab from the item list focuses
  the first preview action.
- Color interface with text-marker fallback and scrolling for long lists.
- Editors for internal group policies, members, nested groups, and metadata.
- Editors for identity entity policies, internal group membership, and
  disabled state.
- Userpass password changes with write-only password fields.
- Optimistic conflict detection and post-write verification.
- Explicit reporting for applied, failed, and pending operations when
  multi-group membership changes partially apply.
- Optional deployment-specific membership notice without a runtime dependency
  on external provisioning systems.
- Visible disabled controls for deferred and destructive operations.
- Secure owner-only TOML configuration and token handling.
- Read-only cluster connectivity check with `--check`.
- Static release archives for Linux amd64/arm64, macOS amd64/arm64, and
  Windows amd64, with baseline architecture levels and a shared SHA-256
  checksum file.
- Tag-driven GitHub Actions release publication.
- Automated configuration, UI state-machine, fake Vault API, conflict,
  partial-save, and race tests.
