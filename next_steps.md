# Next Steps

## Phase 1 status

Implemented:
- Persist per-connection summaries keyed by `pid + protocol + domain`.
- Attribute network activity to the current process name when available.
- Infer a coarse protocol from the remote port and show it in `summary`.

Left out on purpose:
- Packet capture.
- TLS handshake parsing.
- Connection start/end timestamps and durations.
- Cross-user comparison or upload APIs.

## Recommended next phases

### Phase 2: connection lifecycle
- Track when a `pid + remote endpoint` is first seen and last seen.
- Persist connection duration and active/closed state.
- Surface long-lived background connections separately from bursty traffic.

### Phase 3: per-process network patterns
- Count unique domains per process.
- Flag processes contacting an unusually high number of domains.
- Add process-level network totals so `summary` can answer "which app is talking the most?"

### Phase 4: richer protocol and TLS metadata
- Capture TLS handshake details where feasible: SNI, ALPN, TLS version, certificate issuer.
- Distinguish HTTPS over TCP from QUIC/HTTP3 more reliably.
- Store transport/protocol metadata without requiring payload inspection.

### Phase 5: telemetry heuristics
- Track connection frequency over longer windows.
- Identify noisy background services and repeated low-volume polling patterns.
- Add simple heuristics for "possible telemetry" versus interactive traffic.

### Phase 6: export and comparison prep
- Write a stable event/schema layer intended for future upload.
- Separate privacy-sensitive fields from aggregate counters.
- Define opt-in boundaries before any remote API work.
