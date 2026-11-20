# Project Hygiene Backlog

This file tracks the remaining hygiene tasks after completion of the first-priority developer workflow, MCP catalog, MCP stdio testing, and release snapshot work.

Each task should be implemented independently with focused verification, documentation updates where applicable, and a green full CI run before delivery.

## Priority 2

### 5. Add a machine-readable JSON Schema for reports

**Size:** M–L

**Evidence:** schema version `1.1.0` is repeated in code and documentation, but no machine-readable schema exists. The schema badge in `README.md` currently has an empty target.

**Scope:**

- Centralize the report schema version instead of repeating string literals.
- Add a versioned JSON Schema document for the public report format.
- Validate report serialization and the captured example against the schema in automated tests.
- Keep schema `1.1.0` backward-compatible; any incompatible change requires an explicit schema-version decision.
- Point the README schema badge to the schema document.
- Update affected English and Russian documentation together.

**Done when:** report fixtures and generated output are validated in ordinary unprivileged tests, the schema version has one source of truth, and the published schema link works.

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

## Priority 3

### 8. Group non-breaking Dependabot updates

**Size:** S

**Evidence:** `.github/dependabot.yml` currently creates an individual PR for every Go module and GitHub Actions update, which increases review noise.

**Scope:**

- Add separate groups for Go modules and GitHub Actions.
- Group only minor and patch updates; keep major updates separate for individual compatibility review.
- Preserve weekly schedules and existing PR limits.
- Do not enable automatic merge as part of this task.

**Done when:** Dependabot configuration is valid, future minor/patch updates use the intended groups, and major updates still arrive independently. If no eligible update is available during implementation, validate the configuration through GitHub and record that runtime grouping has not yet been observed.

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
