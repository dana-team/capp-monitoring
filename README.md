# capp-monitoring

Monitoring, observability, and **Knative autoscaling SLO benchmarking** for [CAPP](https://github.com/dana-team/container-app-operator) and Knative. The current focus is measuring autoscaling behaviour against SLOs and publishing the results to **VictoriaMetrics** so SRE can build dashboards — designed to run in **air-gapped OpenShift**.

Ships as a single Helm umbrella chart with three sub-charts:

| Component | What it does | Default |
|---|---|---|
| **benchmarks** | A **test Capp** plus a **scale benchmark** that drives load, measures Knative autoscaling SLOs, and pushes them to VictoriaMetrics. (Legacy k6/iter8 load tests are bundled but **off** by default.) | enabled |
| **observability** | ServiceMonitors + Grafana dashboard/alert ConfigMaps (kube-prometheus-stack sidecar convention). | optional |
| **status-page** | Go server that polls Deployment readiness and serves a status page + `capp_component_up` metric. | optional |

> For the VictoriaMetrics SLO use case you typically enable **only** the benchmarks sub-chart. See [`examples/values-airgap.yaml`](examples/values-airgap.yaml) and the step-by-step [air-gapped runbook](docs/RUNBOOK-victoriametrics.md).

---

## Autoscaling SLO benchmarking

### What it measures

| SLO | Metric produced | Default warn / crit |
|---|---|---|
| Cold-start latency (0→N) | `capp_cold_start_seconds` | 3s / 8s |
| Scale-delay (demand → scale-out past 1 pod) | `capp_scale_delay_seconds` | 10s / 30s |
| Time-to-scale (load → `targetPods` Ready) | `capp_time_to_scale_seconds` | 30s / 60s |
| Scale-test outcome | `capp_scale_test_success` (1/0) | — |
| Scale-lag (desired − actual pods) | derived from `autoscaler_*` | 30 / 60 pods |

### How it works — two metric pipes into VictoriaMetrics

```
                          ┌─ scale-test job ── push ──▶ VM /api/v1/import/prometheus   (capp_* synthetic)
test Capp (go-httpbin) ───┤
                          └─ scraped by OpenShift Prometheus ── remote-write ──▶ VM    (autoscaler_*, activator_*)
                                                                                         │
                                                                              Grafana (VM datasource) ── SRE dashboard
```

1. **Pushed (synthetic).** Knative can't expose event-durations like "time to scale" as a gauge, so the **scale-test harness** measures them and POSTs them straight to VM's Prometheus import endpoint. These live **only in VM** (they bypass OpenShift Prometheus).
2. **Pulled (Knative).** `autoscaler_*` / `activator_*` are scraped by OpenShift monitoring and reach VM via a Prometheus **remote-write** (you configure this once — see the runbook).

### The test Capp (system-under-test)

A `Capp` (`rcs.dana.io/v1alpha1`) running a small **delay-capable HTTP image** (`mccutchen/go-httpbin`), created with **no pods** so the first request is a genuine cold start:

- `spec.scaleSpec.metric: concurrency`, `spec.scaleSpec.minReplicas: 0`
- `autoscaling.knative.dev/initial-scale: "0"` on the revision template

> Creating at zero requires Knative `allow-zero-initial-scale: "true"` in `config-autoscaler` (set it via the `KnativeServing` CR: `spec.config.config-autoscaler`). If your cluster doesn't allow it, set `benchmarks.testCapp.initialScale: "1"` — the harness then waits for idle scale-to-zero before the cold-start check.

### The scale-test harness

`benchmarks/scale/scale-test.sh` (runs in the `capp-benchmark-runner` image, which bundles k6/kubectl/jq/curl):

1. Confirm the Capp is at **zero pods** (skips cold-start, never fakes it, if it isn't).
2. Fire one request → measure **cold-start TTFB**.
3. Drive concurrency with **k6** against `/delay/0.5` (no think-time, so in-flight concurrency ≈ VUs).
4. Watch `autoscaler_actual_pods` → record **scale-delay** (`>1` pod) and **time-to-scale** (`targetPods` reached).
5. POST the results to VM. The service scales back to zero on its own — **no ksvc annotation** (the operator owns it).

`targetUrl` / `knativeService` are auto-derived from the test Capp, so you don't set them.

**Run it:**

```bash
# hourly via the CronJob (default), or on demand:
kubectl create job capp-scale-now \
  --from=cronjob/capp-monitoring-benchmark-scale -n <namespace>
```

There's also a `benchmarks.scale.oneshot` flag that fires a self-cleaning Job on install/upgrade — handy for `helm`, but **leave it `false` under ArgoCD** (its name is non-deterministic and causes perpetual drift).

---

## Metrics reference

**Pushed to VM** (labels `{capp, namespace}`): `capp_cold_start_seconds`, `capp_scale_delay_seconds`, `capp_time_to_scale_seconds`, `capp_scale_test_success`.

**Pulled into VM via remote-write** (Knative, label `configuration_name` / `revision_name`): `autoscaler_desired_pods`, `autoscaler_actual_pods`, `activator_request_latencies_bucket`, and (if your Knative build exports them) `autoscaler_target_concurrency`, `autoscaler_panic_mode`, `activator_request_count`.

> Forward only what you need: the remote-write `writeRelabelConfigs` keeps `autoscaler_.*|activator_.*`. Widen that regex to add more.

---

## SRE dashboard

[`grafana/sre-dashboard.json`](grafana/sre-dashboard.json) — import into Grafana and set the `ds` variable to your VictoriaMetrics (Prometheus-type) datasource. Sections: SLO summary tiles, autoscaling behaviour (desired vs actual, scale lag, concurrency, panic mode), cold-start & scale timing, and traffic & latency. It auto-scopes to scale-tested capps (the `capp` variable is sourced from `capp_scale_test_success`).

---

## Deployment

### Helm (air-gapped OpenShift)

Mirror the images to your internal registry, then install only the benchmark suite:

```bash
helm install capp-monitoring charts/capp-monitoring \
  --namespace capp-monitoring --create-namespace \
  -f examples/values-airgap.yaml
```

Fill in `<REGISTRY>` and `<VM_INSERT>` (the VM cluster ingest URL, `…/insert/0:0/prometheus`) in the values file. Full steps — enabling UWM, configuring remote-write, adding the Grafana datasource, verifying — are in [`docs/RUNBOOK-victoriametrics.md`](docs/RUNBOOK-victoriametrics.md).

### ArgoCD

Deploy via an `Application` pointing at `charts/capp-monitoring` with the air-gap values inline. Caveats: keep `scale.oneshot=false`, ensure the `Capp` CRD/operator syncs first, and inline the values (the example file is outside the chart path). See the chat history / ask for the ready-made `Application` manifest.

### Images (mirror these)

- `ghcr.io/dana-team/capp-benchmark-runner:0.2.0` — k6, vegeta, iter8, hey, kubectl, jq, curl
- `mccutchen/go-httpbin:v2.15.0` — the test workload

---

## Legacy benchmarks (disabled by default)

The chart still bundles the original load tests, but they're **off** because they generate sustained traffic that keeps the test Capp warm and breaks the scale test's cold-start measurement:

| Job | Value to enable | Notes |
|---|---|---|
| k6 latency + `cold-start.sh` | `benchmarks.k6.enabled=true` | hits `/healthz` (404 on go-httpbin); `cold-start.sh` uses an illegal ksvc annotation |
| iter8 SLO assertion | `benchmarks.iter8.enabled=true` | pass/fail only, emits no metrics |

Only enable them if you point them at a **different** target than the scale test's Capp.

---

## Status server (optional)

Go server on port `8080`. `GET /` (status page), `GET /api/status` (JSON), `GET /metrics` (`capp_component_up{component,group}` — `1` operational, `0` degraded/down). Polls Deployment readiness for CAPP/Knative/infra components every 30s. Namespaces are overridable via `NS_*` env vars — **check these match your install** (the defaults don't match the prereq helmfile).

## Observability (optional)

ServiceMonitors for the backend/operator plus Grafana dashboard and SLO alert rules delivered as kube-prometheus-stack **sidecar** ConfigMaps (`grafana/alerts.yaml`, `grafana/dashboard.json`). Not needed for the VictoriaMetrics SLO flow (OpenShift has no Grafana sidecar).

---

## Development

```bash
make build        # compile the status-server binary
make test         # go test -v -race ./...
make lint         # golangci-lint run ./...
make helm-lint    # helm lint charts/capp-monitoring

helm template t charts/capp-monitoring -n capp-bench   # render & inspect
jq -e . grafana/sre-dashboard.json                     # validate the dashboard
```
