# SLOs

Service Level Objectives for CAPP Knative workloads, covering four reliability
dimensions: availability, latency, cold-start time, and scaling responsiveness.

## Definitions

| # | SLI | SLO | Alert condition |
|---|-----|-----|-----------------|
| 1 | Synthetic probe success rate | 100 % of probes succeed | Any probe failure |
| 2 | p99 activator request latency | ≤ 1 s (5 m window) | p99 > 1 s for 5 m |
| 3 | p99 cold-start duration | ≤ 8 s (1 h window) | p99 > 8 s for 5 m |
| 4 | Scale lag (desired − actual pods) | ≤ 40 pods | Gap > 40 for 5 m |

All alerts evaluate every 5 minutes with a 5-minute pending period before firing.

## Rationale

| SLO | Why this threshold |
|-----|-------------------|
| Availability | The probe exercises the full path (routing → activation → response). Any failure means the workload cannot serve traffic. |
| Activator latency ≤ 1 s | The activator proxies requests during scale-from-zero. Above 1 s at p99 users experience unacceptable delays. |
| Cold start ≤ 8 s | Covers image pull + container init + readiness probe. Exceeding 8 s signals image bloat, registry issues, or resource pressure. |
| Scale lag ≤ 40 pods | Tolerates normal burst transients while catching sustained provisioning failures (node capacity, scheduling, quotas). |

## Metric sources

| Metric | Source |
|--------|--------|
| `kube_pod_owner`, `kube_pod_status_phase` | kube-state-metrics |
| `activator_request_latencies_bucket` | Knative Serving activator |
| `capp_cold_start_seconds` | CAPP benchmark runner |
| `autoscaler_desired_pods`, `autoscaler_actual_pods` | Knative Serving autoscaler |
