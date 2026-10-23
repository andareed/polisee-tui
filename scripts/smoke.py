#!/usr/bin/env python3
"""Exercise the actual Bubble Tea executable in an isolated terminal."""
import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import time

project = Path(__file__).resolve().parents[1]
code_root = next(
    (path for path in project.parents if path.name in ("code.sandbox", "code.playground")),
    project.parent,
)
artifacts = code_root / ".agent-work" / "polisee" / "tui"
artifacts.mkdir(parents=True, exist_ok=True)
# A mounted code root can be SSHFS, which cannot host a local Unix socket.
# Keep captures under the code root and the runtime socket on a local filesystem.
runtime = tempfile.TemporaryDirectory(prefix="polisee-tui-smoke-", dir=os.environ.get("XDG_RUNTIME_DIR"))
socket = Path(runtime.name) / "tmux.sock"
session = "polisee-tui-smoke"
target = session
binary = project / "bin" / "polisee-tui"
exit_status = artifacts / "smoke-exit-status.txt"
exit_status.unlink(missing_ok=True)


def tmux(*args, check=True):
    result = subprocess.run(
        ["tmux", "-S", str(socket), "-f", "/dev/null", *args],
        check=False, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    if check and result.returncode:
        raise RuntimeError(f"tmux {args!r}: {result.stderr.strip()}")
    return result.stdout


def capture():
    # tmux may compress padding into tab stops; compare displayed columns.
    return tmux("capture-pane", "-p", "-t", target).expandtabs(8)


def wait_for(*texts, timeout=8):
    deadline = time.monotonic() + timeout
    last = ""
    while time.monotonic() < deadline:
        last = capture()
        if all(text in last for text in texts):
            return last
        time.sleep(0.08)
    (artifacts / "smoke-failure.txt").write_text(last)
    raise AssertionError(f"Terminal did not show {texts!r}; see {artifacts / 'smoke-failure.txt'}")


def keys(*values):
    for value in values:
        tmux("send-keys", "-t", target, value)
        if value == "Escape":
            time.sleep(0.15)


def text(value):
    tmux("send-keys", "-t", target, "-l", value)


def mouse(x, y, event):
    code, suffix = {"down": (0, "M"), "drag": (32, "M"), "up": (0, "m")}[event]
    text(f"\x1b[<{code};{x + 1};{y + 1}{suffix}")


def visible_position(frame, value, minimum_x=0):
    for y, row in enumerate(frame.splitlines()):
        if y < 5:
            continue
        x = row.find(value, minimum_x)
        if x >= 0:
            return x, y
    raise AssertionError(f"Visible text {value!r} not found")


xml_path = artifacts / "smoke-synthetic.xml"
with xml_path.open("w") as fixture:
    fixture.write('<POLICY_FOLDER ID="synthetic" NAME="Synthetic smoke"><RULE ID="smoke" NAME="Synthetic smoke policy" APP_VERSION="synthetic-1" CACHE_TTL="60" CACHE_TTL_SYNCED="true" CLASSIFICATION="synthetic" DESCRIPTION="" NOT_COND_UPDATE="true" RECHECK_MAIN_RULE_DEF="true">')
    fixture.write('\n  <EXPRESSION EXPR_TYPE="OR">\n    <CONDITION FIELD_NAME="demo.category" LABEL="Device category">\n      <FILTER TYPE="equals" FILTER_ID="layout-filter-a" CASE_SENSITIVE="false">\n')
    for index in range(12):
        fixture.write(f'        <VALUE VALUE="Example category/Branch {index:02}/Managed endpoints"/>\n')
    fixture.write('      </FILTER>\n    </CONDITION>\n    <CONDITION FIELD_NAME="in-group" LABEL="Member of group">\n      <FILTER FILTER_ID="layout-filter-b">\n        <GROUP ID="group-a" NAME="Example lab devices"/>\n      </FILTER>\n    </CONDITION>\n  </EXPRESSION>\n')
    for segment in range(24):
        fixture.write(f'<SEGMENT ID="seg-{segment}" NAME="Segment {segment}" EXTRA="smoke metadata"><RANGES>')
        for index in range(1200):
            fixture.write(
                f'<RANGE FROM="192.0.2.{index}" TO="198.51.100.{index}" '
                f'ID="range-{segment:02}-{index:04}" EXTRA="smoke range"/>'
            )
        fixture.write("</RANGES></SEGMENT>")
    fixture.write("</RULE></POLICY_FOLDER>")

try:
    # Exercise the documented launcher without relying on caller build flags.
    launcher_env = os.environ.copy()
    launcher_env.pop("GOFLAGS", None)
    launched = subprocess.run(
        [str(project / "run.sh"), "--snapshot", "--width", "100", "--height", "30"],
        cwd=project, env=launcher_env, capture_output=True, text=True,
    )
    assert launched.returncode == 0, f"Normal launcher failed: {launched.stderr.strip()}"
    assert "Office access" in launched.stdout, "Normal launcher did not render its demo"
    (artifacts / "launcher-terminal.ans").write_text(launched.stdout)
    # Older tmux versions do not expose pane_dead_status. Record the actual
    # application status in the launcher shell and still require the pane to die.
    launch = (shlex.quote(str(binary)) + '; result=$?; printf "%s" "$result" > ' +
              shlex.quote(str(exit_status)) + '; exit "$result"')
    tmux("new-session", "-d", "-x", "140", "-y", "42", "-s", session, launch)
    tmux("set-option", "-w", "-t", target, "remain-on-exit", "on")
    frame = wait_for("Office access", "Conditions", "Actions")
    assert "Ctrl+Q quit" in frame.splitlines()[-1], "quit shortcut is missing from the bottom bar"
    assert "e/E" not in frame, "duplicate expand/collapse shortcuts remain in the headers"
    (artifacts / "demo-terminal.txt").write_text(frame)
    assert "FILTER_ID" not in frame and "Filter ID" not in frame, "filter ID is visible with diagnostics off"
    text("d")
    frame = wait_for('PLUGIN_NAME="Inventory"', 'FILTER_ID="first"')
    diagnostic_row = next(row for row in frame.splitlines() if 'PLUGIN_NAME="Inventory"' in row)
    assert "LOGIC=" not in diagnostic_row or diagnostic_row.index("PLUGIN_NAME=") < diagnostic_row.index("LOGIC="), "plugin name is not first in diagnostics"
    (artifacts / "diagnostics-terminal.txt").write_text(frame)
    text("d")
    frame = wait_for("Diagnostics off")
    assert "FILTER_ID" not in frame and "Filter ID" not in frame, "filter ID stays visible after hiding diagnostics"
    text("/")
    wait_for("Property", "Group", "Action", "Value", "Tab field")
    keys("Tab", "Tab")
    text("staff")
    keys("Enter")
    wait_for("1 / 3 matches", "Office access")
    keys("C-r", "Escape")
    wait_for("All policies")
    text("v")
    frame = wait_for("Text [v]", 'Example OS', "AND NOT")
    assert "PARENTHESIS" not in frame, "text mode includes redundant wrapper nodes"
    (artifacts / "demo-text-terminal.txt").write_text(frame)
    text("v")
    frame = wait_for("Visual [v]", "Office access", "Operating system")
    x, y = visible_position(frame, "Office access", minimum_x=43)
    mouse(x, y, "down")
    time.sleep(0.42)
    wait_for("Text selected", "Ctrl+C copy")
    mouse(x + 5, y, "drag")
    mouse(x + 5, y, "up")
    frame = wait_for("Text selected", "Office access")
    (artifacts / "selection-terminal.txt").write_text(frame)
    (artifacts / "selection-terminal.ans").write_text(tmux("capture-pane", "-p", "-e", "-t", target))
    keys("C-c")
    wait_for("Selected text sent to terminal clipboard", "Office access")
    keys("Escape")
    frame = capture()
    x, y = visible_position(frame, "Scope & Filters", minimum_x=43)
    mouse(x, y, "down")
    mouse(x, y, "up")
    wait_for("╭─ ▸ Scope & Filters")
    mouse(x, y, "down")
    mouse(x, y, "up")
    wait_for("╭─ ▾ Scope & Filters")
    keys("M-l", "j", "k", "j")
    wait_for("Office access", "ID office")
    keys("[", "h")
    wait_for("╭─ ▸ Scope & Filters")
    keys("l")
    wait_for("╭─ ▾ Scope & Filters")
    text("?")
    wait_for("Keyboard shortcuts", "Alt+H / Alt+L")
    keys("Escape")
    text("/")
    text('"Nested disabled"')
    keys("Enter")
    frame = wait_for("1 / 3 matches", "Nested disabled rule", "Disabled by parent")
    (artifacts / "search-terminal.txt").write_text(frame)
    keys("C-r")
    wait_for("All policies")
    keys("C-o")
    wait_for("Open policy XML")
    text(str(xml_path))
    keys("Enter")
    frame = wait_for("Synthetic smoke policy", "Member of group", "Example lab devices", "╮", "╯")
    (artifacts / "layout-terminal.txt").write_text(frame)
    rows = frame.splitlines()
    header = next(index for index, row in enumerate(rows) if "Conditions" in row and "╭" in row)
    left, right = rows[header].index("╭"), rows[header].rindex("╮")
    closed = False
    for row in rows[header + 1:]:
        if len(row) > right and row[left] == "╰" and row[right] == "╯":
            closed = True
            break
        assert len(row) > right and row[right] == "│", "condition frame lost its right edge"
        assert row[left + 1:right].strip(), "XML indentation became a blank terminal row"
    assert closed, "condition frame never closed"
    keys("M-l", "End")
    frame = wait_for("▾ Attributes", "9 attributes", "APP_VERSION", "synthetic-1", "CACHE_TTL", "60", "DESCRIPTION", '\"\"', "NOT_COND_UPDATE", "RECHECK_MAIN_RULE_DEF")
    (artifacts / "attributes-terminal.txt").write_text(frame)
    text("d")
    wait_for("▾ Attributes", "APP_VERSION", "synthetic-1", "RECHECK_MAIN_RULE_DEF")
    text("d")
    keys("Home")
    keys("M-l", "]", "]", "j", "j", "j", "Enter")
    frame = wait_for("Exported fields", "FILTER", "layout-filter-a", "text =", "\\n")
    x, y = visible_position(frame, "false")
    mouse(x, y, "down")
    mouse(x + 4, y, "drag")
    mouse(x + 4, y, "up")
    wait_for("Text selected")
    keys("C-c")
    wait_for("Selected text sent to terminal clipboard", "Exported fields")
    keys("Escape")
    wait_for("Exported fields")
    keys("Escape")
    keys("Home", "]", "Down", "Enter")
    wait_for("Segments", "24 / 24", "page 1 / 3")
    keys("NPage")
    wait_for("page 2 / 3", "Segment 8")
    keys("PPage")
    keys("Enter")
    wait_for("Segment ranges", "page 1 / 48", "range-00-0000")
    keys("NPage")
    wait_for("page 2 / 48", "range-00-0025")
    text("/")
    text("range-00-0017")
    keys("Enter")
    wait_for("1 / 1200", "range-00-0017")
    keys("Right")
    wait_for("ID seg-1", "1200 / 1200")
    keys("Left")
    frame = wait_for("ID seg-0", "1 / 1200", "range-00-0017")
    (artifacts / "ranges-terminal.txt").write_text(frame)
    keys("Enter")
    wait_for("Exported fields", "EXTRA", '"smoke range"')
    keys("Escape", "Escape", "Escape")
    wait_for("Synthetic smoke policy", "Scope & Filters")
    keys("C-o")
    wait_for("Open policy XML")
    text(str(artifacts / "deliberately-missing-policy.xml"))
    keys("Enter")
    wait_for("Open failed", "Synthetic smoke policy")
    tmux("resize-window", "-t", target, "-x", "80", "-y", "24")
    frame = wait_for("polisee", "Policy details")
    (artifacts / "narrow-terminal.txt").write_text(frame)
    assert len(frame.splitlines()) == 24, "terminal resize lost rows"
    keys("C-q")
    frame = wait_for("Quit Polisee?", "Enter to quit", "Esc to cancel")
    assert tmux("display-message", "-p", "-t", target, "#{pane_dead}").strip() == "0", "Ctrl+Q quit without confirmation"
    (artifacts / "quit-confirmation-terminal.txt").write_text(frame)
    keys("Escape")
    wait_for("Synthetic smoke policy", "Ctrl+Q quit")
    keys("C-q", "Enter")
    deadline = time.monotonic() + 5
    status = ""
    while time.monotonic() < deadline:
        dead = tmux("display-message", "-p", "-t", target, "#{pane_dead}").strip()
        result = exit_status.read_text().strip() if exit_status.exists() else ""
        status = dead + " " + result
        if status == "1 0":
            break
        time.sleep(0.08)
    assert status == "1 0", f"program did not quit cleanly: {status!r}"
    print(f"PASS: normal launcher, terminal startup, focus, search, visual/text conditions, frames/whitespace/inspection, held-drag selection/copy/clicks, 28,800 ranges, state, failed load, resize and confirmed quit; captures: {artifacts}")
finally:
    tmux("kill-server", check=False)
    runtime.cleanup()
