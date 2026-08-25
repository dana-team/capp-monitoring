# capp-monitoring

Benchmarking suite for [CAPP](https://github.com/dana-team/container-app-operator) and Knative. Ships as a single Helm umbrella chart with k6, iter8, and vegeta load-test jobs as Kubernetes CronJobs.

## Quick start

### Helm

```bash
helm install capp-monitoring charts/capp-monitoring \
  --set benchmarks.targetUrl=http://my-app.capp-system.svc.cluster.local \
  --set benchmarks.cappName=my-capp
```

## SLOs

SLOs for CAPP Knative workload reliability are defined in [`docs/slo.md`](docs/slo.md).

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
  cappName: "my-capp"
```

## Development

```bash
make helm-lint    # helm lint charts/capp-monitoring
make docker-build # build benchmark runner image
```
