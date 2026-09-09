# capp-monitoring

Benchmarking suite for [CAPP](https://github.com/dana-team/container-app-operator) and Knative. Ships as a single Helm chart with k6, iter8, and vegeta load-test jobs as Kubernetes CronJobs.

## Quick start

### Helm

```bash
helm install capp-monitoring charts/capp-monitoring \
  --set targetUrl=http://my-app.capp-system.svc.cluster.local \
  --set cappName=my-capp
```

## SLOs

SLOs for CAPP Knative workload reliability are defined in [`docs/slo.md`](docs/slo.md).

## Benchmarks

Each benchmark runs as a Kubernetes CronJob. Enable them independently via Helm values. Metrics are pushed to VictoriaMetrics when `victoriametrics.importUrls` is set. Cold-start time to first byte can alternatively use `pushgatewayUrl`.

### Scale (`scale.enabled`)

Measures Knative autoscaling SLOs using k6 for load generation. Requires `testCapp` at zero pods (`minReplicas: 0`). Conflicts with iter8 if both are enabled, because iter8 targets the same Capp.

| Metric | Description |
|--------|-------------|
| `capp_cold_start_seconds` | Time to first byte (TTFB) of the first request after genuine scale-to-zero |
| `capp_scale_delay_seconds` | Load start → scale-out past the first pod (>1 ready) |
| `capp_time_to_scale_seconds` | Load start → target pods Ready |
| `capp_scale_test_success` | 1 if target reached within timeout, 0 otherwise |

### Latency (`latency.enabled`)

Measures sustained latency SLOs using k6 — p99 < 500 ms, error rate < 1%, plus cold-start time to first byte. Uses its own dedicated Capp (`latency.testCapp`, `minReplicas: 1`) so it does not warm the scale Capp. Safe to enable alongside scale.

| Metric | Description |
|--------|-------------|
| `capp_cold_start_ttfb_seconds` | Time to first byte after patching the Capp to zero replicas |
| `http_req_duration` | Request latency (p99 < 500 ms threshold) |
| `http_req_failed` | Error rate (< 1% threshold) |

### iter8 (`iter8.enabled`)

Formal HTTP SLO pass/fail assessment using iter8. Sends traffic to `testCapp`. Conflicts with scale if both are enabled; to run both, disable iter8 or point it at a different target via `targetUrl`.

| SLO | Threshold |
|-----|-----------|
| Mean latency | ≤ 200 ms |
| p99 latency | ≤ 500 ms |
| Error rate | ≤ 1% |

### On-demand (`oneshot.enabled`)

Runs a single benchmark as a Helm hook on `install`/`upgrade`. Set `oneshot.type` to one of: `k6-latency`, `k6-throughput`, `cold-start`, `iter8`. `k6-latency` and `cold-start` use `latency.testCapp` (the Capp is created even when `latency.enabled` is false) so they do not warm the scale Capp. `k6-throughput` and `iter8` target `testCapp` and conflict with scale.

## Development

```bash
make helm-lint    # helm lint charts/capp-monitoring
make docker-build # build benchmark runner image
```
