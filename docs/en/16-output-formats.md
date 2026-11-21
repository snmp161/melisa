# Chapter 16: Output Formats

## Overview

melisa produces three output types: structured JSON, SVG flame graphs, and AI prompts. The output package (`internal/output/`) handles formatting and writing.

## Source Files: output/ (5 files)

| File | Lines | Purpose |
|------|-------|---------|
| `json.go` | ~106 | JSON serialization to file or stdout |
| `flamegraph.go` | ~106 | SVG flame graph from folded stacks |
| `ai_prompt.go` | ~103 | AI/LLM analysis prompt (see Chapter 13) |
| `progress.go` | ~53 | Collection progress display |

## JSON Output

### WriteJSON()

```go
func WriteJSON(report *model.Report, path string) error {
    data, err := json.MarshalIndent(report, "", "  ")

    if path == "" || path == "-" {
        os.Stdout.Write(data)
    } else {
        // Atomic write: write to temp file, then rename
        tmpFile := path + ".tmp"
        os.WriteFile(tmpFile, data, 0644)
        os.Rename(tmpFile, path)
    }
}
```

**Atomic write pattern**: Writing to a temporary file and renaming prevents partial writes if melisa is killed mid-write. `os.Rename` is atomic on the same filesystem.

### JSON Schema

The public report format is versioned as `1.1.0`. Its canonical machine-readable definition uses JSON Schema Draft 2020-12:

- [Report JSON Schema 1.1.0](../schema/report-1.1.0.schema.json)
- [Sanitized example report](../../testdata/report_1.1.0.json)

The schema describes required fields, optional `omitempty` fields, nested collector data, and summary objects. `Result.data` contains one of the documented typed payloads or `null`; BCC results commonly use `null` because their measurements are stored in `histograms`, `events`, or `stacks`. Fields such as `metadata.focus_areas`, `metadata.capabilities`, and `summary.anomalies` can also be `null` when their Go slices are nil.

Ordinary unprivileged tests compile the schema and validate both the captured example and output written by `WriteJSON`. Collection itself does not perform runtime schema validation, so schema validation remains optional for consumers and does not change partial-report behavior.

## Flame Graph Generator

### WriteFlamegraph()

```go
func WriteFlamegraph(stacks []model.StackTrace, outputPath string) error {
    // 1. Convert stacks to folded format
    //    "main;handleRequest;db.Query 42"
    //    "main;handleRequest;json.Marshal 15"
    // 2. Generate SVG using flame graph algorithm
    // 3. Write to output file
}
```

Flame graphs visualize CPU profiling data:

```
┌──────────────────────────────────────────────────────────┐
│ root                                                      │
├──────────┬───────────────────────────┬───────────────────┤
│ goroutine│ goroutine 2               │ goroutine 3       │
│ 1        │                           │                   │
│          ├───────────┬───────────────┤                   │
│          │handleReq  │other          │                   │
│          ├─────┬─────┤               │                   │
│          │json │ db  │               │                   │
│          │     │Query│               │                   │
└── ── ── ─┴─────┴─────┴── ── ── ── ──┴── ── ── ── ── ──┘
Width = proportion of CPU time
```

- Wide bars = hot functions consuming lots of CPU
- Vertical stacking = call chain
- Read bottom-to-top: caller → callee

## Progress Reporter

```go
type ProgressReporter struct {
    quiet bool
}

func (p *ProgressReporter) Start(name string) {
    if !p.quiet {
        fmt.Fprintf(os.Stderr, "  %-30s ", name)
    }
}

func (p *ProgressReporter) Done(duration time.Duration) {
    if !p.quiet {
        fmt.Fprintf(os.Stderr, "✓ %s\n", duration.Round(time.Millisecond))
    }
}
```

Output during collection:
```
melisa v0.2.0 — collecting with standard profile (30s)
  cpu_utilization               ✓ 1.003s
  memory_info                   ✓ 12ms
  disk_io                       ✓ 1.001s
  network_stats                 ✓ 45ms
  process_top                   ✓ 1.005s
  container_info                ✓ 3ms
  system_info                   ✓ 218ms
```

---

*Next: [Chapter 17 — Appendix](17-appendix.md)*
