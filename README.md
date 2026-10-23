# Polisee TUI

A standalone Go / Bubble Tea v2 counterpart to polisee-gui, using the adjacent
polisee-lib. It follows the GUI's orange and grey palette, browsing order,
condition hierarchy and compact detail sections. Names, descriptions and values
lead; IDs remain visible and complete technical fields are available in a
contextual inspector.

## Download

The [v0.1.0 Linux x86-64 release](https://github.com/andareed/polisee-tui/releases/tag/v0.1.0)
contains a static executable named polisee-tui. Extract the archive, then run:

```sh
./polisee-tui /path/to/policies.xml
```

No desktop, library checkout or glibc installation is needed. An older executable
named polisee-tui2 may come from the original project and use dynamic linking;
use the released polisee-tui executable or rebuild this project.

## Run

Requires Go 1.26+ and a UTF-8 terminal. Keep this module beside polisee-lib:

```text
polisee/
  polisee-lib/
  polisee-gui/
  polisee-tui/
```

```sh
cd polisee-tui
./run.sh                      # GUI's same reviewed synthetic demo
./run.sh /path/to/policies.xml # open an explicit local export
./run.sh build                # build bin/polisee-tui
./bin/polisee-tui --help
```

The launcher rebuilds incrementally before starting. The launcher and Makefile
build with `CGO_ENABLED=0`, producing a static Linux executable without a glibc
or dynamic-loader dependency. This also supports headless servers with older
glibc versions; a UTF-8 terminal is sufficient. No global CLI, C compiler,
Node, desktop display or running GUI is required. Dependencies are pinned in
go.mod/go.sum with the same sibling library replacement as the GUI.

Builds omit optional Git revision stamping so the launcher also works from a
source directory without Git metadata or a mounted checkout with inaccessible
repository metadata. No `GOFLAGS` workaround is needed.

Bubble Tea, Bubbles and Lip Gloss use their current v2 module paths at
[charm.land](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md).

Exports are read locally, never modified or uploaded. Open a new file with
**Ctrl+O**; type a path, use Tab or the suggestions, and press Enter. A failed
open preserves the current document. Files must be regular UTF-8 policy XML
exports up to 64 MiB. Nothing scans the private data directory on startup.

## Browsing

- The tree and details have independent focus and scrolling. At 78 columns or
  wider they appear side by side; narrower terminals show the focused pane at
  full width. Alt+H and Alt+L focus the tree and details.
- Folders come first; folders and top-level policies use numeric name ordering.
  Subrules preserve exported source order. Enabled, disabled and disabled by
  parent remain distinct.
- A policy starts with its name, effective state, ID and description, followed by
  Scope & Filters, Conditions, Actions, Timers & Admissions, then folded Sub Rules
  where child rules exist. Attributes appears last, showing all exported rule
  attributes independently of diagnostics. Empty sections and default-only timing
  start folded.
- Conditions retain the library's Boolean tree, grouping metadata, exported
  unknown-result behavior and unknown operators. Unavailable external expressions
  show a notice; the app does not resolve them or evaluate criteria. Complete
  section frames and hanging indents keep wrapped values aligned. XML formatting
  whitespace is omitted from inline summaries and retained in the inspector.
- Action names and parameter values appear together. Readable group names lead,
  with reference IDs alongside; technical identifiers follow in muted text.
  Each action shows its own Enabled/Disabled state; the rule's state is shown
  separately on the rule.
  Schedules and timer units show only what the export supplies.
- Diagnostics start hidden. Toggle their supporting previews with d; previews
  use spare space without reducing value width or changing row heights and scroll
  position. PLUGIN_NAME appears first in previews and inspectors; filter IDs appear
  only in diagnostics. Enter on a data row opens
  its full exported context. Exact values are quoted to preserve whitespace,
  empty values and controls safely in a terminal.
- Search combines text, property, group, action and action parameter filters
  through the library. Matching ancestors stay visible and n/N visits direct
  matches in displayed tree order. Quoted phrases work as in the GUI.
  `/` reveals all five filter fields. Tab/Shift+Tab moves between fields without
  accepting suggestions; arrows choose a suggestion and Enter applies it.
  Escape closes the editor and returns to the tree.
  Suggestions come from the loaded export; named groups are deduplicated by
  name/ID and selecting one searches its ID. Typed IDs remain valid.
- p adds/removes a pinned shortcut at the top of the tree. Pins and browsing state
  last for the current document; a successful file replacement clears them.
- Enter on the Segments row opens an eight-row overview with a name/ID filter. Enter opens the
  selected segment's 25-row range pages. Range filters match literal exported
  addresses, IDs and additional values; they do not infer address containment.
  Each segment retains its filter, page, selection and viewport position when
  moving with left/right. Enter inspects a range; m inspects segment metadata.
- Mouse clicks select rows, toggle headings and open inspectors. In policy details
  and exported fields, drag to select text, or hold briefly before dragging.
  A normal click still activates the row. Selection stays within the text pane;
  copied text omits section borders, the tree and empty padding. Ctrl+C
  copies the selection; Escape clears it. Wide characters, emoji and combining
  characters stay intact.
- The wheel scrolls the owning pane without changing the policy and clears text
  selection. --no-mouse leaves mouse handling to the terminal. Ctrl+C sends the current
  ID when no text is selected. Clipboard commands use terminal OSC 52; support
  depends on the terminal and multiplexer. Terminals such as
  [kitty](https://sw.kovidgoyal.net/kitty/overview/) also offer Shift+drag for native
  terminal selection while an application uses the mouse.

## Keys

| Key | Action |
| --- | --- |
| Alt+H / Alt+L | Focus tree / details |
| j/k or up/down; Page Up/Down; Home/End | Move within the focused pane |
| h/l or left/right | Collapse/expand a branch or section heading |
| Left/right in ranges | Previous/next segment |
| Enter; Escape | Open/inspect/apply; close/return |
| [ / ] | Previous/next detail heading |
| e | Toggle expand/collapse all in the focused pane |
| / | Open Text, Property, Group, Action and Value filters |
| Tab / Shift+Tab in filters | Next / previous field |
| Up/down then Enter | Choose and apply a suggestion |
| Ctrl+R; n / N | Clear policy filters; next/previous direct match |
| p; Ctrl+C | Pin; copy selected text, otherwise the selected ID |
| d; v | Diagnostics; Visual/Text conditions |
| /; Page Up/Down; m in a browser | Literal filter; page; segment metadata |
| Ctrl+O; r | Open local XML; reload the explicit file |
| Drag / hold and drag | Select text in details or exported fields |
| Escape with a selection | Clear the selection |
| ?; Ctrl+Q | Help; quit confirmation (Enter quits, Escape cancels) |

Both condition modes use polisee-lib's shared expression projection. Visual mode
uses inline AND/OR connectors and scope brackets that continue through wrapped
filter details. Text mode (`v`) uses an infix expression with line breaks and
actual operand values. Source grouping and contextual fields remain inspectable.

In an editor, ordinary letters, including h/j/k/l, remain text. Escape exits a
browser filter before closing its inspector. Ctrl+C copies selected text, or the current policy ID
outside editors. Ctrl+Q asks for confirmation; Enter quits and Escape cancels,
preserving the current view or editor. The quit shortcut stays on the bottom bar.
Terminal
font size, clipboard support and window size are controlled by the terminal. This app has no desktop
zoom or release-check network requests. It stores no startup file preferences.

## Checks

```sh
make check  # formatting, vet, synthetic regression tests and compilation
make race   # race-enabled tests, including stale asynchronous results
make smoke  # real terminal session through an isolated tmux socket
```

The smoke check requires Python 3 and tmux. It opens the demo, searches a nested
rule, opens a fabricated 24-segment / 28,800-range export, pages and filters ranges,
checks held-drag selection, copying, normal clicks, state restoration and failed
file replacement, resizes the terminal, and quits. Its synthetic XML and terminal captures stay outside source under
<code-root>/.agent-work/polisee/tui/. It never touches a user's tmux session.

A single frame can also be printed without an interactive terminal:

```sh
./bin/polisee-tui --snapshot --width 140 --height 42
```

## Provenance

XML parsing, enabled-state inheritance, expression structure, field labels and
policy matching come from polisee-lib. Presentation and keyboard conventions
follow the current polisee-gui workspace. The embedded demo is copied from the
GUI's reviewed synthetic fixture, originally polisee-lib/testdata/policies.xml.
This is a separate implementation: it imports neither GUI internals nor the
historical TUI, duplicates no field catalog, and contains no private exports.
