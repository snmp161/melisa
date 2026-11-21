# Project Hygiene Backlog

This file tracks the remaining hygiene tasks after completion of the first-priority developer workflow, MCP catalog, MCP stdio testing, release snapshot work, report schema validation, and Dependabot update grouping.

Each task should be implemented independently with focused verification, documentation updates where applicable, and a green full CI run before delivery.

## Priority 2

### 6. Refresh the integration-test distribution matrix

**Size:** S, plus a separately approved manual test run

**Evidence:** `tests/integration/distros.txt` includes Fedora 40, which is end-of-life. The README lists the same matrix.

**Scope:**

- Select currently supported distribution releases while preserving the intended Debian/Ubuntu/Fedora/Rocky coverage.
- Update `tests/integration/distros.txt`, README, and affected documentation consistently.
- Record image pull or repository limitations separately from Melisa regressions.
- Do not move the privileged Docker suite into GitHub Actions as part of this task.

**Done when:** the documented matrix contains supported releases and the approved manual Docker run has a recorded result for every target. The suite must not be run on a developer or production host without explicit approval.

### 7. Pin GitHub Actions to full commit SHAs

**Size:** S–M

**Evidence:** CI and release workflows currently reference `actions/checkout`, `actions/setup-go`, and `goreleaser/goreleaser-action` through movable major-version tags such as `@v7`.

**Scope:**

- Resolve the intended release of each Action and replace the major tag with its full 40-character commit SHA.
- Retain a human-readable version comment next to every SHA.
- Keep Dependabot enabled for GitHub Actions so pinned SHAs continue to receive update PRs.
- Preserve workflow permissions, triggers, Go `1.27.4`, and GoReleaser `v2.20.2`.
- Do not combine this mechanical supply-chain change with unrelated workflow refactoring.

**Done when:** all workflows use reviewed full SHAs, Dependabot still recognizes the dependencies, and Test & Build, BPF smoke, and Release snapshot are green. Actual publishing remains tag-only.

## Documentation hygiene

### 9. Align AI and MCP documentation with the implementation

**Priority:** 2

**Size:** S–M

**Evidence:** the code and README describe 27 AI anti-patterns, while `docs/en/13-ai-integration.md` and `docs/ru/13-ai-integration.md` describe only 21. The quickstarts link to chapter 13 as an MCP setup guide, but that chapter currently documents prompt generation without actual MCP setup instructions.

**Scope:**

- Update the English and Russian anti-pattern counts and lists to match `internal/output/ai_prompt.go` (P1–P27).
- Add the actual MCP stdio configuration and all four tools to both versions of chapter 13, using `internal/mcp/server.go` as the source of truth.
- Keep quickstart links pointed at the expanded chapter; do not add a new chapter or renumber existing documentation in this task.
- Check related examples and descriptive counts against the implementation rather than copying stale values.

**Done when:** EN/RU anti-pattern documentation matches the code, both MCP setup links lead to working configuration instructions, and all four MCP tools are documented consistently.

## Delivery notes

- Keep changes scoped and use Conventional Commits.
- Run `GOTOOLCHAIN=go1.27.4 make check` and `make build` for Go changes.
- Do not claim privileged integration or validation suites passed unless they were explicitly approved and actually run.
- Remove generated binaries, `dist/`, reports, and temporary files before committing.
