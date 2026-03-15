# dialtone-watcher

`dialtone-watcher` is a small Go agent for macOS and Linux that watches what a machine is doing without pretending to be mystical about it.

It samples:

- running processes
- CPU and memory usage
- outbound and inbound network endpoints
- coarse protocol guesses like `HTTPS`, `DNS`, `QUIC`, and `Postgres`
- a normalized hardware profile for the machine

It stores a local summary, and it can periodically POST compact rollups to Dialtone so the data in the demo can become real:

- Demo: <https://dialtoneapp.com/demo>
- Developer: Andrew Arrow <https://github.com/andrewarrow/>

This project is opinionated in a useful way: collect enough to make a machine profile interesting, but not so much that the backend gets crushed or the user has to wonder whether the app is doing secret goblin behavior.

## Why This Exists

Most tools do one slice of this problem:

- Activity Monitor and `htop` show local processes.
- Wireshark and Little Snitch show network activity.
- Geekbench compares hardware.
- Enterprise EDR tools collect machine telemetry, but behind a corporate wall.

`dialtone-watcher` tries to combine those ideas into something much more human:

> What apps run on this machine all day, where do they connect, what looks normal, and what looks weird compared to similar hardware?

That is the core of the Dialtone demo.

## What It Does Today

The watcher runs as a background process and keeps a compact in-memory model of the current machine.

It currently supports:

- hardware profile collection with `gopsutil`
- process polling
- per-domain network traffic summaries
- per-process connection grouping by `pid + protocol + domain`
- reverse lookup of public IPs, cached in memory
- local machine identity persisted on disk
- periodic upload of bounded JSON summaries to `https://dialtoneapp.com/api/v1/watcher`

The local CLI is intentionally plain:

```bash
./dialtone-watcher start
./dialtone-watcher stop
./dialtone-watcher summary
./dialtone-watcher help
```

When you run `summary`, you get a concise machine report instead of a 900-line process dump.

## Example Summary

An early prompt in this repo captured the shape we were aiming for:

```text
Watcher running: true
Watcher pid: 96964
Polls completed: 3
Tracked processes: 976
Tracked domains: 13
Hardware: aas-MacBook-Pro.local on darwin (Apple M4 Max, 36.0 GB RAM, 14 logical cores)
Interesting processes:
  1. pid=83479 name=com.apple.Virtualization.VirtualMachine cpu=5.67% rss=2705.3 MB seen=3 polls
  2. pid=70214 name=plugin-container cpu=5.28% rss=1999.4 MB seen=3 polls
Interesting domains:
  1. domain=17.57.144.121 rx=70.2 KiB tx=165.8 KiB seen=3 polls
```

That led directly to two improvements:

- top-6 process and domain summaries instead of a single "interesting" item
- async IP-to-domain resolution with caching so summary output stays fast

## Demo Shape

The current upload format is designed to support the Dialtone demo view, especially the fleet tables and single-machine profile.

Some of the demo data from `~/dev/dialtoneapp/src/pages/Demo/index.jsx`, translated into markdown:

### Fleet Overview

| Metric | Value | Detail |
| --- | ---: | --- |
| Active machines | 20,000 | 30-day rolling sample |
| Connections classified | 1.84B | process + domain + protocol |
| Process snapshots | 412M | CPU, memory, runtime state |
| Traffic attributed | 96.2% | mapped back to an app or service |

### Top Destinations

| Domain | Machines | Fleet Share | Traffic | Trend |
| --- | ---: | ---: | ---: | ---: |
| googlevideo.com | 14,882 | 74.4% | 318.2 TB | +12% |
| cloudflare.com | 13,906 | 69.5% | 281.7 TB | +9% |
| apple.com | 11,104 | 55.5% | 92.4 TB | +4% |
| github.com | 7,262 | 36.3% | 61.8 TB | +18% |
| openai.com | 5,418 | 27.1% | 33.7 TB | +31% |

### Top Processes

| Process | Machines | Avg CPU | Avg Memory | Persistence |
| --- | ---: | ---: | ---: | --- |
| Google Chrome Helper | 9,844 | 11.8% | 1.3 GB | high |
| Slack Helper | 7,408 | 2.1% | 428 MB | very high |
| Docker Desktop | 4,222 | 6.4% | 2.7 GB | high |
| Cursor | 2,861 | 8.7% | 1.0 GB | medium |
| Telegram | 2,513 | 0.7% | 392 MB | very high |

The watcher’s upload payload is deliberately smaller than those tables imply. Each machine sends a bounded summary window, and the backend does the fleet math.

## How The Agent Uploads Data

Uploads are compact rollups over a period of time, not a firehose of every poll forever.

The payload includes:

- `schema_version`
- `sent_at`
- `period.started_at`
- `period.ended_at`
- `period.duration_seconds`
- `period.polls`
- machine metadata
- normalized hardware metadata
- summary counts and total bytes for the window
- top processes for the window
- top domains for the window
- top connections for the window

The request includes:

- HTTP header: `machine_id`
- endpoint: `https://dialtoneapp.com/api/v1/watcher`

Example shape:

```json
{
  "schema_version": 1,
  "sent_at": "2026-03-15T17:05:00Z",
  "period": {
    "started_at": "2026-03-15T17:00:00Z",
    "ended_at": "2026-03-15T17:05:00Z",
    "duration_seconds": 300,
    "polls": 60
  },
  "machine": {
    "hostname": "aas-MacBook-Pro.local",
    "os": "darwin",
    "platform": "macOS",
    "platform_version": "15.x",
    "kernel_version": "24.x",
    "hardware": {
      "cpu": {
        "model": "Apple M4 Max",
        "model_normalized": "apple_m4_max",
        "physical_cores": 14,
        "logical_cores": 14,
        "frequency_mhz": 0
      },
      "memory": { "total_gb": 36.0 },
      "disk": { "total_gb": 0 }
    }
  },
  "summary": {
    "running": true,
    "poll_count": 60,
    "tracked_process_count": 980,
    "tracked_domain_count": 24,
    "tracked_connection_count": 24,
    "total_rx_bytes": 11264000,
    "total_tx_bytes": 786432
  },
  "processes": [
    {
      "pid": 61421,
      "name": "firefox",
      "command": "/Applications/Firefox.app/Contents/MacOS/firefox",
      "average_cpu_percent": 2.57,
      "peak_cpu_percent": 7.11,
      "average_memory_rss_mb": 843.0,
      "peak_memory_rss_mb": 912.4,
      "polls_seen": 60
    },
    {
      "pid": 83479,
      "name": "com.apple.Virtualization.VirtualMachine",
      "command": "/System/Library/Frameworks/Virtualization.framework/...",
      "average_cpu_percent": 5.93,
      "peak_cpu_percent": 11.26,
      "average_memory_rss_mb": 6900.0,
      "peak_memory_rss_mb": 7024.2,
      "polls_seen": 60
    }
  ],
  "domains": [
    {
      "domain": "104.16.132.229",
      "display_name": "cloudflare.com",
      "rx_bytes": 11239424,
      "tx_bytes": 51200,
      "polls_seen": 18
    },
    {
      "domain": "microsoft.com",
      "rx_bytes": 997171,
      "tx_bytes": 146944,
      "polls_seen": 16
    }
  ],
  "connections": [
    {
      "pid": 61421,
      "process_name": "firefox",
      "domain": "104.16.132.229",
      "display_name": "cloudflare.com",
      "protocol": "HTTPS",
      "rx_bytes": 11239424,
      "tx_bytes": 51200,
      "polls_seen": 18
    },
    {
      "pid": 70214,
      "process_name": "com.docker.backend",
      "domain": "microsoft.com",
      "protocol": "HTTPS",
      "rx_bytes": 997171,
      "tx_bytes": 146944,
      "polls_seen": 16
    }
  ]
}
```

In the actual implementation, those arrays are bounded to keep uploads predictable:

- `processes`: top 12
- `domains`: top 20
- `connections`: top 20

The interval is configurable:

- production default: `15m`
- test mode default: `15s`

Environment variables:

```bash
DIALTONE_WATCHER_UPLOAD_URL
DIALTONE_WATCHER_UPLOAD_INTERVAL
DIALTONE_WATCHER_UPLOAD_TIMEOUT
DIALTONE_WATCHER_DISABLE_UPLOAD
DIALTONE_WATCHER_HOME
```

## Privacy, Explained Like An Adult

If you are privacy-sensitive, this section is the important one.

The project is intentionally inspectable:

- the local summary is written as JSON in the watcher home directory
- the machine ID is stored on disk in `machine-id.json`
- the upload payload is assembled in `internal/watcher/upload.go`
- the on-disk summary model is in `internal/watcher/state.go`
- process and network collection live in `internal/watcher/*.go`

What is collected locally:

- hostname
- OS, platform, kernel version
- CPU model and core counts
- total memory and disk estimates
- process name
- process command line
- process CPU percent and RSS memory
- per-domain RX/TX byte totals
- per-connection groups keyed by PID, protocol, and domain
- cached reverse DNS results for public IPs

What is sent remotely in the current implementation:

- machine metadata and normalized hardware profile
- aggregate counts for the upload period
- top process summaries for the period
- top domain summaries for the period
- top connection summaries for the period

What is not currently implemented:

- packet payload capture
- TLS handshake parsing
- full browsing history reconstruction
- keystroke logging, screenshots, or content capture

You should still treat process names, domains, and command lines as sensitive telemetry. The point is not "trust us, probably fine." The point is that you can read the code and know exactly what happens.

## How It Got Built

This repo is unusually honest because the prompt trail is still here in `prompts/`.

The rough sequence:

1. Start with a simple CLI that can `start`, `stop`, and `summary`.
2. Poll processes and keep an in-memory hardware profile.
3. Expand from one "interesting process" to a top list.
4. Add network observation and domain summaries.
5. Resolve IPs back to hostnames and cache them.
6. Add machine identity persistence.
7. Define a compact upload schema that could scale to 20,000 machines.

There are a few good moments in that history.

From the very first prompt:

> keep `main.go` in `./` do not make `cmd/` package

Reasonable. Violently anti-overengineering. Good instinct.

From the summary refinement prompt:

> don't list every pid. Just the main interesting one.

This was the correct product constraint. Raw dumps feel productive right up until a human has to read them.

From the reverse-lookup prompt:

> do this in background thread safe so that when summary is called it's fast

Also correct. The best CLI output is the one that does not stall while trying to be clever.

From the identity discussion:

I initially had the shape of a more elaborate custom identity story in mind. Andrew pushed the project back toward simpler primitives. The best example is UUID generation: instead of writing unnecessary custom logic, the repo uses Google’s UUID library. That is the right kind of correction. If the standard library or a stable dependency solves it, take the win and move on.

## Human + Machine Collaboration Notes

Andrew Arrow set the product bar here:

- keep the CLI simple
- make the summary readable
- collect enough signal to be useful
- think ahead to fleet comparison without turning the agent into spyware

My job in that collaboration was mostly to turn those constraints into working code:

- structure the watcher service
- add platform-specific network collectors
- keep state persisted cleanly
- design the upload payload so the backend can aggregate it efficiently
- add tests around payload shape and posting behavior

It was a productive pattern:

- human decides what matters
- machine writes the plumbing
- human deletes nonsense before it becomes architecture

That is, frankly, how more software should be built.

## Running It

Build:

```bash
go build .
```

Start the watcher:

```bash
./dialtone-watcher start
```

Inspect the current local summary:

```bash
./dialtone-watcher summary
```

Stop it:

```bash
./dialtone-watcher stop
```

Linux notes and Docker-based Linux test steps are in [`linux.md`](/Users/aa/os/dialtone-watcher/linux.md).

## What’s Next

The next useful layers are already sketched in [`next_steps.md`](/Users/aa/os/dialtone-watcher/next_steps.md):

- connection lifecycle tracking
- process-level network totals
- richer protocol and TLS metadata
- telemetry heuristics
- cleaner export/comparison boundaries

If this grows into a serious comparison product, the hard part will not be collecting more data. The hard part will be choosing what not to collect.
