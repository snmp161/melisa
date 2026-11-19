# Quick Start

From zero to diagnosis in 2 minutes.

## 1. Install

```bash
# One-liner (Linux amd64/arm64)
curl -sSL https://melisa.dev/install | sh

# Or build from source
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o melisa ./cmd/melisa/
sudo mv melisa /usr/local/bin/
```

Prebuilt Linux packages are available from [GitHub Releases](https://github.com/snmp161/melisa/releases): DEB for Debian/Ubuntu and RPM for AlmaLinux/Rocky Linux/RHEL, both for amd64 and arm64. Packages install the binary as `/usr/bin/melisa`.

## 2. Run Your First Scan

```bash
# Quick health check (10 seconds, no BCC tools needed)
sudo melisa collect --profile quick -o report.json
```

You'll see:

```
melisa v0.4.1 | profile=quick | duration=10s

Tier 1 (procfs)  ████████████████████████████████████████ 8/8   2.1s
Tier 2 (BCC)     ████████████████████████████████████████ 4/4  10.3s

Health Score:  68 / 100  ⚠️
Anomalies:     cpu_utilization CRITICAL (98.7%)
               load_average WARNING (3.2x CPUs)
Recommendations: 2

Report saved to report.json
```

## 3. Read the Results

```bash
# Health score (0-100)
jq '.summary.health_score' report.json

# What's wrong?
jq '.summary.anomalies[]' report.json

# How to fix it
jq '.summary.recommendations[] | {type, title, commands}' report.json
```

## 4. Apply Fixes

Recommendations include copy-paste commands:

```bash
# Example: melisa recommends enabling BBR
sysctl -w net.core.default_qdisc=fq
sysctl -w net.ipv4.tcp_congestion_control=bbr
```

## 5. Verify the Fix

```bash
# Run again and compare
sudo melisa collect --profile quick -o after.json
melisa diff report.json after.json
```

The diff shows what improved, what regressed, and health score delta.

## What's Next?

| Goal | Command |
|------|---------|
| Full analysis with all 67 BCC tools | `sudo melisa collect --profile standard -o report.json` |
| Deep dive (stacks, memleak, biotop) | `sudo melisa collect --profile deep -o report.json` |
| Focus on network only | `sudo melisa collect --profile standard --focus network -o net.json` |
| Profile a specific process | `sudo melisa collect --profile standard --pid 12345 -o app.json` |
| Profile a container | `sudo melisa collect --profile standard --cgroup /sys/fs/cgroup/system.slice/nginx.service -o cg.json` |
| Install BCC tools (first time) | `sudo melisa install` |
| Use with Claude/Cursor (MCP) | `melisa mcp` → [MCP setup guide](13-ai-integration.md) |
| Check system capabilities | `melisa capabilities` |

## Collection Profiles

| Profile | Duration | What runs | Best for |
|---------|----------|-----------|----------|
| **quick** | 10s | Tier 1 + 4 essential BCC tools | Health checks, CI gates |
| **standard** | 30s | All Tier 1 + all 67 BCC tools | Regular diagnostics |
| **deep** | 60s | Everything + memleak, biostacks, wakeuptime | Root cause analysis |

## CLI Reference

```
melisa collect   Collect metrics and produce JSON report
  --profile       quick|standard|deep (default: standard)
  --focus         cpu,memory,disk,network,stacks (comma-separated)
  --pid           Filter to specific PID
  --cgroup        Filter to cgroup path
  --duration      Override profile duration (e.g., 15s, 1m)
  --ai-prompt     Include AI analysis prompt in output
  --output, -o    Output file (- for stdout)
  --quiet, -q     Suppress progress output
  --verbose, -v   Debug logging

melisa diff      Compare two reports
  <baseline.json> <current.json>
  --output, -o    Output diff file

melisa install   Install BCC tools and dependencies
  --dry-run       Show what would be installed

melisa mcp       Start MCP server (stdio JSON-RPC)

melisa capabilities  Show available tools and kernel features
```

---

*Next: [Introduction — How melisa Works](index.md)*
