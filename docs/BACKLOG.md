# Project Backlog

This file tracks the remaining Melisa tasks and their acceptance criteria.

- Tasks 6, 7, and 9 are the remaining hygiene items after completion of the first-priority developer workflow, MCP catalog, MCP stdio testing, release snapshot work, report schema validation, and Dependabot update grouping.
- Tasks 10–28 come from the code review of 2026-11-27. Review environment: Ubuntu 24.04.5, kernel 7.0, bpfcc-tools 0.29.1, Go 1.27.4; `make check` passed. Only non-root CLI runs were executed. Statements about root-only behavior are marked *inferred from code* and must be confirmed during implementation. Line references are against commit `2cf5cb2`.
- Tasks 29–46 come from the review of repository structure, CI/CD, and the mix of languages on the same date and against the same commit.
- Tasks 47–55 come from the review of the GitHub repository settings on the same date. Settings that were already changed are listed under *Repository settings applied on 2026-11-27*.
- Tasks 56–66 come from the code-quality review on the same date. Function length and cyclomatic complexity were measured with a `go/ast` script over non-test files.
- Tasks 67–69 and the *Tooling decisions* section come from the evaluation of code-quality tools on the same date. Linter counts cited in tasks come from a trial run of additional linters in the pinned golangci-lint v2.15.0, without a configuration file and without deduplication by line.
- Tasks 70–84 and the *Direction for the next months* section come from an operations review (SRE, DevOps, system administration) on the same date.
- Tasks 85–93 come from a coverage review on the same date, which compared every procfs/sysfs path and external command the collectors read against a map of Linux system diagnostics.

Tasks may be delivered in batches by stage (see *Direction for the next months*). Each task in a batch still needs focused verification, documentation updates where applicable, and a green full CI run before delivery.

## Direction for the next months

Melisa's strongest position is a read-only, single-binary snapshot that an operator can run on a sick host and trust. The next months focus on that position, in three stages.

Principles:

- Stay point-in-time and read-only: no daemon, agent, continuous monitoring, or automatic remediation (AGENTS.md).
- Be trustworthy before broad. A missing or failed measurement must be visible, never an empty success.
- Be safe on a sick host. The diagnostic run must not make an incident worse.
- Be readable by a person during an incident, and parseable by automation and AI agents.
- Prefer a small core of native CO-RE eBPF programs over a long tail of BCC tools.

| Stage | Goal | Tasks |
|---|---|---|
| 1, about one month | A trustworthy snapshot | 10, 11, 12, 13, 14, 29, 56, 70, 71, 72, 85; release v0.2.0 afterwards (task 47) |
| 2, months 1–3 | Operator-grade Tier 1 and safe runs | 73, 74, 75, 76, 77, 78, 79, 80, 82, 86, 87, 88, 89, 91, and 15, 17, 18, 19, 20, 21 |
| 3, months 3–6 | Native core and integration | 30, 35, 83, 81, 84, 90, 92, 93, and 31, 32, 33 |

- Engineering tasks (CI, refactoring, code quality) run alongside, preferably in the stage that touches the same code.
- Deprioritized until stages 1 and 2 are done: the README roadmap items Web UI and AMD ROCm support. OpenTelemetry export is evaluated together with task 84.

## Priority 1 — report correctness, safe advice, and release security

### 10. Report BCC tool failures instead of empty successful results

**Size:** M

**Evidence:**

- `internal/executor/executor.go:188` returns `raw, nil` for any `*exec.ExitError`, and `internal/collector/bcc_adapter.go` never inspects `ExitCode`, `Stderr`, or `raw.Truncated`. Failure classification exists only as `log.Printf` lines on stderr.
- A non-root `melisa collect --profile quick --duration 2s` run logged failures for all four BCC tools (`oomkill` and `biolatency`: BPF compile failure; `opensnoop` and `tcpretrans`: Python exception). In the report each of them had `errors: null` and no events or histograms, which is indistinguishable from "no activity".
- `BCCCollector.Available()` only checks that the binary exists, so BCC tools are started without root or BPF capabilities.
- The 50 MB stdout cap sets `raw.Truncated`, but the flag is not copied to `Result.Truncated`.
- `ParseBiolatency` (`internal/executor/parsers.go:332`–`335`) treats a histogram parse error as an empty successful result, which `nilerr` reports. "No I/O during the window" and "malformed output" become indistinguishable.

**Scope:**

- Classify tool outcomes: clean exit, stopped by Melisa at the collection deadline (keep partial output), argument or usage error, BPF compile or load failure, permission error, Python traceback, other non-zero exit.
- Add a bounded single-line summary to `Result.Errors` for every failure class. Do not copy multi-kilobyte compiler or Python dumps into the report.
- Propagate `raw.Truncated` to `Result.Truncated`.
- In parsers, distinguish "no data" from "malformed output", and return an error for the latter. `nilerr` must pass.
- Return Tier 0 with a reason from `Available()` when the process lacks root or the required capabilities (for example `CAP_BPF` and `CAP_PERFMON`, or `CAP_SYS_ADMIN`, read from `/proc/self/status`). Apply this to both BCC and native eBPF collectors.
- Make `BCCCollector` depend on the existing `executor.Executor` interface so tests can use fakes. Test seams in the executor must leave the production security checks unchanged.
- Use only existing schema fields (`errors`, `truncated`). A per-collector status field is listed under *Schema change candidates*.
- Update the README troubleshooting table ("Empty histogram data — Normal") and the affected EN/RU documentation.

**Done when:** unit tests cover each outcome class, truncation, and the deadline stop without root. A real non-root CLI run shows BCC collectors as skipped with a reason or failed with an error, never as empty successes.

### 11. Fix BCC tool arguments and add an argv contract check

**Size:** M

**Evidence:** the generated arguments were compared with `--help` output of bpfcc-tools 0.29.1 (Ubuntu 24.04). These invocations are rejected or mean something else:

| Tool | Generated args | Problem |
|---|---|---|
| `biosnoop` | `-d 30` | `-d` is `--disk`; the tool exits because disk "30" does not exist |
| `execsnoop`, `statsnoop` | `-d 30` | no `-d` option |
| `tcprtt` | `30` | no positional arguments; duration is `-d` |
| `ext4slower`, `xfsslower`, `nfsslower`, `zfsslower`, `btrfsslower`, `fileslower` | `1 30` | only one positional argument (`min_ms`) |
| `runqslower` | `10000 30` | only one positional argument (`min_us`) |
| `llcstat` | `30 1` | only one positional argument (`duration`) |
| `tcpsynbl` | `30 1` | no positional arguments |
| `stackcount` | `-fK 30` | `30` is parsed as the trace pattern; duration is `-D` |
| `wakeuptime` | `-fK 30` | no `-K` option |
| `memleak` | `-a 30` | no count, so it runs until killed; `-a` lists every outstanding allocation |

Not checked (not installed on the review host, or not inspected): `bitesize`, `biostacks`, `sockstat`, `scsilatency`, `nvmelatency`, `btrfsdist`, `nfsdist`, `zfsdist`, and the event tools started without arguments. Parser tests use fixtures and never exercise `BuildArgs`, and task 10 hides the resulting failures.

**Scope:**

- Correct `BuildArgs` for every listed tool. Choose options that exist across the documented distribution matrix; where versions differ, document the minimum BCC version or detect it.
- Make `memleak` exit on its own (interval and count) and reconsider the output volume of `-a`.
- Add a table-driven unit test that asserts the exact argv of every `Registry` entry, with and without a PID filter.
- Add a non-privileged check script (for example under `tests/integration/`) that runs each installed tool with the generated arguments as a non-root user. It fails on argument errors (argparse `usage:`, `unrecognized arguments`, `error: argument`, or tool-specific checks such as `no such disk`). Without root, valid arguments fail later at BPF load, so the check stays read-only.

**Done when:** every registry entry has an argv assertion, and the check script passes on each target of the approved manual distribution run. An approved root validation run shows data from the corrected tools.

### 12. Apply the effective duration and validate profile and focus input

**Size:** S–M

**Evidence:**

- `internal/orchestrator/orchestrator.go:55` sets the run timeout from `profile.Duration + 30s`, and `:316` writes `metadata.duration` from the profile. Both ignore `cfg.Duration`. A run with `--duration 2s` on the quick profile reported `"duration": "10s"`; `--duration 5m` on quick would be cancelled after 40 s.
- `internal/orchestrator/profiles.go:65` falls back to standard for unknown names. `--profile bogus` ran the full standard set and labeled the report `"profile": "bogus"`. The MCP handler relies only on the advisory schema enum.
- Unknown `--focus` values are used as raw categories and silently filter out every Tier 2/3 collector.
- `ProfileConfig.FocusDuration` and `GetDuration` are never used.

**Scope:**

- Use the effective duration for the orchestrator timeout, collector deadlines, and `metadata.duration`.
- Reject unknown profiles and focus values in the CLI and in MCP with an error that lists the valid names. Reject non-positive durations.
- Implement focus-specific durations or remove `FocusDuration` and `GetDuration`.
- Update README and the EN/RU documentation where they describe duration and profiles.

**Done when:** tests cover the duration override (timeout and metadata), unknown profile and focus errors in the CLI and MCP, and the chosen `FocusDuration` behavior.

### 13. Repair the native eBPF tcpretrans collector

**Size:** L

**Evidence** (root behavior inferred from code; not executed during the review):

- `internal/collector/ebpf_tcpretrans.go:115`: the read loop ends only on context cancellation or at `MaxEventsPerCollector`. The context is the orchestrator's (`Duration + 30s`), so whenever the native collector is selected, the run lasts about 30 s longer than the profile.
- `internal/ebpf/c/tcpretrans.bpf.c:50` drops IPv6 events. This is a regression compared with the BCC `tcpretrans` it replaces.
- `Makefile:28` compiles with `-D__TARGET_ARCH_x86` and x86 include paths. The same object is embedded in the arm64 release, where `BPF_KPROBE` reads arguments using the x86 `pt_regs` layout.
- When `TryLoad` fails at collection time, there is no fallback to BCC. AGENTS.md requires preserving the BCC fallback. `Available()` does not check privileges.
- The disk fallback path `internal/ebpf/bpf/tcpretrans.o` is relative to the working directory.
- The two implementations emit different events under the same collector name. The BCC parser produces `laddr:lport`, `raddr:rport`, and a textual `state` (`ESTABLISHED`); the native collector produces `laddr`, `daddr`, `lport`, `dport`, and a numeric `state`.
- Perf lost samples are ignored.

**Scope:**

- Stop collection at the effective duration.
- Prefer the stable `tcp:tcp_retransmit_skb` tracepoint (IPv4 and IPv6 addresses, ports, state) over the kprobe.
- Build per-architecture objects for amd64 and arm64 and select one by `runtime.GOARCH`. Task 30 owns the build mechanics (`bpf2go`, pinned inputs, CI jobs).
- Fall back to the BCC tool when native loading or attaching fails, and record the native error in `Result.Errors`.
- Align the event `details` keys and the state representation with the BCC parser. Report lost samples.
- Remove the working-directory-relative object path or make it explicit. Keep generated objects out of Git, and keep clean-checkout builds working.

**Done when:** unit tests cover event decoding (IPv4 and IPv6), the duration stop through a reader seam, and fallback selection. CI builds objects for both architectures. An approved manual root run on x86_64 and arm64 shows IPv4 and IPv6 events with a runtime close to the requested duration.

### 14. Remove unconditional and boot-cumulative recommendations

**Size:** S–M

**Evidence:**

- `internal/model/recommendations.go:97` recommends `vm.overcommit_memory=2` whenever the kernel default `0` is set, which is nearly every host. Strict overcommit can make fork-heavy or large-virtual-memory workloads (JVMs, Redis background saves) fail with `ENOMEM`.
- "fix" recommendations are triggered by counters that accumulate since boot: `ListenOverflows` (`:348`), softnet `TimeSqueeze` (`:478`), and `PruneCalled` (`:608`). The README says rate-based rules exist to separate current activity from cumulative counters.
- An idle desktop received 11 recommendations, among them:
  - "TCP memory pressure detected", from `PruneCalled=15832` accumulated since boot;
  - `netdev_budget` "time_squeeze detected";
  - ARP table tuning "for container/K8s environments".
- `somaxconn` is recommended as 4096 (`:239`) and as 65535 (`:355`) in the same report.
- Persistent commands append to `/etc/sysctl.d/99-melisa.conf` and create duplicates when repeated.

**Scope:**

- Remove the overcommit recommendation, or make it informational without a command and with explicit risk text.
- Base "fix" recommendations on rates measured during the collection window. `ListenOverflowRate` already exists; `TimeSqueeze` and `PruneCalled` have no rate field in schema 1.1.0 (see *Schema change candidates*).
- Separate baseline tuning that is triggered only by kernel defaults from evidence-backed fixes. Gate it on workload evidence where available.
- Remove conflicting duplicates. Make persistent commands idempotent, or state that they append.
- Update `docs/en/12-recommendations.md`, `docs/ru/12-recommendations.md`, and README.

**Done when:** tests show three things. An idle-host fixture with kernel defaults and zero deltas produces no "fix" recommendations. Overcommit is not recommended without evidence. No report contains conflicting values for the same sysctl.

### 29. Separate release verification from publishing permissions

**Size:** S

**Evidence:** `.github/workflows/release.yml` runs every step in one job with `permissions: contents: write`. `actions/checkout` keeps the default `persist-credentials: true`, so a write-scoped `GITHUB_TOKEN` stays in `.git/config` while untrusted code runs:

- `make generate`;
- `make check`: the tests, golangci-lint and govulncheck started through `go run`, and downloaded module code;
- the snapshot build.

Only the final GoReleaser step needs write access.

**Scope:**

- Split the workflow into two jobs:
  - `verify`, with `contents: read`: generate, `make check`, GoReleaser check, snapshot, artifact verification;
  - `release`, with `contents: write` and `needs: verify`.
- Set `persist-credentials: false` on every checkout that does not push.
- Keep the tag-only trigger, Go `1.27.4`, and GoReleaser `v2.20.2`. Coordinate with task 7 (SHA pinning) and task 33, which grants `id-token: write` to the publishing job only.

**Done when:** only the publishing step can reach a write-scoped token, the CI release snapshot still passes, and a release from a test tag in a fork or disposable repository still publishes every artifact.

### 56. Reject non-finite numbers in parsed tool output and fuzz all parsers

**Size:** S–M

**Evidence:**

- `ParseTabularEvents` (`internal/executor/parsers.go:263`) stores every field that `strconv.ParseFloat` accepts as a number in `Event.Details`.
- `ParseFloat` accepts `nan`, `inf`, and `Infinity`. `encoding/json` refuses to encode `NaN` or `±Inf` (`json: unsupported value: NaN`). Both behaviors were verified on Go 1.27.
- A single such value therefore makes the whole report fail to serialize: `WriteJSON` in the CLI, `json.Marshal` in MCP.
- Untrusted columns that can carry such a word include `FILE` in `filelife` (the file name), `HOST` in `gethostlatency`, and `PATH` in `opensnoop` (the path as passed to `open`). An unprivileged local user can run `touch nan; rm nan` during a collection and break the report. Each step was verified separately; the end-to-end run needs root and was not executed.
- The repository has no fuzz tests (`func Fuzz…`), although AGENTS.md requires malformed-input coverage for parsers of untrusted input.

**Scope:**

- Never produce non-finite floats from parsed input: keep such values as strings, or reject them. Audit every parser in `internal/executor/parsers.go` and `internal/collector` for the same pattern.
- Add native Go fuzz tests for each parser. Assert no panics, bounded output, and that every result passes `json.Marshal`. Seed the corpora from `testdata/`.
- Add a regression test with `nan`, `inf`, and `Infinity` in path, file, and host columns.

**Done when:** a report containing these words serializes successfully, fuzz targets exist for every parser and run for a short time in `make test` or a dedicated target, and the regression test passes.

### 70. Redact sensitive data and protect report files

**Size:** M

**Evidence:**

- Reports can carry sensitive strings:
  - command-line arguments from `execsnoop` (`ARGS`);
  - file paths from `opensnoop` and `filelife`;
  - hostnames from `gethostlatency`, peer addresses from `tcpconnect` and `tcplife`;
  - `dmesg` lines and kernel boot parameters.

  Command lines often contain credentials (`mysql -p…`, `--password=`, tokens in URLs). Once task 11 fixes the `execsnoop` arguments, these arguments start appearing in reports.
- There is no redaction option, and `SECURITY.md` asks users to redact reports by hand.
- `internal/output/json.go:17` creates reports with `os.Create` (mode 0666 before umask, typically 0644). `melisa diff -o` writes with mode 0644 (`cmd/melisa/main.go:282`).
- `os.Create` follows symlinks, and README examples write root-collected reports to `/tmp/report.json`. A local user who creates `/tmp/report.json` as a symlink in advance can make a root run overwrite any file (inferred from code; not reproduced).
- MCP returns the full report to the agent, which forwards it to its LLM provider (tasks 21 and 22).

**Scope:**

- Redact by default:
  - keep the executable name but mask arguments that match secret patterns (`password=`, `token`, `secret`, `-p<value>`, `Authorization:`, credentials in URLs);
  - offer levels, for example `--redact=default|strict|none`, where `strict` also masks IP addresses, hostnames, and paths;
  - use the strict level by default in MCP.
- Write report and diff files atomically: create a temporary file with mode 0600 in the target directory, then rename it over the target, so a pre-existing symlink is never followed.
- Record the redaction level in the report (see *Schema change candidates*).
- Land this task before task 11 or together with it.
- Update README, SECURITY.md, and the EN/RU documentation, and change the examples away from `/tmp`.

**Done when:** tests show masking in every listed field; files are created with mode 0600 and the write does not follow a pre-existing symlink; MCP uses the strict level.

### 85. Read sysctl-based values from paths that exist

**Size:** S–M

**Evidence:** of the 45 procfs paths that the collectors read through `readSysctl*`, six do not exist on the review host (kernel 7.0). `readSysctl*` returns 0 for a missing file, and the affected fields use `omitempty`, so they are silently 0 or absent from every report.

- **Conntrack drop counters.** `internal/collector/network.go:389`–`391` read `nf_conntrack_drop`, `nf_conntrack_insert_failed`, and `nf_conntrack_early_drop` from `/proc/sys/net/netfilter/`. These are not sysctls: the kernel exposes the counters per CPU in `/proc/net/stat/nf_conntrack`, which requires `CONFIG_NF_CONNTRACK_PROCFS` and is absent on the review host, or over netlink (`conntrack -S`). `insert_failed` is a classic cause of packet loss under SNAT on Kubernetes nodes.
- **NUMA balancing.**
  - `internal/collector/memory.go:73` reads `kernel.sched_numa_balancing`; the real sysctl is `kernel.numa_balancing`, which exists on the review host.
  - `internal/model/recommendations.go:161` and `:164` suggest `sysctl -w kernel.sched_numa_balancing=1` and a persistent line with the same wrong name whenever a node's NUMA miss ratio exceeds 5%. The command fails, and the persistent line causes an error each time sysctl settings are applied (inferred).
  - The MCP explanation (`internal/mcp/handlers.go:689`, `:692`) uses the same wrong name.
- **CFS tunables.** `kernel.sched_latency_ns` and `kernel.sched_min_granularity_ns` are absent on current kernels (task 27).

**Scope:**

- Sum the per-CPU conntrack counters from `/proc/net/stat/nf_conntrack` when the file is present. Otherwise use `conntrack -S` through `CommandRunner` when it is installed. A native netlink client would add a dependency and needs discussion per AGENTS.md.
- Compute conntrack rates over the collection window, and add an anomaly rule for `insert_failed` and `drop`.
- Read `kernel.numa_balancing`, and fix the recommendation and the MCP text. Recommend enabling balancing only when it is actually disabled.
- When a value is unavailable, record that in the collector's `errors` instead of reporting 0.
- Add a test that checks every `readSysctl*` path against a list of known sysctl names, so a misspelled or nonexistent path fails CI.

**Done when:**

- fixture tests cover the conntrack stat file, fake `conntrack -S` output, and the unavailable case;
- the NUMA recommendation uses the real sysctl and depends on its value;
- the path test passes;
- a real run on the review host reports unavailable counters as unavailable.

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

### 15. Measure observer overhead and track child PIDs while they run

**Size:** M

**Evidence:**

- `internal/collector/bcc_adapter.go:97` registers the tool PID only after `executor.Run` has returned, when the process has already exited, and removes it when `Collect` returns. As a result, `observer_overhead.child_pids` is always empty and child CPU and memory are never counted.
- `/usr/bin/time` for a non-root quick run (four BCC tools) measured 27.7 s CPU (20.1 s user + 7.6 s system) and 228 MB peak RSS. The report showed `cpu_user_ms + cpu_system_ms = 150`.
- Context switches are read from `/proc/self/status` for the main thread only; the report showed 0. RSS assumes 4 KiB pages.
- Cross-tool self-event filtering through `IsOwnPID` depends on timing, because a PID is registered only between process exit and the end of its own `Collect`.

**Scope:**

- Register each child PID immediately after `Start` and keep it until Phase 2 ends.
- Account child CPU and peak RSS from `ProcessState.SysUsage()` (rusage) or `getrusage(RUSAGE_CHILDREN)`. Account self usage with `getrusage(RUSAGE_SELF)`. Use `os.Getpagesize()`.
- Keep the existing `observer_overhead` fields and document what they include.
- Update README and the affected EN/RU documentation.

**Done when:** a unit test with a helper child process (no root) verifies child CPU accounting and PID registration during execution. A real non-root run reports overhead of the same order of magnitude as `/usr/bin/time`.

### 16. Make `melisa diff` compare JSON-loaded reports deterministically

**Size:** M

**Evidence:**

- `internal/diff/diff.go:185`, and the equivalent memory helper, type-assert `Result.Data` to `*model.CPUData` or `*model.MemoryData`. `LoadReport` decodes `Data` as `map[string]interface{}`, so for real files these details are never compared:
  - CPU: `user_pct`, `iowait_pct`, `load_avg_1`, `context_switches_per_sec`;
  - memory: `major_faults`.

  Tests pass because they build typed reports in memory.
- Output order depends on map iteration. Five runs over the same two files produced three different outputs.
- Reports from different hosts, profiles, or schema versions are compared without a warning.
- `docs/en/14-report-diffing.md`, `docs/en/00-introduction.md`, `docs/en/17-appendix.md`, and their RU counterparts document a `--json` flag that does not exist. JSON is written only with `-o <file>`.

**Scope:**

- Decode `Result.Data` into typed structs by collector (for example with `Result.UnmarshalJSON`) without changing the JSON shape.
- Sort changes deterministically.
- Warn on a host, profile, or schema-version mismatch.
- Implement `--json` for stdout or remove it from the EN/RU documentation.

**Done when:** a test loads files based on `testdata/report_1.1.0.json` through `LoadReport` and asserts CPU and memory changes. Repeated runs are byte-identical. Mismatch warnings are tested.

### 17. Normalize USE saturation and fix scoring false positives

**Size:** M

**Evidence:**

- `internal/model/report.go:149` defines network saturation as the number of TIME_WAIT plus CLOSE_WAIT sockets. Busy servers routinely hold thousands of TIME_WAIT sockets, which costs 15 points under the shared `> 50` threshold.
- `report.go:80` computes `container_cpu` utilization by dividing throttled time accumulated since container start by the quota ratio. Saturation is the cumulative `nr_throttled` count.
- Disk saturation is the instantaneous in-flight I/O count, but it is evaluated with the same thresholds as percentage metrics.
- `internal/model/anomaly.go:633`: when the minimum per-CPU `NET_RX` delta is 0, `irq_imbalance` returns the absolute maximum delta instead of a ratio. An idle desktop reported "critical 173x".

**Scope:**

- Define every saturation metric in consistent units (percentage or normalized ratio):
  - disk: average queue size from weighted I/O time per interval, relative to device queue capacity;
  - network: rate-based signals (drops, overflows, retransmits) instead of TIME_WAIT.
- Compute container throttling over the collection window from two-point `nr_throttled` and `nr_periods` samples.
- Make `irq_imbalance` always return a ratio. Require a minimum per-CPU traffic level, and skip single-queue setups and CPUs that do not serve NIC interrupts.
- Update the README health score table and `docs/en/11-anomaly-detection.md` / `docs/ru/11-anomaly-detection.md`.

**Done when:** table-driven tests show no penalties or anomalies for four fixtures: a busy server with many TIME_WAIT sockets, an unthrottled container, a deep NVMe queue, and a low-traffic single-queue NIC. They also include a true positive for each metric.

### 18. Report Tier 1 read failures instead of zero values

**Size:** M

**Evidence:**

- `internal/collector/cpu.go:161` returns an empty `CPUData` when the total delta is zero, which also happens when `/proc/stat` cannot be read. The resulting `IdlePct = 0` produces 100% CPU utilization, a critical `cpu_utilization` anomaly, and a large health-score deduction.
- `readProcStat`, `readDiskStats`, and similar helpers return zero values or `nil` on errors, and `Collect` reports success without `errors`. AGENTS.md forbids hiding failures behind empty successful results.
- The first CodeQL run (2026-11-27) opened three `go/incorrect-integer-conversion` alerts: `internal/collector/disk.go:81`, `internal/collector/memory.go:132`, and `internal/collector/memory.go:134`.
  - The memory conversions are harmless on the 64-bit release targets.
  - The disk deltas at `disk.go:77`–`83` subtract `uint64` counters without checking that they did not decrease. A counter reset or a replaced device therefore produces huge values.
- `gosec` rule G115 reports 14 unchecked integer conversions in non-test code, the same class as the CodeQL alerts.
- `GPUCollector.Collect` returns `nil, nil` when nothing is detected (`internal/collector/gpu.go:50`, reported by `nilnil`), so the collector silently disappears from the report.

**Scope:**

- Return an error, or add a `Result.Errors` entry, when a collector's primary source is unreadable or malformed.
- Treat zero-delta or missing samples as "no data". Make USE metrics and anomaly evaluators skip missing data.
- Handle counter decreases and out-of-range values without unsigned underflow or unchecked narrowing, and resolve CodeQL alerts #1–#3.
- Test missing, unreadable, truncated, and static (zero-delta) inputs with `ProcRoot`/`SysRoot` in `t.TempDir()`.

**Done when:** no Tier 1 collector reports success with metrics derived from missing data, the tests above pass without root, and CodeQL, `gosec` G115, and `nilnil` report nothing for these files.

### 19. Bound the cost of a collection run

**Size:** M–L

**Evidence:**

- The standard profile registers every `Registry` entry, and Phase 2 starts every available tool at once (`runCollectorsParallel`). Each BCC tool is a Python process that compiles BPF with clang. A non-root quick run with four tools cost 27.7 s CPU and 228 MB peak RSS (task 15). The cost of about 60 tools on a root host was not measured.
- Expensive tracers run in the standard profile: `memleak` (kernel allocation tracing), `funccount tcp_*` (kprobes on every matching function), `stackcount`, and `profile -a`.
- The deep profile's `Extra` list (`memleak`, `offwaketime`, `biostacks`, `wakeuptime`, `biotop`, `tcpstates`, `tcplife`) has no effect, because standard already registers every registry tool. The README still describes deep as "Standard set plus memleak, biostacks, wakeuptime, biotop".
- Tool deadlines are `duration + 5s` (`bcc_adapter.go`). Under parallel compilation, startup may take longer than this grace period (inferred; not measured).

**Scope:**

- Limit concurrent BCC startup with a semaphore and a documented default; consider a CLI flag.
- Add cost classes to `ToolSpec`. Keep expensive tracers out of the standard profile and make the deep profile actually different.
- Skip expensive tools, with a recorded reason, when `MemAvailable` is low.
- Revisit the startup grace period so that slow compilation is not cut off at the deadline.
- Update the README profile table and the EN/RU documentation.

**Done when:** unit tests cover profile composition and the concurrency limit. An approved manual root validation run records wall time, CPU time, and peak memory for each profile.

### 20. Prevent hangs and orphaned processes

**Size:** M

**Evidence** (inferred from code; not reproduced):

- `internal/collector/system.go:104` runs `df -P -T`, which can block in uninterruptible sleep on an unresponsive NFS mount. `exec.CommandContext` cannot kill a process in that state, and `Cmd.WaitDelay` is not set, so Phase 1 and the whole report can hang.
- `ExecCommandRunner.Run` reads unbounded output, for example `ss -tn state established` on a host with many connections. AGENTS.md requires bounded allocations for command output.
- BCC tools run in their own process group, and 25 event tools get no duration argument. If Melisa exits without cleanup, those tools keep running with their probes attached. This can happen on SIGKILL, on an OOM kill, or when the MCP server returns from `Start` on SIGTERM while a collection is in flight.
- NIC enrichment runs five `ethtool` commands per interface sequentially and skips only `lo`, `veth*`, `docker*`, and `br-*`. Kubernetes nodes with `cali*`, `lxc*`, or `tap*` interfaces spawn hundreds of processes.

**Scope:**

- Replace `df` with `statfs` over `/proc/self/mountinfo` with per-mount timeouts, or skip network filesystems. Set `WaitDelay` for every external command.
- Cap `CommandRunner` output and record truncation.
- Make sure children terminate when Melisa dies, for example with `Pdeathsig` (mind the OS-thread caveat) or explicit tracking and kill on shutdown. Make the MCP server wait for in-flight collections to clean up before exiting.
- Run `ethtool` only for interfaces that have `/sys/class/net/<if>/device`, with a per-interface timeout.

**Done when:**

- tests with fake runners that block or overflow pass without hanging;
- a test confirms that cancellation kills the child process group;
- a manual check shows no orphaned BCC processes after SIGTERM to `melisa mcp` during a collection.

### 21. Harden the MCP server

**Size:** M

**Evidence:**

- `collect_metrics` returns the whole report (events, stacks, histograms, AI prompt) as one text block. A non-root quick report is 26 KB; root standard and deep reports can be far larger than an agent's useful context.
- The mcp-go stdio transport dispatches tool calls through a worker pool (five workers by default), and nothing serializes collections. Two concurrent `collect_metrics` calls start two full BCC sets.
- Arguments are inconsistent with the CLI or not validated:
  - `focus` accepts one value, so `"disk,network"` matches nothing, while the CLI splits on commas;
  - `pid` is not validated, so zero or negative values reach the tools;
  - `profile` is validated only by the advisory enum.
- `get_health` has three problems:
  - it intends to include `container_info`, but the quick profile filter removes it, together with `gpu_pcie`;
  - its `load_avg` field actually contains the CPU saturation percentage;
  - its description promises "~1s", although it also runs `ss`, `ethtool`, and `dmesg`.

**Scope:**

- Return a summary first (health, anomalies, recommendations, per-collector errors), with arguments that request detailed sections or cap the number of events.
- Allow one collection at a time and return a clear busy error otherwise.
- Parse `focus` the same way as the CLI. Validate `pid` and `profile`.
- Select the `get_health` collectors explicitly. Return the real load average in `load_avg` and saturation under its own name. Fix the description.
- Coordinate the documentation with task 9.

**Done when:** handler tests cover summary and detail modes, the busy error, argument validation, and the `get_health` collector set and fields. EN/RU chapter 13 documents the behavior.

### 22. Make AI context and scoping claims accurate

**Size:** S–M

**Evidence:**

- `internal/output/ai_prompt.go:71` states that latency histograms from runqlat, biolatency, and tcpconnlat reflect only the target PID. `biolatency` has no PID filter.
- The prompt states that Tier 1 baselines "are accurate". Phase 1 reduces observer effect but does not remove it.
- `--cgroup` filters only the process and container collectors. BCC tools stay host-wide, although many of them support `--cgroupmap`. Results do not say whether a PID or cgroup filter was applied.
- Untrusted strings (process names, file paths, hostnames, `dmesg` lines) reach the LLM without being marked as data. This is a prompt-injection vector for MCP agents.

**Scope:**

- Generate the scoping statements from the registry (`PIDFlag`) instead of hard-coded text, and name the host-wide tools explicitly.
- Replace the absolute accuracy claims with the actual guarantees.
- Add an explicit notice that report contents are untrusted data, not instructions.
- Evaluate cgroup filtering for BCC tools as a follow-up.
- Update EN/RU chapter 13 (coordinate with task 9).

**Done when:** tests assert that, for a PID-scoped report, the prompt lists exactly the PID-filtered tools and contains the untrusted-data notice.

### 23. Fix installer package selection and failure reporting

**Size:** M

**Evidence** (`internal/installer/installer.go`, which has no tests):

- `pacman -Sy` followed by `pacman -S` is a partial upgrade, which Arch does not support.
- openSUSE and SLES are detected as `zypper`, but no step defines zypper packages. Nothing is installed, and the command still prints "Installation complete". The openSUSE `ID` values (`opensuse-leap`, `opensuse-tumbleweed`) are rejected as unsupported, and `ID_LIKE` is ignored.
- Debian gets `linux-headers-amd64` on every architecture (`:166`), although arm64 packages are released.
- `kernel-devel` on yum/dnf (`:185`) is not pinned to the running kernel (`kernel-devel-$(uname -r)`), so BCC may compile against the wrong headers.
- Package failures are printed as warnings, and the command exits 0.
- It installs bpftrace, perf, and git, and clones FlameGraph into `/opt` as root, but Melisa never uses any of them.
- All 12 external commands in the installer use `exec.Command` without a context, which `noctx` reports. An installation therefore cannot be cancelled or time-limited.

**Scope:**

- Fix package selection per distribution and architecture, handle `ID_LIKE`, and remove the partial upgrade.
- Run the package manager commands with a context and timeouts.
- Exit with a non-zero status when required packages fail, and print a summary.
- Install only what Melisa uses, or make the extras an explicit option.
- Make package selection testable by injecting the kernel version and architecture.
- Update README and the EN/RU documentation.

**Done when:** unit tests cover package selection for every supported distribution and architecture, and the exit status. The `--dry-run` output matches. The approved manual distribution run records `melisa install` results.

### 24. Cover collection paths with fixture-based tests

**Size:** M

**Evidence:**

- Package coverage: `cmd/melisa` 0%, `internal/ebpf` 17.7%, `internal/mcp` 40.3%, `internal/collector` 51.2%.
- Functions with 0% coverage:
  - `Collect` of the CPU, disk, network, system, and container collectors;
  - every `BCCCollector` method;
  - `BCCExecutor.Run`, including the SIGINT→SIGKILL escalation;
  - the native tcpretrans collector;
  - the installer.
- `NewNetworkCollector` hard-codes `/sys` (`internal/collector/network.go:25`), so `RegisterCollectors` ignores `cfg.SysRoot` for the network collector.

**Scope:**

- Add fixture-based `Collect` tests for every Tier 1 collector using `ProcRoot`/`SysRoot` and fake runners.
- Test `BCCExecutor.Run` with a helper process, keeping the production security defaults unchanged.
- Pass `SysRoot` to the network collector.
- Cover whatever tasks 10, 15, 18, and 20 do not already cover.

**Done when:** every function listed above is covered, including error and cancellation paths, without root, BCC, eBPF, or host procfs/sysfs.

### 30. Adopt bpf2go and pin BPF build inputs

**Size:** M–L

**Evidence:**

- `Makefile:28` invokes clang by hand with `-D__TARGET_ARCH_x86` and `-I/usr/include/x86_64-linux-gnu`. One object is embedded in both the amd64 and arm64 releases (task 13).
- `make generate` dumps `vmlinux.h` from `/sys/kernel/btf/vmlinux` of the build host. In CI and release, that host is a GitHub runner on the floating `ubuntu-latest` image, so the released object depends on whichever kernel and clang the image had that day.
- The event layout is mirrored by hand in Go. `internal/collector/ebpf_tcpretrans.go` declares `TcpretransEvent`, decodes it with manual `binary.LittleEndian` offsets 0–40, and hard-codes `tcpretransEventMinSize = 28`. `internal/ebpf/loader.go` looks programs up by section name with fallbacks.
- Generated objects are ignored by Git, so `go install github.com/snmp161/melisa/cmd/melisa@latest` silently produces a binary without native eBPF.

**Scope:**

- Generate objects and Go bindings for amd64 and arm64 with `bpf2go`, which is part of the existing `cilium/ebpf` module. Include the event type (`-type`), and use the generated loaders instead of section-name lookup.
- Pin every build input:
  - a per-architecture `vmlinux.h` from a pinned source, verified by checksum (for example `libbpf/vmlinux.h`);
  - a pinned clang/LLVM version;
  - pinned runner images for the BPF jobs.
- Decide whether generated objects and bindings are committed.
  - If they are, update AGENTS.md, `.gitignore`, the Makefile, and CI together, and add a CI check that regeneration yields identical files.
  - If they stay ignored, document that `go install` builds lack native eBPF.
- Keep clean-checkout builds and the BCC fallback working.

**Done when:** CI regenerates objects for both architectures from pinned inputs with identical results across runs. Go code uses the generated types and loaders. The chosen artifact policy is documented in AGENTS.md, README, and CONTRIBUTING.md.

### 31. Verify release contents beyond file names

**Size:** S

**Evidence:** `tests/integration/verify_release_artifacts.sh` checks binary names, targets, executability, and package file lists. It does not check:

- that the BPF object is embedded, which is the reason the BPF toolchain is installed in three jobs;
- that the binary starts at all;
- `checksums.txt`.

**Scope:**

- Make the embedded native programs observable, for example in `melisa capabilities` or in verbose version output. Assert that both architecture binaries contain them, for example by searching each binary for a known object section name.
- Run the amd64 binary (`--version`, `capabilities`). Run the arm64 binary on an arm64 runner (task 32).
- Verify `checksums.txt` against the artifacts.
- Check the package metadata added in task 44.

**Done when:** the snapshot job fails if either binary lacks the BPF object, a binary does not start, or a checksum does not match.

### 32. Build and test on arm64 in CI

**Size:** S–M

**Evidence:** releases ship `melisa-linux-arm64` and arm64 deb/rpm packages, but every CI job runs on amd64 (`ubuntu-latest`). Architecture-specific defects are therefore invisible to CI: the BPF target in task 13, the 4 KiB page assumption in task 15, the Debian kernel headers in task 23.

**Scope:**

- Add a native arm64 runner (for example `ubuntu-24.04-arm`; check its availability for the repository plan) to the test job: build, race-enabled unit tests, vet.
- Add it to the BPF smoke build once task 30 provides arm64 objects.
- Keep the job names that branch protection relies on, or update the protection rules.

**Done when:** pull requests run build and unit tests on amd64 and arm64, and the arm64 job is required for merging.

### 33. Sign release artifacts and publish SBOM and provenance

**Size:** M

**Evidence:** Melisa is installed as root on production hosts from GitHub Release binaries and deb/rpm packages. `.goreleaser.yaml` produces only `checksums.txt`: there are no signatures, SBOMs, or build provenance attestations.

**Scope:**

- Generate SBOMs for binaries and packages with GoReleaser `sboms` (syft).
- Sign `checksums.txt` with keyless cosign and add build provenance (for example `actions/attest-build-provenance`). Grant `id-token: write` and `attestations: write` to the publishing job only (task 29).
- Pin any new third-party Actions and tools by SHA and version (task 7).
- Document the verification commands in README and SECURITY.md.
- Signing deb/rpm packages requires managing a signing key and is a separate decision.

**Done when:** a release publishes SBOMs, a verifiable checksum signature, and provenance, and the documented verification commands succeed against a test release.

### 34. Provide one collection entry point for the CLI and MCP

**Size:** M

**Evidence:**

- `orchestrator.RegisterCollectors` (`internal/orchestrator/orchestrator.go:354`) builds every collector, the native eBPF loader, and the BCC executor by itself.
- The CLI (`cmd/melisa/main.go`) and the MCP handlers (`internal/mcp/handlers.go`) each build `CollectConfig`, resolve profiles, and attach the AI prompt separately. This divergence has already caused defects: different `focus` parsing and the wrong `get_health` collector filtering (task 21).
- `orchestrator.BuildReport` was written for this purpose, but nothing calls it.

**Scope:**

- Define one options type and one entry point (for example `orchestrator.Collect(ctx, Options)`) that validates input (task 12), selects collectors, runs them, and builds the report. Make the CLI and MCP thin adapters over it.
- Inject the collector set or factories so that tests can run the full path with fakes.
- Keep the refactor behavior-preserving, apart from the fixes owned by tasks 12 and 21.

**Done when:** the CLI and MCP share the same validation and collector-selection code, and an end-to-end test with fake collectors covers both adapters.

### 35. Narrow the BCC tool scope and order the native migration

**Size:** S (decision and documentation)

**Evidence:**

- Tier 2 depends on 67 BCC Python tools whose options and text output vary by BCC version (task 11). It also needs Python, LLVM, and often kernel headers on the target host.
- At the same time, `--help` describes Melisa as a "single Go binary" and the package description as "Single-binary".
- `docs/development/NATIVE_EBPF_MIGRATION.md` plans to port tools to native eBPF and keep BCC as a fallback, but it does not prioritize a subset.

**Scope:**

- Choose a core set, for example runqlat, biolatency, tcpretrans, tcpconnlat, profile, offcputime, oomkill, execsnoop, and cachestat.
- Decide what happens to the remaining tools: keep them as optional, move them to the deep profile, or drop them.
- Evaluate libbpf-tools (CO-RE C ports of BCC tools that need neither Python nor LLVM) as an intermediate Tier 2 backend: availability across the distribution matrix, options, and output formats.
- Decide whether user-space runtime probes belong to the core set. None is in the registry today. Examples are USDT probes for Java GC, Python, and Node, used by the BCC tools `ugc`, `ustat`, and `uthreads`.
- The installer installs `perf` and `bpftrace`, but no collector uses them (task 23).
- Record the order and the rationale in `docs/development/NATIVE_EBPF_MIGRATION.md`.
- Align README, `--help`, and the package description ("67 BCC tools", "single binary") with the decision.

**Done when:** the migration document lists the core set, the fate of the other tools, and the libbpf-tools assessment, and the user-facing descriptions match it.

### 47. Fix release notes and versioning

**Size:** S–M

**Evidence:**

- The notes of the only release, `v0.1.0`, were generated by GoReleaser and contain 5,980 characters of raw commit SHAs from the upstream melisai history, for example `Merge pull request #24 from dmitriimaksimovdevelop/...` and `... (#8)`. GitHub links these numbers to this repository's pull requests; here #8 is a Dependabot update.
- The changelog configuration in `.goreleaser.yaml` has only `sort: asc` and two exclude filters.
- `cmd/melisa/main.go` defaults `version` to `0.4.1`, inherited from upstream, so local builds report a newer version than any release.
- The release binary's build info already contains `vcs.revision`. GoReleaser also passes `-X main.commit` and `-X main.date`, but those variables do not exist (task 27).
- `main` has 12 commits that are not in any release, including report schema 1.1.0. README on `main` documents them without marking them as unreleased, and there is no CHANGELOG.

**Scope:**

- Configure the GoReleaser changelog so that notes start from Melisa's own history: `use: github`, groups by Conventional Commit type, merge commits excluded.
- Rewrite the `v0.1.0` notes with `gh release edit`: what the release contains and a link to the commit range. The maintainer chooses the wording.
- Default `version` to `dev` and read the revision from `debug.ReadBuildInfo()` when ldflags are absent. Task 27 owns the variables.
- Add a `CHANGELOG.md` with an Unreleased section, or document that GitHub Releases are the changelog. Mark unreleased features in README.
- Optionally protect the publishing job (task 29) with a `release` environment that requires approval.
- Cut the next release after the Priority 1 tasks.

**Done when:** release notes contain only Melisa changes, grouped by type and with correct links. Local builds report `dev` plus the revision. CONTRIBUTING.md documents the changelog policy.

### 48. Restrict the GitHub Actions policy

**Size:** S (after task 7)

**Evidence:** the repository Actions permissions are `allowed_actions: all` and `sha_pinning_required: false`. Any Action can run, including ones referenced by movable tags.

**Scope:**

- After task 7 pins the workflows, enable the repository setting that requires Actions to be pinned to a full-length commit SHA.
- Restrict allowed Actions to GitHub-owned ones plus an explicit list: `goreleaser/goreleaser-action` and any Actions added by tasks 33, 38, and 50.
- Check that Dependabot and CodeQL default setup keep running.

**Done when:** `gh api repos/snmp161/melisa/actions/permissions` reports `allowed_actions: selected` and `sha_pinning_required: true`, and every workflow is green.

### 49. Complete the security policy

**Size:** S

**Evidence:**

- Until 2026-11-27, private vulnerability reporting was disabled, so the reporting link in `SECURITY.md` and in the issue template's contact links did not work.
- `SECURITY.md` still lacks a response timeline, a disclosure process, a fallback contact, and a description of how fixes and advisories are published.

**Scope:**

- State best-effort acknowledgement and fix timelines for a single maintainer, and describe coordinated disclosure.
- Add a fallback contact that does not require a GitHub account.
- Describe how fixed vulnerabilities are announced (GitHub Security Advisories, CVE requests).
- Keep the "latest release only" support statement explicit.

**Done when:** `SECURITY.md` covers these points, and a non-maintainer account can open a private report through the documented link.

### 57. Parse BCC tables with per-tool column specifications

**Size:** M

**Evidence:** `ParseTabularEvents` (`internal/executor/parsers.go:193`) splits each line on whitespace and maps the fields to headers by position.

- A `COMM` containing spaces, such as the Firefox thread name "Web Content", shifts every following column. In `opensnoop`, the FD, ERR, and PATH values then belong to other columns.
- Fields beyond the header count are dropped, so paths or arguments with spaces are silently truncated.
- Integers above 2^53, such as inode numbers, lose precision because numeric fields become `float64`.
- 38 registry entries rely on this generic parser.

**Scope:**

- Describe the columns of each tabular tool: name, type, and whether the last column takes the rest of the line. Parse by that description instead of by blind whitespace position.
- Keep integers as integers.
- Prefer machine-readable output where a tool offers it, for example JSON from `biolatency -j` or the CSV mode (`-j`) of the `*slower` tools. Check the options across the distribution matrix (task 11).
- Add fixtures with spaces in `COMM` and in the last column for the affected tools.

**Done when:** the new fixtures parse into the correct columns, no field is silently truncated, and task 56's fuzz tests keep passing.

### 58. Let `--max-events` raise the event limit

**Size:** S

**Evidence:**

- The registry passes a hard-coded limit of 1000 to every event parser: `ParseTabularEvents(raw, 1000)` 38 times, plus `ParseBiosnoop`, `ParseTcpconnlat`, and `ParseExecsnoop`.
- `BCCCollector.Collect` (`internal/collector/bcc_adapter.go:130`) can only lower the limit further, despite its comment "overrides hardcoded parser limits". So `--max-events 5000` (`cmd/melisa/main.go:184`) has no effect for BCC tools.
- The native tcpretrans collector honors the flag, but with a limit of 0 it stops after the first event (`internal/collector/ebpf_tcpretrans.go:159`).

**Scope:**

- Pass `cfg.MaxEventsPerCollector` to the parsers, and define the meaning of 0 consistently across all collectors.
- Enforce an upper bound so the flag cannot cause unbounded memory use.
- Document the flag and the bound in README and the EN/RU documentation.

**Done when:** tests show that the flag raises and lowers the limit for BCC and native collectors, and that the upper bound is enforced.

### 59. Test the CLI through Cobra commands

**Size:** S–M

**Evidence:**

- `cmd/melisa/main_test.go` has ten tests, yet coverage of `cmd/melisa` is 0%: the tests re-implement CLI logic instead of executing it.
- `TestCLIDurationOverridesProfile` (`:40`) assigns `cfg.Duration = 15s` and then checks that it is 15s. This is why the `--duration` defect (task 12) went unnoticed.
- `TestCLIUnknownProfileFallsBack` (`:160`) codifies the silent fallback that task 12 removes.

**Scope:**

- Build the command tree in a factory (for example `newRootCmd(deps)`) with an injectable orchestrator, output writer, and installer.
- Test through `cmd.SetArgs(...)` and `cmd.Execute()`, asserting on the configuration that reaches the orchestrator and on the exit errors.
- Replace the tautological tests, and update `TestCLIUnknownProfileFallsBack` together with task 12.

**Done when:** every subcommand and flag has at least one test that executes the real command. Coverage of `cmd/melisa` is reported, and it covers the flag-to-configuration paths.

### 60. Route library logging through one logger that honors `--quiet`

**Size:** S

**Evidence:**

- `internal/executor/executor.go:172`, `:175`, `:177` and `internal/ebpf/loader.go:128` call `log.Printf` directly, bypassing `output.Progress`.
- A `--quiet` run still printed four `[executor] …` lines to stderr.
- `output.Progress` writes to `os.Stderr` directly (`internal/output/progress.go:41`), so tests cannot capture its output.

**Scope:**

- Inject one logger (for example `log/slog`) through the configuration. Map `--quiet` and `--verbose` to levels, and keep stdout reserved for the report and for MCP JSON-RPC.
- In MCP mode, decide between stderr and MCP logging notifications.
- Make test output capturable.
- Task 10 moves failure details into the report; logs remain diagnostics only.

**Done when:** `--quiet` produces no stderr output for successful runs, library packages do not import `log`, and tests can assert on log output.

### 67. Report CRAP scores and dead code

**Size:** S

**Evidence:**

- The review computed CRAP (CC² × (1 − coverage)³ + CC) with a `go/ast` script and `go test -coverprofile`, using crap4go's complexity rules. Of 249 functions, 28 score above 30 and 86 score between 5 and 30. The top entries:

  | CRAP | Function | CC | Coverage |
  |---:|---|---:|---:|
  | 992 | `NetworkCollector.Collect` | 31 | 0% |
  | 726 | `GenerateRecommendations` | 99 | 60% |
  | 462 | `BCCExecutor.Run` | 21 | 0% |
  | 420 | `BCCCollector.Collect` | 20 | 0% |
  | 337 | `MemoryCollector.parseNUMAStats` | 20 | 7.5% |

- Most of the top ten match functions where the review found defects. `parseNUMAStats` has not been reviewed yet.
- Unreachable code is found only by hand (task 27). `deadcode` from `golang.org/x/tools` was not run in the review.

**Scope:**

- Add a standard-library-only Go program and a `make crap` target. It computes CRAP from `go test -coverprofile` with crap4go's rules (1 + `if`, `for`, `range`, each `case` and `select` clause, `&&`, `||`) and prints a table, worst first.
- Add `make deadcode` using a pinned `golang.org/x/tools` version, with `cmd/melisa` as the root.
- Publish both reports as CI job summaries next to the coverage summary (task 38). Keep them informational until tasks 24 and 61 reduce the top entries. Then fail when a changed function scores above 30.
- Review `MemoryCollector.parseNUMAStats`.

**Done when:** `make crap` and `make deadcode` run without root, CI shows both reports, and the threshold policy is documented in CONTRIBUTING.md.

### 71. Print a human-readable summary

**Size:** M

**Evidence:** `melisa collect` writes JSON only, and README teaches operators to read it with `jq`. During an incident, an operator needs the conclusion on screen at least as fast as `vmstat 1` and `iostat -xz 1` deliver it. Only `melisa diff` has a text output.

**Scope:**

- Add `--format text|json` to `collect`, and a `melisa show <report.json>` command that renders a saved report.
- Decide whether text becomes the default when stdout is a terminal. JSON stays the default for files and pipes.
- The summary contains:
  - a one-line host context: kernel, CPUs, memory, virtualization;
  - the health score and a status per resource;
  - anomalies by severity, with value and threshold;
  - collectors that failed or were skipped (task 10);
  - the top processes and the recommendation titles;
  - suggested next commands.
- Keep lines within 100 columns, and disable colors when stdout is not a terminal or `NO_COLOR` is set.

**Done when:** golden tests cover healthy, degraded, and partial reports, and README shows the text output.

### 72. Return automation-friendly exit codes

**Size:** S

**Evidence:** `melisa collect` exits 0 whenever it writes a report, whatever the anomalies, collector failures, or interruptions. Cron jobs, Ansible runs, and monitoring checks cannot react without parsing JSON.

**Scope:**

- Add an opt-in flag, for example `--fail-on=warning|critical`.
- Use distinct codes for warning, critical, partial report, and collection error. Offer a Nagios-compatible 0/1/2/3 mapping as an option.
- Keep 0 as the default, and document the codes.

**Done when:** tests cover every code, and README and the EN/RU documentation list them.

### 73. Sample Tier 1 rates over the collection window

**Size:** M

**Evidence:**

- CPU, disk, network, and memory rates come from a single two-point delta over `SampleInterval`, which is 1 s by default (`internal/collector/collector.go:101`), whatever the profile. The quick profile is nevertheless described as a 10 s collection.
- A 1 s window is noisy and misses bursts; operators use `vmstat 1 5` and `iostat -xz 1` for that reason. The `irq_imbalance` false positive (task 17) came from a 1 s window.

**Scope:**

- Take several samples, for example one per second over a configurable Phase 1 window, and report the mean in the existing fields.
- Decide how to trade a longer Phase 1 against observer effect and total runtime. One option is to keep sampling Tier 1 rates during Phase 2 at lowered priority. Document the choice.
- Expose minimum, maximum, and p95 as additive fields (see *Schema change candidates*), and let anomaly rules distinguish sustained load from bursts.

**Done when:** tests with fixture sequences cover the multi-sample statistics, the report metadata states the sampling window, and the documentation explains it.

### 74. Add missing saturation and limit signals

**Size:** M–L

**Evidence:** none of the following is collected today.

- The run queue and blocked tasks (`procs_running` and `procs_blocked` in `/proc/stat`), and a count of uninterruptible (`D`) tasks. `ProcessData` counts only running, sleeping, and zombie processes.
- Swap activity (`pswpin`, `pswpout`). `swap_usage` judges usage alone: it flagged the review host as critical at 96%, without any evidence that swapping was actually happening.
- OOM kills during the window: `oom_kill` in `/proc/vmstat`, and `memory.events` per cgroup.
- Inode usage per filesystem; only space is reported, via `df`.
- System-wide file handles (`/proc/sys/fs/file-nr` against `file-max`), and PID and thread usage (`kernel.pid_max`, `kernel.threads-max`).
- Interface throughput in bytes and packets per second. `rx_bytes` and `tx_bytes` are counters since boot, and the network USE utilization is always 0 (`internal/model/report.go`), although `enrichNICDetails` reads the link speed.
- CPU steal: `StealPct` is collected, but no anomaly rule uses it.

**Scope:**

- Collect each signal from procfs and sysfs. Read inode data through `statfs`, coordinated with task 20.
- Add anomaly rules with documented thresholds. Replace the usage-only swap rule with one based on swap activity.
- Compute network utilization from throughput and link speed.
- List the top `D`-state processes, with `wchan` where it is readable.
- Update the README rules table and the EN/RU documentation. The new fields are additive (see *Schema change candidates*).

**Done when:** fixture tests cover every signal and rule, and the rule count in README matches the code.

### 75. Add hardware, platform, and kernel-log signals

**Size:** M

**Evidence:**

- There is no CPU frequency governor or thermal throttling data (`scaling_governor`, `thermal_throttle/*_throttle_count`). A `powersave` governor on a server and thermal throttling are common causes of slow hosts.
- There are no memory error counters (EDAC `ce_count`, `ue_count`) and no machine-check information.
- There is no virtualization or cloud context (`/sys/class/dmi/id/*`, the `hypervisor` CPU flag), so steal time and noisy-neighbor effects cannot be interpreted.
- There is no time synchronization status. Clock offsets break TLS, distributed locks, and log correlation.
- There is no systemd state, so failed units and restart loops go unreported.
- `dmesg` fails silently when `kernel.dmesg_restrict=1`, which is the default on the review host. `collectDmesg` returns `nil` on any error (`internal/collector/system.go:177`–`181`), so the report shows neither the kernel log nor an error.

**Scope:**

- Read the governors, throttle counters, EDAC counters, and DMI and hypervisor data from sysfs and procfs.
- Read the time synchronization status without changing anything:
  - either `adjtimex` without modes, through `golang.org/x/sys/unix` (already an indirect dependency; making it direct needs discussion per AGENTS.md);
  - or `timedatectl show`.
- Report failed units through `CommandRunner` with `systemctl --failed --plain --no-legend`.
- When `dmesg` is restricted, fall back to `journalctl -k` or record an explicit error.
- Add anomaly rules where they are meaningful: throttling, uncorrected memory errors, an unsynchronized clock, failed units.

**Done when:** fixture and fake-runner tests cover every source and the restricted `dmesg` path, and the documentation is updated.

### 76. Add a node view for Kubernetes and containers

**Size:** M–L

**Evidence:**

- The container collector describes the cgroup Melisa itself runs in, or a single `--cgroup` path. `PodName` is Melisa's own `HOSTNAME` (`internal/collector/container.go:51`).
- On a Kubernetes node or a Docker host there is no breakdown per pod or per container: CPU throttling, memory usage against the limit, PSI, and OOM kills per workload.
- The top process lists carry no cgroup, service, or pod attribution.
- The README roadmap item "Per-cgroup memory pressure tracking" is not implemented.
- There is no documented way to run Melisa on a node (privileged pod, `hostPID`, mounts, `kubectl debug node/…`).

**Scope:**

- Enumerate the cgroup v2 hierarchy (`kubepods.slice`, `system.slice`, Docker scopes). Report the top N by throttling ratio during the window, by memory usage against the limit, by PSI, and by OOM events.
- Derive the pod UID, QoS class, and container ID from cgroup paths, without access to the API server.
- Attribute top processes to their cgroup and systemd unit.
- Document node deployment (task 82). The new fields are additive (see *Schema change candidates*).

**Done when:** fixtures with kubepods and system slices produce the expected per-workload data, and the documentation describes node deployment.

### 77. Protect the host during collection

**Size:** M

**Evidence:**

- BCC tools and other child processes inherit Melisa's CPU and I/O priority and its OOM score; there is no `nice`, `ionice`, or `oom_score_adj` adjustment. On a host under memory pressure, the OOM killer can therefore pick a workload instead of a diagnostic child.
- Phase 2 starts without a preflight check of memory and load. Task 19 adds cost classes and concurrency limits; this task adds host guards.
- Nothing prevents two collections from running at once, for example a cron job and an operator. Task 21 serializes collections only within one MCP server.

**Scope:**

- Start child processes with lower CPU and I/O priority and a raised `oom_score_adj`. This changes only Melisa's own children, not host state, and raising these values needs no privileges.
- Before Phase 2, check `MemAvailable`, PSI, and load. Skip expensive tools with a recorded reason, unless an explicit override flag is given.
- Hold a host-wide lock during a collection and report "another collection is running". An abstract Unix socket avoids writing to the filesystem; abstract sockets are scoped to a network namespace, so document that limitation.
- Enforce a total time budget for the run.

**Done when:** tests cover the priority settings through a fake executor, preflight skips, the lock, and the time budget, and the documentation describes the guards.

### 78. Make readiness checks accurate and machine-readable

**Size:** S–M

**Evidence:**

- `melisa capabilities` prints text only.
- For a non-root user on the review host it reported "BPF Capability Level: Tier 3", because it checks the kernel configuration and BTF, not privileges.
- It does not show the installed BCC tools, the BCC version, the embedded native programs, or the kernel lockdown mode.
- For `kernel.unprivileged_bpf_disabled` and `kernel.perf_event_paranoid` it checks only that the files exist, not their values (`internal/ebpf/btf.go:48`, `:73`).
- For the same non-root user it showed `kprobes` as unavailable, yet the overall level was still Tier 3.

**Scope:**

- Add `--json`.
- Report:
  - effective privileges: euid and the `CapEff` bits for `CAP_BPF`, `CAP_PERFMON`, `CAP_SYS_ADMIN`;
  - the lockdown mode (`/sys/kernel/security/lockdown`) and the sysctls above;
  - tracefs and debugfs availability;
  - the BCC version and per-tool availability, optionally with the argv validation from task 11;
  - the embedded programs (task 31).
- Compute the achievable tier from all of these, and exit with a non-zero code when a tier requested with `--require-tier` is unavailable.

**Done when:** fixture tests cover the root, non-root, and lockdown cases, and fleet readiness can be checked with `melisa capabilities --json`.

### 79. Preview a collection with `--dry-run`

**Size:** S–M

**Evidence:** an operator cannot see in advance which collectors and BCC tools a profile will run, with which arguments, at what cost, and what will be skipped. Tasks 11, 19, and 77 change all of these.

**Scope:** `melisa collect --dry-run` prints, as text or JSON:

- the selected collectors and tools, with their argv and cost class;
- skip reasons;
- the expected duration.

It loads no BPF programs and starts no tools.

**Done when:** a test with a fake executor shows that no process is started, and README documents the flag.

### 80. Allow threshold overrides and rule suppression

**Size:** M

**Evidence:** all 37 anomaly thresholds are hard-coded (`internal/model/anomaly.go`), yet host roles differ: databases run with high memory utilization, batch hosts at 100% CPU, desktops with full swap. Operators cannot tune or silence a rule, so recurring false positives hide real findings.

**Scope:**

- Add an optional rules file (for example `--rules <path>` or `/etc/melisa/rules.json`) that overrides warning and critical values and disables rules. Add a CLI flag to ignore specific rules.
- Use JSON from the standard library, or discuss a YAML dependency per AGENTS.md.
- Record the applied overrides in the report (see *Schema change candidates*) and in the text summary (task 71).
- Keep "no configuration required" as the default.

**Done when:** tests cover overrides, suppression, invalid files, and the recorded overrides, and the documentation describes the file format.

### 86. Extend memory detail

**Size:** M

**Evidence:**

- `MemoryCollector.parseMeminfo` (`internal/collector/memory.go:94`) reads only 10 keys of `/proc/meminfo`. Not reported:
  - `Committed_AS` and `CommitLimit` (overcommit headroom). Task 14's overcommit recommendation is made without looking at either.
  - `Shmem`, for tmpfs filling RAM.
  - `SReclaimable` and `SUnreclaim`, for slab growth.
  - `AnonHugePages`, `PageTables`, `Mlocked`, `Unevictable`, `KernelStack`.
- Process memory is RSS only. There is no PSS, USS, or per-process swap (`/proc/<pid>/smaps_rollup`).
- zswap and zram state and KSM are not reported. Slab caches are visible only through the BCC tool `slabratetop`.

**Scope:**

- Parse the listed meminfo keys.
- Add PSS and swap for the top processes from `smaps_rollup` where it is readable.
- Report zswap and zram (`/sys/module/zswap/parameters`, `/sys/block/zram*`) and KSM (`/sys/kernel/mm/ksm`).
- Add rules for low commit headroom, unreclaimable slab growth, and large `Shmem`.
- Feed commit headroom into task 14.
- The new fields are additive (see *Schema change candidates*).

**Done when:** fixture tests cover every new field and rule, and the documentation lists them.

### 87. Report storage and filesystem health

**Size:** M–L

**Evidence:**

- `isVirtualOrPartition` (`internal/collector/disk.go:222`) skips every `dm-*` device. Latency and utilization of LVM, LUKS, and other device-mapper volumes are therefore invisible.
- RAID state from `/proc/mdstat` (degraded or resyncing arrays) and multipath state are not read.
- No filesystem errors are reported: neither ext4 `errors_count`, nor XFS shutdown, nor a filesystem remounted read-only. Mount options are not read at all.
- Disk health (SMART, NVMe smart-log: temperature, wear, media errors) is not collected.
- Deleted files that are still open and hold space ("`df` is full but `du` is not") are not detected.
- Per-process I/O (`/proc/<pid>/io`) is available only through the BCC tool `biotop`.
- NFS client statistics from `/proc/self/mountstats` (per-mount RTT and retransmissions) are available only through the BCC tool `nfsslower`.

**Scope:**

- Report device-mapper devices with their slave mapping, without double-counting I/O in totals.
- Parse `/proc/mdstat`.
- Read mount options and filesystem error state from `/proc/self/mountinfo` and `/sys/fs/ext4/*/errors_count`.
- Read NVMe health from sysfs where available, and SMART through `smartctl` as an optional tool via `CommandRunner`.
- Count deleted-but-open files per process with bounded scanning of `/proc/<pid>/fd`.
- Add per-process I/O for the top processes, and parse NFS `mountstats`.
- Add anomaly rules for a degraded array, a read-only filesystem, filesystem errors, failing disk health, and large deleted-but-open usage.

**Done when:** fixture tests cover every source and rule, including a host without `smartctl`, and the documentation is updated.

### 88. Extend network detail

**Size:** M

**Evidence:**

- `/proc/net/snmp6` is never read, so IPv6 retransmissions and errors are invisible (task 13 covers IPv6 only in the native tcpretrans collector).
- Queueing disciplines report only the `default_qdisc` sysctl. Per-interface drops, backlog, and requeues (`tc -s qdisc`) are missing.
- Neighbour table overflow (`table_fulls` in `/proc/net/stat/arp_cache` and `ndisc_cache`) is not read, although a recommendation tunes `gc_thresh3`.
- Bonding reports only "is a slave"; slave state and failover counts from `/proc/net/bonding/*` are missing.
- Everything is collected in the host network namespace only, so container network namespaces are invisible (related to task 76).
- DNS resolver configuration (`/etc/resolv.conf`, `ndots`), policy routing, MTU mismatches, nftables rule counts, and attached XDP/tc BPF programs are not reported.

**Scope:**

- Parse `snmp6` and add IPv6 counterparts of the TCP and UDP rules.
- Read qdisc statistics through netlink or `tc -s -j qdisc` via `CommandRunner`.
- Parse neighbour cache statistics and bonding state.
- Report resolver settings and interface MTUs.
- Evaluate per-namespace collection together with task 76.
- Add anomaly rules for qdisc drops, neighbour table overflow, a degraded bond, and IPv6 retransmissions.

**Done when:** fixture and fake-runner tests cover every source and rule, and the documentation is updated.

### 89. Report kernel state and recent kernel events

**Size:** M

**Evidence:**

- `dmesg --level=err,warn` (`internal/collector/system.go:177`) reads the whole ring buffer without a time window, so errors from boot time look current.
- The tainted flag (`/proc/sys/kernel/tainted`), loaded and out-of-tree modules, and live patches are not reported.
- Hung tasks, soft lockups, and RCU stalls appear only as unclassified `dmesg` lines; there are no counters.
- SELinux and AppArmor denials (audit) are not reported.
- Reboot and crash history (journal boots, `/var/crash`, kdump) is not reported.
- Only about 30 network sysctls and a few `vm` sysctls are captured. There is no fuller sysctl snapshot to compare against.

**Scope:**

- Limit kernel messages to a configurable recent window (task 75 adds the `journalctl -k` fallback), and classify hung task, soft lockup, RCU stall, OOM, and I/O error messages into counters.
- Report the tainted flags decoded, plus out-of-tree and unsigned modules.
- Report recent LSM denials from the journal or the audit log when readable, and boot and crash history.
- Capture an allow-listed sysctl snapshot that task 81's drift report can use.

**Done when:** fixture tests cover message classification, the time window, tainted decoding, and the missing-permission paths, and the documentation is updated.

### 91. Extend per-process detail

**Size:** S–M

**Evidence:** for the top processes, Melisa reports CPU, RSS, thread count, FD count, and state. Missing:

- voluntary and involuntary context switches (`/proc/<pid>/status`; the equivalent of `pidstat -w`);
- resource limits (`/proc/<pid>/limits`), for example an `nofile` limit close to the FD count;
- process age and parent;
- the OOM score;
- recent core dumps (`coredumpctl`, journal) and restart loops (task 75 covers systemd units).

**Scope:**

- Add context-switch rates over the window, limits with a "close to limit" flag, process start time and parent, and `oom_score`.
- Report recent core dumps where readable.
- Add rules for FD usage close to the limit and for high involuntary context switching.
- Keep scanning bounded to the top N processes.

**Done when:** fixture tests cover every field and rule, and the documentation is updated.

## Priority 3

### 25. Harden the BCC subprocess environment and binary checks

**Size:** S

**Evidence:**

- `SanitizeEnv` (`internal/executor/security.go:116`) keeps the caller's `PATH` and `HOME`. BCC tools are Python scripts (`#!/usr/bin/python3` on Ubuntu), and Python loads user site-packages from `HOME`.
- `VerifyBinary` checks root ownership and the world-writable bit only. It does not check for group-writable binaries or parent directory permissions.
- Entries in `/snap/bin` are symlinks to `/usr/bin/snap`, which dispatches by `argv[0]`, while Melisa executes the resolved path.

**Scope:**

- Set a fixed `PATH`, a fixed or empty `HOME`, and `PYTHONNOUSERSITE=1`.
- Reject group-writable binaries and binaries in writable parent directories.
- Preserve `argv[0]`, or drop `/snap/bin` from the allowed paths.

**Done when:** unit tests cover the environment contents and each permission rule.

### 26. Use iostat-compatible disk latency

**Size:** S

**Evidence:** `internal/collector/disk.go:89` divides the weighted I/O time delta by the number of read and write operations. This mixes two things:

- the weighted time includes in-flight I/O at the window edges and discard/flush time;
- discard and flush operations are not counted in the divisor.

iostat computes `r_await` and `w_await` from the read and write ticks (`fields[6]` and `fields[10]` in `readDiskStats`), which Melisa does not parse.

**Scope:** parse the read and write ticks and compute latency the way iostat does. Reuse the weighted time for average queue size (task 17). Keep the `avg_latency_ms` field.

**Done when:** fixture tests match hand-computed iostat values, including a sample with heavy discard/flush activity.

### 27. Remove dead code and fix build metadata

**Size:** S

**Evidence:**

- `GenerateFlameGraphSVG` and `GenerateFlameGraphFromFolded` are used only by tests, and the `flamegraph_svg` field is never populated.
- `BCCExecutor.auditLog` is always false and would write audit lines into the tool's stderr buffer.
- `.goreleaser.yaml` sets `-X main.commit` and `-X main.date`, but `cmd/melisa` defines only `version`.
- The `ExecCommandRunner` comment says "combined output", but the function returns stdout only.
- `sched_latency_ns` and `sched_min_granularity_ns` are read from `/proc/sys`. Kernels 5.13 and newer moved them to debugfs, and EEVDF kernels removed them, so these fields are always empty on such kernels.

**Scope:**

- Wire FlameGraph generation into an explicit output, or remove the generator. Keep the schema field until a major schema change.
- Use the `deadcode` report from task 67 to find the remaining unreachable code. For example, `orchestrator.BuildReport` has no callers.
- Remove or fix audit logging.
- Add commit and date variables and print them in `--version`.
- Fix the comments and document the CFS fields.

**Done when:** `make check` passes and a snapshot build prints the commit and date in `melisa --version`.

### 28. Update vulnerable indirect modules

**Size:** S

**Evidence:** `govulncheck -show verbose` reports these vulnerabilities. Melisa code does not reach any of them, and Dependabot has not proposed these indirect updates.

| Vulnerability | Module | Fixed in |
|---|---|---|
| GO-2026-6629 | `golang.org/x/text@v0.14.0` | v0.41.0 |
| GO-2026-5970 | `golang.org/x/text@v0.14.0` | v0.39.0 |
| GO-2026-5024 | `golang.org/x/sys@v0.43.0` | v0.44.0 |

**Scope:** bump both modules and run `go mod tidy`. Check whether `.github/dependabot.yml` should allow indirect updates.

**Done when:** govulncheck reports no module findings and `make check` passes.

### 36. Clean up workflow configuration

**Size:** S

**Evidence:**

- The BPF toolchain installation block is copied three times: the `bpf-smoke` and `release-snapshot` jobs in `ci.yml`, and `release.yml`.
- `ci.yml` triggers on the `develop` branch, which exists neither locally nor on `origin`.
- `cancel-in-progress: true` also cancels runs on `main`, so some `main` commits end up without a CI result.
- Every job runs on the floating `ubuntu-latest` image.

**Scope:**

- Move the toolchain installation into one script (for example `scripts/ci/install-bpf-toolchain.sh`) or a composite action.
- Remove the `develop` triggers. Cancel in-progress runs only for pull requests: `cancel-in-progress: ${{ github.event_name == 'pull_request' }}`.
- Pin runner images (for example `ubuntu-24.04`), coordinating with tasks 30 and 32.

**Done when:** the workflows contain no duplicated toolchain block, no dead triggers, and no floating images, and every CI job stays green.

### 37. Configure golangci-lint explicitly

**Size:** M

**Evidence:**

- There is no `.golangci.yml`, so only the default linter set runs. That is thin coverage for code that executes processes and parses procfs and tool output.
- The Makefile runs `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`. The golangci-lint project does not recommend this installation method, because dependency resolution can differ from the release binaries.
- A trial run of additional built-in linters reported 666 findings, 106 of them in tests for `goconst` alone. Non-test findings of the useful linters:

  | Linter | Findings | Examples and owning tasks |
  |---|---:|---|
  | `errorlint` | 1 | `executor.go:188`: type assertion on the `ExitError` (task 10) |
  | `nilerr` | 1 | `parsers.go:332`–`335` (task 10) |
  | `nilnil` | 1 | `gpu.go:50` (task 18) |
  | `noctx` | 13 | 12 in the installer (task 23); `executor.go:81` is intentional and needs a justified exclusion |
  | `forcetypeassert` | 2 | `executor.go:85`–`86` (task 64) |
  | `gochecknoglobals` | 10 | task 42 |
  | `gocyclo` | 4 | above 30; the same functions as in task 61 |
  | `gocognit`, `nestif` | 9, 11 | task 61 |
  | `dupl` | 0 / 145 | 0 at the default threshold of 150 tokens, 145 pairs at a threshold of 60 (task 62) |
  | `modernize` | 49 | task 63 |
  | `gosec` | 44 | G304 ×22 and G204 ×7 are expected for procfs reads and the executor; G115 ×14 (task 18); G306 ×1 |

- `goconst` (245) and `revive` (130) are noisy without tuning.

**Scope:**

- Add `.golangci.yml` with these linters enabled:
  - correctness: `errorlint`, `nilerr`, `nilnil`, `noctx`, `forcetypeassert`;
  - structure: `gochecknoglobals`, `gocyclo`, `gocognit`, `nestif`, `dupl` (threshold about 80–100);
  - style and safety: `modernize`, `misspell`, `gosec`, `gocritic`.
- Exclude `gosec` G304 and G204 by path for procfs readers and the executor, and set `goconst` and `revive` to explicit, low-noise rules.
- Adopt the configuration gradually with `issues.new-from-merge-base: main`, so new and changed code must comply at once. Existing findings are fixed by the owning tasks listed above.
- Tighten thresholds as tasks 61 and 62 land: for example, lower `gocyclo` from 30 to 15.
- Install golangci-lint from the official release binary with checksum verification, or through the official Action. Keep `v2.15.0` pinned in one place.
- Fix findings or document narrowly scoped exclusions; do not use blanket `nolint`.

**Done when:** `make lint` and CI use the configuration and the pinned binary, new code passes every enabled linter, and each remaining existing finding is owned by a backlog task or covered by a documented exclusion.

### 38. Check scripts, documentation links, and coverage in CI

**Size:** S–M

**Evidence:** CI checks only Go code.

- The shell scripts (730 lines) and Python validators (571 lines) in `tests/` run only in the manual suites, so breakage is noticed late.
- Markdown links in 58 files are unchecked.
- Workflows are neither linted nor audited for security. actionlint and zizmor were not run in the review.
- Coverage is not reported, although task 24 lists packages between 0% and 51%.
- CodeQL default setup was enabled on 2026-11-27 for Go, Actions, and Python. The BPF C source is not covered.

**Scope:**

- Run `shellcheck` on `tests/**/*.sh` and a Python syntax check (`python3 -m py_compile` or ruff) on `tests/**/*.py`.
- Lint workflows with actionlint (MIT) and audit them with zizmor (MIT), both pinned. Expected zizmor findings include the persisted credentials in the release job (task 29), tag-pinned Actions (task 7), and job permissions.
- Check internal Markdown links (for example with lychee in offline mode). External links can be checked on a schedule.
- Publish a coverage summary (`go test -coverprofile` plus `go tool cover -func`) as a job summary or artifact. Add thresholds only after task 24.
- Keep CodeQL green. After task 30, consider an advanced setup that also covers the BPF C source.
- Pin any new Actions according to task 7.

**Done when:** pull requests fail on shellcheck errors, Python syntax errors, broken internal links, and actionlint or zizmor findings above the agreed severity, and every CI run shows a coverage summary.

### 39. Split `internal/model` into report types and analysis

**Size:** M–L

**Evidence:**

- `internal/model` holds 566 lines of report types and about 1,800 lines of analysis: `anomaly.go` (852), `recommendations.go` (740), `report.go` (USE metrics), and `health.go`.
- The AI prompt generator is also analysis-level logic, yet it lives in `internal/output` next to the JSON writer, terminal progress, and FlameGraph code.

**Scope:**

- Keep report types and `SchemaVersion` in a types package (for example `internal/report`).
- Move USE metrics, anomaly rules, the health score, and recommendations to an analysis package (for example `internal/analysis`).
- Move AI prompt generation next to the analysis code or into its own package, and leave `internal/output` with serialization and progress.
- Make behavior-preserving moves only: JSON output must stay byte-identical for the same input.

**Done when:** the packages are split, the import graph stays acyclic, the tests pass with only import-path changes, and JSON output for the `testdata` fixtures is byte-identical.

### 40. Keep one source of truth for anomaly IDs and explanations

**Size:** S–M

**Evidence:** thresholds are defined in `internal/model/anomaly.go`, while the explanations are a separate text map of about 440 lines in `internal/mcp/handlers.go:269`. The two lists have already diverged ("37 active rules and 3 legacy IDs"), and the CLI cannot show explanations at all.

**Scope:**

- Store each rule's explanation next to its definition, for example in a rule catalog with Markdown embedded through `go:embed`.
- Generate `list_anomalies` from the catalog, and decide what to do with the legacy IDs.
- Optionally add `melisa explain <id>`. This is user-visible, so update README and the EN/RU documentation.
- Add a test that every rule has an explanation and that every explanation has a rule or an explicit legacy marker.

**Done when:** anomaly IDs are defined in one place and the consistency test passes.

### 41. Consolidate BCC code into one package

**Size:** M

**Evidence:** BCC code is spread over two packages.

- `internal/executor` contains the tool registry (797 lines) and output parsers (517 lines) next to process execution and binary security checks.
- The adapter to the collector interface is in `internal/collector/bcc_adapter.go`.

**Scope:**

- Move the registry, parsers, and adapter into one package (for example `internal/bcc`).
- Keep `internal/executor` as the generic, security-checked process runner.
- Keep the change behavior-preserving, and coordinate with tasks 10 and 11, which change the same code.

**Done when:** all BCC-specific code lives in one package, the executor holds no knowledge of BCC tools, and the tests pass.

### 42. Remove exported mutable package state

**Size:** S

**Evidence:**

- `internal/executor/security.go:12` exports `AllowedBinaryPaths` as a mutable slice, so any package can widen the set of directories binaries are executed from.
- `internal/executor/registry.go:36` exports the mutable `Registry` map.
- `internal/model/anomaly.go:17`: `DefaultThresholds()` returns the internal slice, so callers can modify the rules.
- `internal/mcp/handlers.go:25` keeps the version in the package variable `mcpVersion`, which `NewServer` sets.
- In total, `gochecknoglobals` reports 10 package-level variables in non-test code.

**Scope:**

- Unexport these values and expose read-only accessors or copies.
- Pass the version through the server struct.
- Tests that need other values use explicit constructors.

**Done when:** no exported package-level variable controls the security checks, the tool registry, the anomaly rules, or the version; the tests pass; and `gochecknoglobals` (task 37) reports only documented exceptions.

### 43. Add `melisa validate` and tidy the test scripts

**Size:** M

**Evidence:**

- `tests/integration/verify_report.py` (119 lines) and `tests/validation/check_detection.py` (452 lines) re-encode the report structure, which is already described by the Go types and by `docs/schema/report-1.1.0.schema.json`.
- The JSON Schema validator (`santhosh-tekuri/jsonschema`) is a direct dependency but is used only in `internal/output/schema_test.go`.
- `tests/integration/verify_release_artifacts.sh` embeds a Python program in a Bash heredoc.
- `tests/integration/docker_test.sh:35` builds into the fixed path `/tmp/melisa-integration-test`.

**Scope:**

- Add `melisa validate <report.json>`, which checks a report against the embedded schema. This is user-visible: update README and the EN/RU documentation.
- Use it from the integration scripts instead of re-implementing structural checks.
- Consider porting `check_detection.py` to Go tests behind a `validation` build tag, compiled with `go test -c` and copied to the validation host.
- Replace the heredoc with a standalone script or `jq`, and use `mktemp` for temporary binaries.

**Done when:** the scripts rely on `melisa validate` for structural checks, no Python is embedded in shell, temporary paths are unique, and the approved manual suites still pass.

### 44. Enrich deb and rpm packages

**Size:** S

**Evidence:**

- The `nfpms` section of `.goreleaser.yaml` declares no `recommends` or `suggests`, so installing the package gives no hint that BCC is needed.
- The packages ship no shell completions and no man page, although Cobra can generate both.

Version and build metadata are covered by task 27.

**Scope:**

- Add per-format `recommends` through nfpm overrides: `bpfcc-tools` for deb, `bcc-tools` for rpm.
- Generate shell completions and a man page in a GoReleaser `before` hook, and install them to the standard paths. The man page generator (`cobra/doc`) pulls in `go-md2man`; discuss that dependency per AGENTS.md.
- Extend the task 31 verification to the new files.

**Done when:** the packages recommend BCC and install completions and a man page, and release verification checks them.

### 45. Clean up repository hygiene

**Size:** S

**Evidence:**

- `.gitignore` mixes project patterns with personal or local entries: `docs/RELEASE_WORKFLOW.md`, `TEST_RELEASE.md`, `scripts/test_mcp_*.sh`, `scripts/test_null_fix.sh`, `articles/`. It also ignores `.claude/` entirely, which rules out ever sharing a project-level `.claude/settings.json`.
- The root `testdata/` is referenced 38 times as `../../testdata`. The BCC tool outputs belong to the executor, and the procfs/sysfs trees belong to the collectors.
- `cmd/melisa/main.go` declares every command as a closure inside `main()`, while `mcp` lives in its own file as a global variable.

**Scope:**

- Move personal entries to `.git/info/exclude` or a global ignore file, keep the project patterns, and decide whether `.claude/` should stay ignored as a whole.
- Optionally move package-specific fixtures next to their packages, keeping shared ones such as `report_1.1.0.json` at the root.
- Split `cmd/melisa` into one file per command with a consistent construction style.

**Done when:** `.gitignore` contains only project patterns and the CLI has one file per command. Stale branches were already handled on 2026-11-27 (see *Repository settings applied on 2026-11-27*).

### 50. Add OpenSSF Scorecard

**Size:** S

**Evidence:** the repository has no Scorecard analysis. It already meets several Scorecard checks: branch protection, security policy, read-only default token, and CodeQL since 2026-11-27. It fails others, such as pinned dependencies and signed releases, which tasks 7 and 33 address.

**Scope:**

- Add the `ossf/scorecard-action` workflow, pinned by SHA. Keep permissions read-only except `security-events: write` and `id-token: write`, which are needed to publish results.
- Publish the results to code scanning and to the public Scorecard API.
- Add the Scorecard badge to README.

**Done when:** scheduled Scorecard runs succeed, the findings appear in code scanning, and README shows the badge.

### 51. Sign commits and tighten branch protection

**Size:** S

**Evidence:**

- None of the last eight commits on `main` are verified (`verification.verified=false`).
- `main` protection has `required_signatures: false` and `required_conversation_resolution: false`.

**Scope:**

- Set up SSH commit signing for maintainers, document it in CONTRIBUTING.md, and enable vigilant mode.
- Then enable required signatures and conversation resolution on `main`. Squash merges made in the GitHub UI and Dependabot commits are signed by GitHub.
- Check that Dependabot updates can still be merged.

**Done when:** new commits on `main` are verified and both protection settings are enabled.

### 52. Track work in GitHub Issues with labels and milestones

**Size:** S–M

**Evidence:**

- The repository has no issues and no milestones, and only the default labels plus Dependabot's. The 46 tasks exist only in this file.
- The issue forms set no `labels:`, so new issues arrive unlabeled.
- `blank_issues_enabled: true` is set while Discussions are disabled, so questions will arrive as issues.
- The bug form asks for `melisa --version`, OS, kernel, and architecture. It does not ask for `melisa capabilities` output or the BCC version, which tasks 10 and 11 show are needed to triage collection problems.

**Scope:**

- Define labels (type, `area/*`, `priority/*`, `good first issue`) and one milestone per release.
- Create issues from this backlog, and link each task here to its issue or replace this file with a pointer.
- Add `labels:` to the issue forms, and add `melisa capabilities` and BCC version fields to the bug form.
- Decide whether to enable Discussions for questions or to disable blank issues.

**Done when:** every open backlog task has an issue with labels and a milestone, and new issues arrive labeled.

### 53. Make the README reflect the real project status

**Size:** S

**Evidence:**

- All four README badges are static: Go version, license, "BCC tools 67/80" in green, schema version. There are no CI, release, coverage, or Scorecard badges.
- The BCC badge overstates the working coverage (task 11).
- The repository has no homepage URL and no social preview image, and its description says "Single-binary" (task 35).

**Scope:**

- Replace the static badges with CI status, latest release, Scorecard (task 50), and coverage (task 38) badges. Remove or correct the BCC badge.
- Publish the documentation, for example on GitHub Pages with an EN/RU switch (coordinate with task 46), and set it as the homepage.
- Add a social preview image, and align the description with task 35.

**Done when:** the README badges are live, and the About section has a homepage and an accurate description.

### 54. Document and broaden installation channels

**Size:** S–M

**Evidence:**

- Installation means downloading binaries or deb/rpm packages from Releases by hand. README has no checksum verification step.
- README does not mention `go install`, which builds without native eBPF (task 30).
- There is no container image, although images are a common way to run eBPF tools on Kubernetes (`kubectl debug`, a privileged DaemonSet).

**Scope:**

- Add an install snippet that downloads the binary and verifies it against `checksums.txt`, and against signatures after task 33.
- Document the limitations of `go install`.
- Evaluate a GHCR image built by GoReleaser, with the required privileges documented.
- Optionally, evaluate an apt/yum repository.

**Done when:** README has verified installation instructions, and the decision on container images is recorded.

### 55. Add CODEOWNERS for sensitive paths

**Size:** S

**Evidence:** there is no CODEOWNERS file, and branch protection requires no reviews. No owner is designated for the workflows, the release configuration, or the executor security checks.

**Scope:**

- Add `.github/CODEOWNERS` for `.github/`, `.goreleaser.yaml`, `Makefile`, `internal/executor/security.go`, `internal/executor/executor.go`, `internal/ebpf/`, and `docs/schema/`.
- Enable "Require review from Code Owners" only once there is a second maintainer; a sole maintainer cannot approve their own pull requests.

**Done when:** CODEOWNERS is in place and GitHub shows the owners on pull requests that touch these paths.

### 61. Replace `GenerateRecommendations` with a rule table

**Size:** M

**Evidence:**

- `GenerateRecommendations` (`internal/model/recommendations.go:11`) is a single function of 708 lines with cyclomatic complexity 99.
- The priority of each recommendation comes from the order of the `if` blocks and 35 manual `priority++` statements. Inserting a rule renumbers every rule after it.
- Individual rules cannot be tested in isolation.
- Other large functions:

  | Function | Lines | Complexity |
  |---|---|---|
  | `GenerateAIPrompt` (`internal/output/ai_prompt.go:11`) | 171 | 39 |
  | `NetworkCollector.enrichNICDetails` (`internal/collector/network.go:540`) | 120 | 38 |
  | `NetworkCollector.Collect` (`internal/collector/network.go:44`) | 175 | 31 |

  In total, 16 non-test functions have complexity above 15.

**Scope:**

- Represent recommendations as a table of rules, like the anomaly thresholds. Each rule has an ID, a function that checks whether it applies and returns the evidence, and a function that builds the recommendation.
- Derive priority from type and severity, not from code position, and sort deterministically.
- Test each rule on its own.
- Split the other listed functions into smaller, testable units.
- Coordinate with task 14 (which changes recommendation content) and task 37 (complexity linters).

**Done when:** every recommendation rule has its own test, output for the existing fixtures is unchanged apart from intentional changes from task 14, and no function in the listed files exceeds the complexity limit chosen in task 37.

### 62. Remove duplicated helpers and boilerplate

**Size:** M

**Evidence:**

- The registry repeats `ParseTabularEvents(raw, 1000)` in 38 closures. Each closure rebuilds a `Result` with a `Collector` and `Category` that the tool spec already defines and the adapter overwrites anyway.
- `internal/model/anomaly.go` repeats the `if results, ok := r.Categories["…"]; ok { for … r.Data.(*T) }` lookup 33 times. There are 47 `.Data.(*T)` assertions in five files.
- Three PSI parsers are nearly identical: `CPUCollector.parseCPUPSI`, `DiskCollector.parseIOPSI`, `MemoryCollector.parsePSI`.
- `readFile` is duplicated (`internal/collector/system.go:94`, `internal/collector/disk.go:212`). The sysctl readers in `system.go` are used by other collectors.
- `NetworkCollector` has three constructors (`internal/collector/network.go:24`, `:29`, `:34`).
- `bcc_adapter.go` copies `cfg.PIDTracker` into a local variable through a redundant `if`.
- With a threshold of 60 tokens, `dupl` reports 145 duplicate pairs in non-test code, including the PSI parsers (`cpu.go:205` ↔ `disk.go:121`) and dozens of registry entries. At the default threshold of 150 it reports none.

**Scope:**

- Move procfs/sysfs reading and parsing helpers into one tested package (for example `internal/procfs`). The helpers return errors (task 18).
- Add a generic lookup (for example `dataOf[T any](r *Report, category string) (*T, bool)`) and use it in analysis code.
- Replace the tabular closures with a declarative helper (task 66).
- Use one constructor with explicit dependencies, and remove redundant code.

**Done when:** the duplicates listed above are gone, `dupl` at the threshold chosen in task 37 reports no pairs in these areas, and JSON output for the existing fixtures is byte-identical.

### 63. Modernize Go idioms, identifier names, and constants

**Size:** S

**Evidence:**

- Outdated idioms despite Go 1.27:
  - `interface{}` 20 times against one use of `any`;
  - `sort.Slice` 8 times while the `slices` package is never used;
  - manual minimum computations instead of the `min` built-in;
  - line splitting with `strings.Split(…, "\n")`;
  - in total, `modernize` in golangci-lint reports 49 findings in non-test code.
- Non-idiomatic names:
  - `SCREAMING_SNAKE` constants for output types (`HISTOGRAM_PER_DISK`, `JSON_OUTPUT` in `internal/executor/registry.go:26`–`32`);
  - wrong initialisms (`TcpretransEvent`, `Pid`, `Saddr`, `Daddr`, `Lport`, `Dport`, `TcpExt`).
- Trivial wrappers in `cmd/melisa/main.go`: `parseDuration` (just `time.ParseDuration`), `buildAIContext`, `runInstall`, `runCapabilities`.
- Categories, severities, recommendation types, and collector names are bare strings. For example, `histogramP99Evaluator` matches the collector name `"biolatency"` as a string, so renaming a collector would silently disable a rule.

**Scope:**

- Run `go fix ./...` (Go 1.26 and later apply the modernize analyzers) and review the result.
- Rename constants and identifiers to Go conventions. JSON tags stay unchanged.
- Introduce typed constants for categories, severities, recommendation types, and collector names. JSON values stay unchanged.
- Remove the trivial wrappers.
- Enable the naming rules in the linter configuration (task 37).

**Done when:** `go fix ./...` makes no further changes, the naming rules pass, and JSON output for the existing fixtures is byte-identical.

### 64. Remove premature optimizations

**Size:** S

**Evidence:**

- `bufferPool` (`internal/executor/executor.go:19`) pools stdout buffers "to avoid 67+ allocations", yet `stdout.String()` copies the data anyway, and pooled buffers keep their capacity of up to 50 MB. `forcetypeassert` also flags the unchecked assertions on the pooled buffers (`executor.go:85`–`86`).
- The native tcpretrans collector decodes events by hand "to avoid reflection" for at most 1000 events; task 30 replaces this with generated bindings.
- `defaultThresholds` is kept as a singleton "to avoid re-creating 37 closures", and parsers pre-lowercase headers.
- These choices add complexity to a tool that runs one collection every 10–60 s, while basic correctness issues remain (tasks 10 and 11).

**Scope:**

- Remove the buffer pool unless a benchmark of a realistic collection shows a measurable benefit.
- Prefer clear code where no measurement justifies the optimization.
- Compare benchmark runs with `benchstat` (`golang.org/x/perf`).
- Keep the benchmarks that measure realistic paths; remove only those tied to deleted code.

**Done when:** each remaining optimization is justified by a recorded measurement, and the code paths above are simplified.

### 65. Audit misleading and prototype comments

**Size:** S

**Evidence:**

- Prototype notes in production code:
  - `internal/ebpf/c/tcpretrans.bpf.c:48`–`49`: "Simplified … for demo";
  - `internal/collector/ebpf_tcpretrans.go:85`: "In a real long-running agent…";
  - `internal/ebpf/loader.go:99`: "We do this simply; genericCO-RE might need more options";
  - `internal/executor/registry.go:88` and `:161`: "for now".
- Comments that contradict the code:
  - `internal/collector/collector.go:16` promises "combined output", while the implementation returns stdout only;
  - `internal/collector/bcc_adapter.go:130` claims to override parser limits (task 58);
  - `internal/output/ai_prompt.go:71` claims PID filtering for biolatency (task 22).

**Scope:**

- Make every comment describe actual behavior.
- Replace prototype notes with a reference to a backlog task or issue, or remove them.

**Done when:** a search for `demo`, `for now`, `in a real`, and `simplified` in non-test code finds nothing, or only comments that reference a task, and the contradicting comments are fixed.

### 66. Move text data out of Go code

**Size:** S–M

**Evidence:**

- Anomaly explanations are a 439-line string map in `internal/mcp/handlers.go:269` (task 40 owns moving them next to the rules).
- The AI prompt, including its 27 anti-patterns, is assembled with `strings.Builder` inside the 171-line `GenerateAIPrompt`.
- The BCC registry consists of 67 hand-written closure literals. In most of them, argv construction and parser choice could be expressed as data: duration style (positional interval and count, `-d`, `-D`, none), PID flag, and parser kind.

**Scope:**

- Move the prompt text into embedded templates (`go:embed` plus `text/template`) and test the output with golden files.
- Make registry entries declarative, so that argv is derived from data. Hand-written code remains only for genuinely special tools. This also simplifies task 11's argv contract test.

**Done when:** the prompt text lives in embedded files covered by a golden test, and registry entries contain closures only where a declarative description is impossible.

### 68. Add mutation testing for logic packages

**Size:** M (after task 24)

**Evidence:**

- Coverage shows which code runs, not whether the tests would notice a change. It is not known whether the tests assert boundaries such as `>=` versus `>` in anomaly thresholds and health-score bands.
- Under mutate4go-style rules the code has 1,780 mutation sites: 682 in `internal/collector`, 459 in `internal/model`, and 243 in `internal/executor`.
- Most collector sites are uncovered today, so mutation results there would only repeat the coverage gaps.
- Package test runs take 0.6–2.1 s. With recompilation, one mutant is estimated at 2–4 s, so a first full run takes about 10–15 minutes on 8 cores. This is an estimate, not a measurement.

**Scope:**

- Use gremlins (Apache-2.0), pinned by version. Fall back to avito-tech/go-mutesting (MIT) if gremlins does not fit. Neither changes `go.mod`.
- Start with `internal/model`, the parsers in `internal/executor`, and `internal/diff`.
- Run locally and on a nightly schedule. In pull requests, run it only for changed files in those packages, and not as a required check at first.
- Turn survivors into tests. The target is zero surviving mutants in the anomaly rules and the health score.

**Done when:** a documented make target or workflow runs mutation testing for the listed packages, the nightly run publishes a summary, and the anomaly and health-score code has no surviving mutants.

### 69. Enforce package layering with depguard

**Size:** S (after tasks 39 and 41)

**Evidence:** the import graph is acyclic today, but nothing stops a lower layer from importing a higher one, for example `internal/model` importing `internal/collector`. Nothing stops library packages from importing `log` (task 60) either. `depguard` is built into the pinned golangci-lint.

**Scope:**

- After the package split (tasks 39 and 41), define the allowed imports for each layer.
- Forbid `log` in library packages, and `os/exec` outside the packages that own process execution.
- Enable the rules in `.golangci.yml` (task 37).

**Done when:** `depguard` rules exist for every layer, and CI fails on a violating import.

### 81. Make recommendations reviewable before applying them

**Size:** M

**Evidence:**

- `Recommendation` (`internal/model/types.go:548`) carries commands and append-to-file persistence commands. It has no current value, no proposed value as data, no rollback command, no risk level, and no note on whether a configuration file already sets the parameter.
- Drift between runtime sysctl values and persisted configuration (`/etc/sysctl.conf`, `/etc/sysctl.d`, `/run/sysctl.d`, `/usr/lib/sysctl.d`) is not detected, although such drift is a common finding after incidents and reboots.

**Scope:**

- For sysctl recommendations, add the current and proposed values, a revert command, a risk level, and the file that persists the parameter, if any. These are additive fields (see *Schema change candidates*).
- Report drift between runtime and persisted values without changing anything, following the systemd-sysctl precedence.
- Add `melisa recommendations <report.json> --script`, which prints a commented-out, reviewable shell script and never executes anything.
- Coordinate with tasks 14 and 61.

**Done when:** tests cover drift detection with fixture directories and the script output, and the documentation explains the review workflow.

### 83. Add scenario profiles

**Size:** M (after task 35)

**Evidence:** profiles differ by size (quick, standard, deep), but operators start from a symptom. `--focus` only filters Tier 2/3 collectors by category.

**Scope:**

- Once task 35 defines the core tool set, add scenario presets, for example `latency`, `memory`, `network`, `disk`, and `oom`. Each preset selects tools, durations, and the relevant rules.
- Describe the scenarios in the runbooks (task 82).
- Decide together with task 12 whether focus durations remain.

**Done when:** every scenario has tests for its collector set and is documented.

### 84. Evaluate a Prometheus textfile export

**Size:** S (decision)

**Evidence:** Melisa is deliberately not a daemon, yet teams run point-in-time tools from cron or systemd timers. No Melisa output format can be ingested by existing monitoring. The README roadmap lists OpenTelemetry export.

**Scope:**

- Evaluate the node_exporter textfile collector format for low-cardinality data: health, per-resource status, anomaly counts by rule and severity, collector failures.
- Compare it with OpenTelemetry export.
- Keep it an output format only, with no daemon.
- Record the decision and its reasons in *Tooling decisions*, and add a follow-up task if it is accepted.

**Done when:** the decision and its reasons are recorded.

### 90. Report hardware health

**Size:** M

**Evidence:**

- hwmon sensors (temperatures, fans) are not read. Task 75 covers CPU thermal throttle counters only.
- PCIe link degradation is not detected: current link speed and width against the maximum, for example a GPU or NIC running at x4 instead of x16. Advanced Error Reporting counters (`aer_dev_correctable`, `aer_dev_nonfatal`, `aer_dev_fatal`) are not read either. The GPU collector maps PCIe devices to NUMA nodes only.
- RDMA and InfiniBand port counters are not reported.

**Scope:**

- Read hwmon sensors with their thresholds.
- Read PCIe link status (`current_link_speed`, `current_link_width`, `max_link_*`) and AER counters for GPUs, NICs, and NVMe devices.
- Read RDMA port counters from `/sys/class/infiniband`.
- Add rules for over-temperature, degraded links, and AER errors.

**Done when:** fixture tests cover every source and rule, and the documentation is updated.

### 92. Extend CPU and scheduler detail

**Size:** M

**Evidence:**

- Hardware interrupts per device and CPU (`/proc/interrupts`) are not read; only softirqs are.
- Per-CPU run-queue wait without BCC (`/proc/schedstat`) is not read.
- CPU vulnerability mitigations (`/sys/devices/system/cpu/vulnerabilities/*`) are not reported, although they noticeably affect syscall-heavy workloads.
- C-state residency and idle drivers are not reported.
- Hardware performance counters (IPC, cache misses, branch misses) are available only through the BCC tool `llcstat`. The installer installs `perf`, but no collector uses it.

**Scope:**

- Parse `/proc/interrupts` as a two-point delta, and extend the IRQ imbalance rule (task 17) to hardware interrupts.
- Parse `/proc/schedstat`.
- Report mitigations and idle states.
- Evaluate reading hardware counters through `perf_event_open`, which needs privileges; record it as Tier 2 or Tier 3.

**Done when:** fixture tests cover the new sources, and the documentation is updated.

### 93. Read the history that already exists on the host

**Size:** M

**Evidence:**

- A point-in-time report cannot show what happened before the operator ran Melisa, yet hosts often keep that history already:
  - sysstat archives (`/var/log/sysstat/sa*` exist on the review host);
  - the journal;
  - atop logs.
- PSI parsing reads only `avg10` and `avg60` (`internal/collector/cpu.go:226`, `disk.go:142`, `memory.go:261`). It ignores `avg300` and the `total` stall counters, which would show pressure over the last minutes.

**Scope:**

- Read existing sysstat data through `sadf -j` via `CommandRunner` for a configurable recent window, without enabling or changing sysstat collection.
- Summarize recent journal error rates by unit.
- Parse PSI `avg300` and `total`.
- Add a "recent history" section to the text summary (task 71).
- Stay read-only and point-in-time: do not create history of Melisa's own.

**Done when:** tests with fixture sysstat JSON and journal output cover the history section, hosts without sysstat are reported as such, and the documentation explains the feature.

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

### 46. Resolve EN/RU documentation drift

**Priority:** 2

**Size:** M

**Evidence:**

- The RU documentation has 108k characters against 173k in EN. Several RU chapters are a fraction of the English text:

  | Chapter | RU size vs. EN |
  |---|---|
  | 13 | 18% |
  | 14 | 19% |
  | 02 | 22% (11 headings against 22) |
  | 15 | 22% |
  | 08 | 23% |
  | 06 | 25% |

- README states that all chapters are available in both languages, and AGENTS.md requires keeping them in sync.
- The educational chapters (Linux fundamentals, NUMA, production checklist) double the maintenance cost of every change.

**Scope:**

- Decide the policy: EN canonical with a full RU translation, or RU as a marked summary. State it in README and CONTRIBUTING.md.
- Bring the RU chapters in line with the policy, starting with chapters touched by Priority 1 and 2 tasks.
- Add a CI check that EN and RU have the same files and, for full translations, the same heading structure.
- Consider separating the product reference from the educational chapters.

**Done when:** the policy is documented, the RU tree follows it, and CI enforces the structure check.

### 82. Document least-privilege deployment and symptom runbooks

**Priority:** 2

**Size:** M

**Evidence:**

- README examples connect as `ssh root@server`, including the MCP configuration. There is no guidance for a dedicated user with a `sudoers` rule, or for an SSH key restricted to a forced command (`command="…",restrict` in `authorized_keys`).
- Nothing documents running on Kubernetes nodes, in containers, or on air-gapped hosts.
- The chapters read like a textbook. There are no symptom-based runbooks (high latency, OOM, packet drops, slow disk, CPU throttling) that map a symptom to a profile, a focus, and report fields.
- The README roadmap (Web UI, ROCm, OpenTelemetry, …) does not match this backlog.

**Scope:**

- Write a deployment guide:
  - a dedicated user with exact `sudoers` entries;
  - a forced-command SSH key for `melisa mcp`;
  - report file locations and permissions (task 70);
  - running on Kubernetes nodes (task 76);
  - installation on air-gapped hosts.
- Write the symptom runbooks in EN and RU.
- Replace the README roadmap with a short summary of *Direction for the next months*.

**Done when:** the guide and runbooks exist in both languages, the README examples no longer require root SSH, and the README roadmap matches the backlog.

## Schema change candidates

These changes affect the public JSON report, so AGENTS.md requires explicit approval before any of them is made. Each one requires a schema version bump and updates to `docs/schema/`, `testdata/`, examples, and the EN/RU documentation.

- A per-collector `status` (`ok`, `partial`, `failed`, `skipped`), plus a list of skipped collectors with reasons (today these are only logged). Supports task 10.
- Rate fields for softnet `time_squeeze` and TCP `PruneCalled`. Supports task 14.
- A numeric `anomalies[].value` and structured thresholds instead of formatted strings. This is a breaking change and requires a major schema version.
- Stable, documented event `details` keys per tool, instead of lowercased BCC column headers such as `laddr:lport` and `t>`. Supports task 57.
- Explicit numeric values: 112 of the 246 numeric fields in `internal/model/types.go` use `omitempty`, so a measured 0 is indistinguishable from "not measured". Values that may be absent would become nullable; measured zeros would always be emitted.
- Additive fields for the operations tasks, as candidates for schema 1.2.0:
  - the redaction level (task 70);
  - multi-sample statistics (task 73);
  - new host signals (tasks 74 and 75);
  - per-workload cgroup data (task 76);
  - applied rule overrides (task 80);
  - reviewable recommendation fields (task 81);
  - new fields from the coverage tasks 85–93.
- Absolute event timestamps. BCC events carry local `HH:MM:SS`, and the native collector formats `15:04:05` without a date or time zone, so correlating events with logs and metrics is error-prone. An RFC 3339 UTC timestamp with milliseconds would fix this.

## Repository settings applied on 2026-11-27

These settings were changed with `gh` from the maintainer's account. They are recorded here so that later tasks do not repeat them.

- Private vulnerability reporting enabled.
- Pull requests:
  - squash merge only, with the PR title as the commit title and the commit messages as the body;
  - head branches deleted on merge;
  - auto-merge allowed;
  - update-branch suggestions enabled.
- CodeQL default setup enabled for Actions, Go, and Python. The first run succeeded and opened three alerts, which task 18 tracks.
- Tag rulesets:
  - `Release tags: admin-only creation`: only repository admins can create `refs/tags/v*`;
  - `Release tags: immutable`: `v*` tags cannot be updated or deleted, and nobody can bypass this. If a broken tag must be removed, disable the ruleset temporarily.
- Head branches of the merged pull requests #5–#7 deleted. They can be restored from each pull request page.
- Not applied: secret scanning non-provider patterns and validity checks. The API accepted the request, but both settings remained `disabled` for this repository.

## Tooling decisions (2026-11-27)

These tools were evaluated and are not adopted. The reasons are recorded so that the tools are not re-evaluated without new information.

| Tool | Decision and reason | Replacement |
|---|---|---|
| unclebob/crapper, mutator, dryer | Not adopted. 0BSD, but about two months old with a single maintainer. They bootstrap a Python venv from PyPI on first run, write EDN for uml-viewer, and mutator needs a sibling `../crapper` checkout. | Task 67 (CRAP), task 68 (mutation testing), `dupl` in task 37 (duplicates) |
| unclebob/mutate4go, unclebob/crap4go | Not adopted: no license. mutate4go also writes a manifest footer into every source file it tests. | Tasks 67 and 68 |
| zimmski/go-mutesting | Not adopted: no commits since 2024-07. | avito-tech/go-mutesting, as the task 68 fallback |
| vladopajic/go-test-coverage | Not adopted: GPL-3.0. Coverage thresholds fit into the task 67 program. | Task 67 |
| uber-go/nilaway | Not as a required check: it needs a custom golangci-lint plugin build and has a noticeable false-positive rate. It can be tried as an experiment. | — |
| gtramontina/ooze | Not adopted: it needs a test file that imports it, which adds a module dependency. | Task 68 |

`deadcode` and actionlint were not run during the review, because session permissions blocked downloading new tools. Statements about their findings are expectations.

## Delivery notes

- Keep changes scoped and use Conventional Commits.
- Run `GOTOOLCHAIN=go1.27.4 make check` and `make build` for Go changes.
- For changes that affect collection or report generation, also run the real CLI as described in AGENTS.md and inspect per-collector errors.
- Confirm findings marked *inferred from code* with a reproduction or a regression test as part of the fix.
- Keep the structural refactors (tasks 34, 39, 41, 42) behavior-preserving and in separate commits from fixes. Schedule them so that they do not collide with Priority 1 changes to the same files.
- Repository setting changes are visible to everyone: apply them only with maintainer approval, and record them in *Repository settings applied on 2026-11-27* or in a new dated section.
- Do not claim privileged integration or validation suites passed unless they were explicitly approved and actually run.
- Remove generated binaries, `dist/`, reports, and temporary files before committing.
