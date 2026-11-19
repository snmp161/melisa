# Example Report: Production Server Performance Analysis

> **melisa** | Profile: `deep` (60s) | Tier 1 + BCC Tools

## Summary

| Metric | Value | Status |
|--------|-------|--------|
| **Health Score** | **32 / 100** | 🔴 Critical |
| Uptime | 16 days | — |
| Kernel | 6.8.0-94-generic (Ubuntu 24.04) | ✅ |
| CPU | 8 cores, 55% util, **14.5% iowait** | ⚠️ |
| RAM | 20 GB / 31 GB (62%) | ✅ |
| Disk | **97.9% utilization**, **113.9 ms latency** | 🔴 |
| Load Average | 6.25 / 5.77 / 5.79 | ⚠️ |

### Anomalies

| Severity | Message |
|----------|---------|
| 🔴 CRITICAL | Disk utilization: **97.9%** |
| 🔴 CRITICAL | Disk avg I/O latency: **113.9 ms** |
| ⚠️ WARNING | CPU iowait: **14.5%** |
| ⚠️ WARNING | CPU PSI pressure: **13.0%** |
| ⚠️ WARNING | I/O PSI pressure: **17.5%** |

### USE Metrics

| Resource | Utilization | Saturation | Errors |
|----------|-------------|------------|--------|
| CPU | 55.3% | 0% | 0 |
| Disk | **97.9%** | **15%** | 0 |
| Memory | 61.6% | 0% | 0 |
| Network | 0% | **246%** | 0 |

---

## BCC Tool Data

### biolatency — Block I/O Latency Distribution (per-disk)

```
disk = sda (HDD, rotational):
     64 -  127 µs : ████  131
    128 -  255 µs : ████████████████████████████████████████  2565  ← bulk I/O
    256 -  511 µs : █████████  583
    512 - 1023 µs : ████  280
   1024 - 2047 µs : █  99
   8192 -16383 µs : █  81       ← HDD head seeking
  16384 -32767 µs :    20
  32768 -65535 µs :    12       ← heavy delays
  65536 -131ms    :    13       ← message broker fsync
 131072 -262ms    :     2       ← extreme tail latency
```

**P99 ≈ 65 ms, Max = 262 ms**. The long tail is caused by message broker fsync operations competing with container log I/O on the same HDD RAID.

### ext4slower — Slow Filesystem Operations (>1ms)

| Time | Process | PID | Type | Latency (ms) | File | Analysis |
|------|---------|-----|------|-------------|------|----------|
| 08:36:14 | dockerd | 1095 | W | 25.33 | overlay2-log | Docker writing container JSON logs |
| 08:36:14 | flb-pipeline | 3132368 | R | 7.50 | overlay2-log | Log shipper reading same file |
| 08:36:15 | kafka-raft-io | 5929 | S (fsync) | **136.49** | kafka-commit.log | **Broker fsync — blocks entire I/O queue** |
| 08:36:15 | dockerd | 1095 | W | **114.25** | overlay2-log | Docker log write stalled behind fsync |
| 08:36:15 | flb-pipeline | 3132368 | R | **115.79** | overlay2-log | Log reader stalled behind fsync |
| 08:36:15 | kafka-raft-io | 5929 | S (fsync) | **261.27** | kafka-commit.log | **Worst-case fsync: 261 ms** |

**Clear pattern**: message broker fsync → blocks HDD → cascading stalls for dockerd and log shippers.

### runqlat — CPU Run Queue Latency Distribution

```
      0 -    1 µs : ████████████████████████████████████████  363557  ← most tasks scheduled fast
     64 -  127 µs : █  10770
    512 - 1023 µs :     3164
   1024 - 2047 µs :     1703    ← 1-2ms CPU wait
   2048 - 4095 µs :     1045
  32768 -65535 µs :      154    ← 32-65ms scheduling delay
```

**154 tasks** waited 32-65 ms for CPU in a 10-second window. Caused by 8 cores being saturated at 55% average (with bursts from containers using 265% CPU total).

### cachestat — Page Cache effectiveness

```
    HITS   MISSES  DIRTIES HITRATIO
       0        0     1815    0.00%     ← zero cache hits
       0        0     1769    0.00%
       0        0     1908    0.00%
```

**0% cache hit ratio** with ~1800 dirty pages/sec. All I/O is write-dominant — page cache provides no read caching benefit.

---

## Root Cause Chain

```
  Message Broker fsync (136-261 ms per commit)
          │
          ▼
  HDD I/O queue saturated (97.9% utilization)
          │
          ├── dockerd: writing container logs (25-114 ms wait)
          │          │
          │          └── Container stdout blocks → app stalls
          │
          ├── Log shipper: reading container logs (5-115 ms wait)
          │
          └── CPU iowait rises to 14.5%
                     │
                     ▼
              Tasks wait in run queue (up to 65 ms)
                     │
                     ▼
              TCP responses delayed → retransmissions (246% saturation)
                     │
                     ▼
              🔴 Application latency increases
```

---

## Recommendations

### Immediate (reduce latency 3-5x)

| # | Action | Command | Effect |
|---|--------|---------|--------|
| 1 | Limit Docker log size | `{"log-opts": {"max-size": "10m", "max-file": "3"}}` in daemon.json | Reduce dockerd + log shipper I/O |
| 2 | Switch I/O scheduler to BFQ | `echo bfq > /sys/block/sda/queue/scheduler` | Prioritize short I/O over broker fsync |
| 3 | Reduce broker fsync frequency | `log.flush.interval.ms=5000` | Fewer fsync = less I/O blocking |

### Strategic

| # | Action | Rationale |
|---|--------|-----------|
| 4 | Move broker data to SSD | fsync on SSD: <1ms vs 96-261ms on HDD |
| 5 | Tune `vm.dirty_ratio=5` | Faster writeback, smaller I/O queue |
| 6 | Consider additional CPU cores | 8 cores at 55% avg with 265% container burst |

---

*This report was generated by `melisa collect --profile deep --ai-prompt`. The `ai_context.prompt` field in the JSON report contains a ready-to-use prompt for AI-driven analysis.*
