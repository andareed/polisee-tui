# Polisee TUI v0.2.0

Export an individual main policy from the terminal viewer.

- Select a main policy and choose **Export** in the header, or press **Ctrl+E**, to choose a destination XML path. Tab completes paths and Escape cancels.
- Exports use EyeSight's `RULES` format and preserve the complete policy, every subrule, and original XML metadata through polisee-lib v0.2.0.
- Existing exports require replacement confirmation. The loaded source file, including aliases, is protected from overwriting.
- Open single-policy exports directly in Polisee. Export controls stay available in narrow terminal layouts.
- Linux builds remain statically linked for headless servers, including the launcher and Makefile correction made after v0.1.0.

## Downloads

Linux x86-64 and Windows x86-64. The Linux executable has no glibc or dynamic-loader dependency. Run `polisee-tui` from a UTF-8 terminal; `--version` reports `0.2.0`. The compiled engine is included, so downloads require no library checkout or GitHub credentials. Source builds require the adjacent private polisee-lib repository.

Windows executables are unsigned and cross-compiled, without Windows runtime testing.

Each archive includes `BUILDINFO.json` with exact committed source revisions, `VERSION`, and a runtime README. Verify downloads against `SHA256SUMS`. Only the reviewed synthetic demonstration is embedded; customer exports and local configuration are excluded.

## Validation

Checks, race tests, export/save/cancel/source-protection regression tests, and the native terminal smoke passed. The terminal smoke saves a main policy and verifies its subrules. Release binaries were built from clean Git exports without GitHub build runners. Linux version and snapshot checks passed; Windows PE executable headers were verified.
