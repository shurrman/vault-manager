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
- Visible disabled controls for deferred and destructive operations.
- Secure owner-only TOML configuration and token handling.
- Read-only cluster connectivity check with `--check`.
- Static Linux amd64 release binary compatible with the baseline x86-64
  instruction set.
- Automated configuration, UI state-machine, fake Vault API, conflict,
  partial-save, and race tests.
