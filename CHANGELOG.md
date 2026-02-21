# Changelog

<!-- AI agents: add entries under an ## [YYYY.MM.DD] header, use date +'%F %H:%M' e.g. ## [2026.2.71]. Do NOT add version numbers. Do NOT duplicate headings or dates - simply update the existing date if you're adding to it. If the file gets over 1000 lines long, truncate the oldest releases, keep items concise and leave this comment as-is-->

## [2026.02.21]

### Added
- Collaboration tool (`collab` / `collab_wait`) for cross-agent communication via shared filesystem mailbox. Session-based message exchange with UUID addressing, file locking, atomic writes, MCP Roots auto-detection for participant names, and MCP notifications for message events.

### Changed
- `collab_wait` converted from MCP task tool to regular tool with internal polling, as MCP clients (including Claude Code) don't yet support task augmentation. Configurable poll interval via `poll_interval_seconds` parameter or `COLLAB_POLL_INTERVAL` env var (default: 60s).
- `collab` create/join responses now include usage hints to guide agents without bloating tool descriptions.
- `collab_wait` automatically enabled when `collab` is enabled (registry alias).
