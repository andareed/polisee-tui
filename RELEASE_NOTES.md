# Polisee TUI v0.1.0

First stable release from the standalone Polisee TUI repository.

- Bubble Tea terminal policy inspection using polisee-lib v0.1.0.
- Policy and rule navigation, filtering, visual and text expression views, contextual inspection, and keyboard/mouse selection and copying.
- Open a local policy export or explore the embedded synthetic demonstration; use --snapshot for a single non-interactive frame.
- New --version option reports the application version without opening an export.
- Native layout verification expands tmux tab-compressed padding before comparing terminal columns and records the application exit status independently of tmux version.

## Downloads

Linux x86-64 and Windows x86-64. Run polisee-tui from a terminal. The private engine is compiled in: no library checkout or GitHub credentials are needed to run it. Source builds require the adjacent private polisee-lib repository.

Windows binaries are unsigned.

Each archive includes BUILDINFO.json with exact source revisions. Verify downloads against SHA256SUMS. Only the reviewed synthetic demonstration is embedded; customer exports, TODO files and local configuration are excluded.
