# capp-monitoring

Status page and automated scale benchmarking for [CAPP](https://github.com/dana-team/container-app-operator) and Knative. Ships two components via a single Helm umbrella chart:

| Component | What it does |
|---|---|
| **status-page** | Go server that polls Kubernetes Deployment readiness and serves a live status page + JSON API + Prometheus metrics |
| **benchmarks** | Automated Knative scale-behaviour CronJob |

## Quick start

### Helm

```bash
helm install capp-monitoring charts/capp-monitoring \
  --set benchmarks.knativeService=my-knative-svc
```

Each sub-chart can be toggled independently:

```bash
# Benchmarks only
helm install capp-monitoring charts/capp-monitoring \
  --set status-page.enabled=false
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

## Benchmarks

The benchmark runner image (`ghcr.io/dana-team/capp-benchmark-runner`) packages **hey** and **kubectl** on UBI9-minimal. It drives the scale-behaviour CronJob that measures cold-start latency, scale-out delay, and time-to-target-pods, pushing results to VictoriaMetrics.

Required Helm values when `benchmarks.enabled=true`:

```yaml
benchmarks:
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
