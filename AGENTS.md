# Polisee TUI

- This is a standalone Bubble Tea v2 consumer of the adjacent polisee-lib.
  The GUI is the reference for presentation and keyboard conventions.
- Keep parsing, domain interpretation, labels and policy matching in the library.
  This application owns terminal rendering, navigation and literal table filtering.
- Preserve exported values, condition operators, subrule source order, declared
  and effective enabled states. Never evaluate unavailable expressions or fetch
  their references.
- Keep primary names and values prominent. IDs remain visible; diagnostics and
  complete contextual fields are supporting information.
- Use synthetic tests only. The embedded demo is the reviewed library fixture
  also used by polisee-gui. Do not copy private exports or the field catalog.
- Run make check, make race and make smoke after integration changes.
  Do not create releases, tags, repositories or version bumps unless requested.
