# Contributing to Melisa

Contributions are welcome. Please open an issue before starting significant changes so the scope and compatibility impact can be discussed first.

## Development environment

- Go 1.27.0 is the minimum language version declared in `go.mod`.
- Use Go 1.27.4 for the complete check suite. This matches CI and includes current standard-library security fixes.
- Ordinary unit tests and builds do not require root, BCC, eBPF support, Docker, or access to the host procfs/sysfs.
- Native eBPF generation requires Linux kernel BTF, clang/llvm, bpftool, and libbpf headers. A clean checkout must still build without generated BPF objects.

Create a topic branch from `main` and keep each change scoped to one task.

## Checks

Run focused tests while iterating:

```bash
go test ./internal/collector ./internal/executor
go test -race ./internal/mcp
```

Before delivery, run the complete suite and build:

```bash
GOTOOLCHAIN=go1.27.4 make check
make build
```

`make check` covers formatting, module tidiness, race-enabled tests, vet, golangci-lint, and govulncheck. Do not replace it with focused tests in the final verification.

Remove binaries, `dist/`, reports, and other artifacts created by local checks before committing.

## Project constraints

- Melisa produces point-in-time diagnostic reports; it is not a daemon or a continuous monitoring system.
- Collection must remain read-only. Never execute recommended tuning commands during collection or tests.
- Missing permissions, files, tools, BCC support, BTF, or kernel features must degrade gracefully rather than crash the entire report.
- Preserve the two-phase collection order, context cancellation, process cleanup, and partial-report behavior.
- Treat the JSON report as a public interface. Keep schema changes backward-compatible unless explicitly requested, and update the version, tests, examples, and documentation together.
- Prefer the standard library and existing dependencies. Discuss a new direct dependency before adding it.
- Pass external executable names and argument arrays directly; never construct a shell command for collector execution.

## Tests and fixtures

Add or update tests for behavior changes and regression tests for bug fixes. Use temporary directories, fake command runners, `ProcRoot`/`SysRoot`, and captured fixtures so unit tests remain unprivileged and independent of the host.

Place reusable captured inputs in `testdata/`. Cover malformed or truncated input, unavailable collectors, cancellation, and deterministic output where relevant. Do not delete or weaken tests to obtain a passing result.

The following suites have significant host side effects and are not part of ordinary development checks:

- `make test-integration` pulls images and starts privileged Docker containers with the host PID namespace. Run it only when explicitly approved.
- `make test-validation` requires root, installs host packages, creates resource-intensive workloads, changes OOM settings, and kills workload/Melisa processes. Run it only with explicit approval in a disposable Linux environment.

## Documentation

For user-visible changes, update `README.md` and the affected English and Russian files under `doc/en/` and `doc/ru/` in the same change. Keep examples and command output consistent with the implementation.

## Commits and pull requests

Use Conventional Commits, for example:

```text
fix(executor): reject truncated histogram headers
docs: explain the local Go toolchain requirement
test(mcp): cover the stdio protocol lifecycle
```

Keep pull requests focused, describe the behavior change and verification performed, and mention any environment-dependent checks that were not run.
