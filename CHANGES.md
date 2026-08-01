# Changes

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
- All 26 preview actions enabled: cluster seal/unseal/Raft restore; group,
  user, policy, engine, SSH role, auth method, and audit device lifecycle;
  secret-path browsing; and token revocation by accessor.
- Cancel-by-default confirmation for destructive actions, conflict prechecks,
  and post-operation verification through fake Vault API tests.
- Multiline policy/JSON preview and editing without flattening line breaks.
- Persistent per-field Help zones with examples and operational tips.
- Separate display and API paths for auth mounts, fixing userpass operations.
- Secure owner-only TOML configuration and token handling.
- Read-only cluster connectivity check with `--check`.
- Static release archives for Linux amd64/arm64, macOS amd64/arm64, and
  Windows amd64, with baseline architecture levels and a shared SHA-256
  checksum file.
- Tag-driven GitHub Actions release publication.
- Automated configuration, UI state-machine, fake Vault API, conflict,
  partial-save, and race tests.
