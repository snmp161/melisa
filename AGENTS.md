# AGENTS.md — Rules for LLM Agents Working in the Melisa Repository

## Core Rules

1. **Preserve the diagnostic contract.** Melisa produces point-in-time Linux performance reports through its CLI and MCP server. It is not a daemon, a continuous monitoring system, or an automatic remediation tool.
2. **Keep collection read-only.** Collectors must not change sysctls, network configuration, services, workloads, or other host state. Package installation belongs only to the explicitly invoked `melisa install` command. Recommendations are report data: never execute suggested tuning commands during collection or tests.
3. **Degrade gracefully.** Missing permissions, procfs/sysfs files, external tools, BCC support, BTF, or kernel features must not crash the entire collection. Return partial results with useful per-collector errors where the existing design allows it.
4. **Preserve observer-effect mitigation.** Tier 1 baselines run before BCC/native eBPF tools. Keep context cancellation, process cleanup, and partial-report behavior on SIGINT/SIGTERM intact.
5. **Treat the JSON report as a public interface.** The current schema version is `1.1.0`. Keep changes backward-compatible unless a schema change is explicitly requested; update schema version, tests, examples, and documentation together when it changes.
6. Keep changes scoped to the requested task. Do not fix unrelated issues or weaken tests to obtain a passing result.

## Repository Map

- `cmd/melisa/`: Cobra CLI, output handling, and MCP subcommand.
- `internal/collector/`: Tier 1 collectors, BCC adapter, and native eBPF collector.
- `internal/executor/`: BCC execution, binary security checks, tool registry, and output parsers.
- `internal/ebpf/`: native eBPF loading, BTF detection, and embedded objects.
- `internal/orchestrator/`: profiles, two-phase collection, filtering, and signal handling.
- `internal/model/`: report types, USE metrics, anomalies, health score, and recommendations.
- `internal/mcp/`: MCP stdio server and tool handlers.
- `internal/output/`: JSON, progress output, FlameGraph SVG, and AI prompts.
- `internal/diff/`: before/after report comparison.
- `internal/installer/`: explicitly invoked BCC/tool installation.
- `internal/observer/`: PID tracking and collection overhead measurement.
- `testdata/`: captured procfs/sysfs trees and tool-output fixtures.
- `tests/integration/`: manual Docker-based distribution tests and release artifact verification.
- `tests/validation/`: privileged workload and anomaly-detection validation.
- `doc/en/` and `doc/ru/`: user and implementation documentation; keep them in sync.

## Go and Dependencies

- The Go language version is pinned in `go.mod` (`1.27.0`); CI and release workflows pin the toolchain separately (`1.27.4`). Do not silently change either version.
- Prefer the standard library and existing dependencies. Discuss a new direct dependency before adding it.
- Release and cross-compilation builds use `CGO_ENABLED=0`. Do not impose that setting on race-enabled tests.
- Match existing package conventions and favor simple, direct code over new frameworks or abstraction layers.
- Propagate contexts and deadlines. External processes must be terminated and resources closed on cancellation.
- Return actionable errors with collector or tool context. Do not hide failures with empty successful results.

## External Commands and Security

- Never construct a shell command for external execution. Pass executable names and argument arrays directly.
- Use `collector.CommandRunner` for ordinary collector utilities so tests can substitute fakes.
- Run BCC tools through `executor.Executor`/`BCCExecutor`; preserve allowed-path, root-ownership, symlink, permission, environment, output-limit, and process-group signal checks.
- Treat procfs, sysfs, command output, and BCC output as untrusted input. Bound allocations and event counts, and test malformed or truncated data.
- Do not print or store secrets, report environment variables unnecessarily, or run recommendations automatically.

## Tests and Fixtures

- Add or update tests for behavior changes and regression tests for fixed bugs.
- Unit tests must run without root, BCC, eBPF, Docker, or access to the real host procfs/sysfs. Use `t.TempDir()`, `ProcRoot`/`SysRoot`, fake command runners, and fixtures.
- Put reusable captured inputs in `testdata/`. Update fixtures deliberately; do not rewrite expected output merely to match a regression.
- Do not delete, skip, or weaken existing tests to make a change pass. A skip must reflect a genuinely unavailable optional capability, not a failing assertion.
- Cover parser malformed-input cases, collector unavailability, cancellation, and deterministic report ordering where relevant.
- For user-visible behavior, update `README.md` and the affected English and Russian documentation in the same change.

## Required Checks

For ordinary Go changes, run before delivery:

```bash
GOTOOLCHAIN=go1.27.4 make check
make build
```

Use Go 1.27.4 for the full check suite to match CI and include current standard-library security fixes. `make check` runs formatting, module tidiness, race-enabled tests, vet, pinned golangci-lint, and govulncheck. Use focused `go test` commands during iteration, but do not replace the final full check with them.

For changes affecting collection or report generation, also exercise the real CLI on Linux when the environment permits:

```bash
go run ./cmd/melisa collect --profile quick --duration 2s --quiet -o -
```

Verify that the output is valid JSON and inspect per-collector errors. Missing BCC tools or kernel/BCC incompatibility on a development host is an environment limitation; distinguish it from a code regression and report it accurately.

### Native eBPF

- Do not edit generated `internal/ebpf/bpf/*.o` or `internal/ebpf/c/vmlinux.h` files by hand.
- `make generate` requires Linux kernel BTF, clang/llvm, bpftool, and libbpf headers. Generated objects and `vmlinux.h` are intentionally ignored by Git.
- A clean checkout must still build without generated objects. Preserve the BCC fallback when native eBPF cannot load.

### Privileged and Distribution Tests

- `make test-integration` pulls images and starts privileged Docker containers with the host PID namespace. It is a manual test target, not part of GitHub Actions; run it only when requested or explicitly approved.
- `make test-validation` requires root, installs host packages, creates resource-intensive workloads, changes OOM settings, and kills workload/Melisa processes. Never run it on a developer or production host without explicit approval; use a disposable Linux environment.
- Do not claim these environment-dependent suites passed when only unit tests were run.

## Git and Releases

- The default branch is `main`.
- Use Conventional Commits when a commit is requested: `type(scope): description`, with types such as `feat`, `fix`, `refactor`, `test`, `docs`, `ci`, `build`, and `chore`.
- Commit or push only when the user asks. Never rewrite published history or use `git push --force`.
- Do not commit binaries, `dist/`, diagnostic reports, generated BPF artifacts, credentials, or local tool configuration.
- Releases are created from `v*` tags, not ordinary pushes to `main`. Keep `.goreleaser.yaml`, release workflows, `LICENSE`, and `NOTICE` consistent when changing release contents.
- Keep this file, the Makefile, README, and CI workflows synchronized when commands, architecture, or development requirements change.
