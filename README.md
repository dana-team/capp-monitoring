# capp-monitoring

Monitoring, observability, and benchmarking suite for [CAPP](https://github.com/dana-team/container-app-operator) and Knative. Ships three components via a single Helm umbrella chart:

| Component | What it does |
|---|---|
| **status-page** | Go server that polls Kubernetes Deployment readiness and serves a live status page + JSON API + Prometheus metrics |
| **observability** | Grafana dashboard and alert rules for CAPP SLOs; ServiceMonitors for Prometheus scraping |
| **benchmarks** | k6, iter8, and vegeta load-test jobs as Kubernetes CronJobs |

## Quick start

### Helm

```bash
helm install capp-monitoring charts/capp-monitoring \
  --set benchmarks.targetUrl=http://my-app.capp-system.svc.cluster.local \
  --set benchmarks.knativeService=my-knative-svc \
  --set observability.grafana.alerts.datasourceUid=<your-prometheus-uid>
```

Each sub-chart can be toggled independently:

```bash
# Observability only
helm install capp-monitoring charts/capp-monitoring \
  --set status-page.enabled=false \
  --set benchmarks.enabled=false
```

### Docker (status server only)

```bash
docker build -f docker/Dockerfile.status-server -t capp-status-server .
docker run --rm -p 8080:8080 capp-status-server
```

> Requires in-cluster Kubernetes credentials. Use `make docker-build` to build the benchmark runner image instead.

## Status server

The server runs on port `8080` (override with `PORT` env var).

### Endpoints

| Path | Description |
|---|---|
| `GET /` | Static status page (HTML, embedded in binary) |
| `GET /api/status` | JSON health summary |
| `GET /metrics` | Prometheus metrics |

### `/api/status` response

```json
{
  "overall": "operational",
  "components": [
    { "name": "CAPP Backend API", "group": "core", "status": "operational" },
    { "name": "cert-manager",     "group": "infrastructure", "status": "degraded", "message": "1/2 replicas ready" }
  ]
}
```

`overall` is the worst status across all components. Possible values: `operational`, `degraded`, `down`.

### Prometheus metric

```
capp_component_up{component="<name>", group="<group>"} 1|0
```

`1` = operational, `0` = degraded or down.

### Monitored components

| Name | Group | Default namespace |
|---|---|---|
| CAPP Backend API | core | `capp-platform-system` (`NS_CAPP`) |
| CAPP Frontend | core | `capp-platform-system` (`NS_CAPP`) |
| Knative Serving | core | `knative-serving` (`NS_KNATIVE`) |
| Container-App-Operator | core | `container-app-operator-system` |
| cert-manager | infrastructure | `cert-manager` (`NS_CERT_MANAGER`) |
| logging-operator | infrastructure | `logging-operator` (`NS_LOGGING`) |
| nfspvc-operator | infrastructure | `nfspvc-operator` (`NS_NFSPVC`) |
| provider-dns | infrastructure | `provider-dns` (`NS_PROVIDER_DNS`) |
| cert-external-issuer | infrastructure | `cert-manager` (`NS_CERT_MANAGER`) |

Namespaces are overridable via environment variables shown in parentheses.

## Observability

### Grafana alerts (SLOs)

| Alert | Severity | Threshold |
|---|---|---|
| CAPP Availability Warning | warning | < 99.9% over 1 h |
| CAPP Availability Critical | critical | < 99.5% over 1 h |
| CAPP p99 Latency Warning | warning | > 500 ms over 5 m |
| CAPP p99 Latency Critical | critical | > 1 s over 5 m |
| Knative Cold-start p99 Warning | warning | > 3 s over 1 h |
| Knative Cold-start p99 Critical | critical | > 8 s over 1 h |
| Autoscaler Scale Lag Warning | warning | desired − actual > 30 pods for 5 m |
| Autoscaler Scale Lag Critical | critical | desired − actual > 60 pods for 5 m |
| CAPP Component Down | critical | `capp_component_up < 1` for 2 m |

Alert source: [`grafana/alerts.yaml`](grafana/alerts.yaml). Dashboard source: [`grafana/dashboard.json`](grafana/dashboard.json).

Set `observability.grafana.alerts.datasourceUid` in `values.yaml` to your Grafana Prometheus datasource UID before deploying.

## Benchmarks

The benchmark runner image (`ghcr.io/dana-team/capp-benchmark-runner`) packages:

- **k6** v0.52.0 — throughput and latency scripts in [`benchmarks/k6/`](benchmarks/k6/)
- **vegeta** v12.13.0 — targets in [`benchmarks/vegeta/targets.txt`](benchmarks/vegeta/targets.txt)
- **iter8** v0.17.3 — SLO validation experiment in [`benchmarks/iter8/experiment.yaml`](benchmarks/iter8/experiment.yaml)
- **hey**, **kubectl** — cold-start TTFB script in [`benchmarks/k6/cold-start.sh`](benchmarks/k6/cold-start.sh)

Required Helm values when `benchmarks.enabled=true`:

```yaml
benchmarks:
  targetUrl: "http://my-app.capp-system.svc.cluster.local"
  knativeService: "my-knative-svc"
```

## Development

```bash
make build        # compile binary to ./capp-status-server
make test         # go test -v -race ./...
make lint         # golangci-lint run ./...
make helm-lint    # helm lint charts/capp-monitoring
```

Run a single test package:

```bash
go test -v -run <TestName> ./internal/checker/
go test -v -run <TestName> ./internal/server/
```

Tests use `controller-runtime/pkg/client/fake` — no cluster required.
