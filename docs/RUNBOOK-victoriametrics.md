# Runbook — CAPP Knative autoscaling SLOs → VictoriaMetrics (air-gapped OpenShift)

Goal: deploy a **test Capp**, run **scale benchmarks** against it, ship the metrics to
**VictoriaMetrics**, and view the **SRE dashboard** in Grafana.

Two metric flows, by design:
- **Scraped** (`autoscaler_*`, `activator_*`) → collected by OpenShift User Workload
  Monitoring (UWM) → **remote-written** to VM.
- **Synthetic** (`capp_cold_start_seconds`, `capp_scale_delay_seconds`,
  `capp_time_to_scale_seconds`, `capp_scale_test_success`) → **pushed directly** to VM by
  the scale-test job. These live **only in VM**, not in the OpenShift console.

Placeholders to fill in:
- `<REGISTRY>` — internal registry (e.g. `registry.internal/dana-team`)
- `<VM_INSERT>` — VM cluster *ingest* URL from the monitoring team
  (e.g. `http://<vminsert>:8480/insert/0:0/prometheus`). Remote-write appends
  `/api/v1/write`; the import push appends `/api/v1/import/prometheus`.
- `<VM_SELECT>` — VM *query* URL for the Grafana datasource
  (e.g. `http://<vmselect>:8481/select/0:0/prometheus`) — a different endpoint, ask the team.
- `<NS>` — namespace for the suite (e.g. `capp-monitoring`)

---

## 0. Prerequisites

- OpenShift with Knative (OpenShift Serverless) installed; `autoscaler_*` / `activator_*`
  already visible in **Observe → Metrics**.
- `container-app-operator` (CAPP) installed — the `Capp` CRD (`rcs.dana.io/v1alpha1`) exists.
- For a genuine 0→N cold start, Knative needs `allow-zero-initial-scale: "true"` in the
  `config-autoscaler` ConfigMap: `oc -n knative-serving get cm config-autoscaler -o yaml`.
  If it's `false`, set `benchmarks.testCapp.initialScale: "1"` (the harness then waits for
  idle scale-to-zero before the cold-start check).
- A reachable VictoriaMetrics instance (vmsingle or cluster).
- Grafana Enterprise reachable, with permission to add a datasource + import a dashboard.
- `oc`, `helm` available against the cluster.

---

## 1. Mirror images into the internal registry

Air-gapped clusters can't reach `ghcr.io` / Docker Hub. Mirror these from a connected host:

```bash
# benchmark runner (contains k6, hey, vegeta, iter8, kubectl, jq, curl)
skopeo copy docker://ghcr.io/dana-team/capp-benchmark-runner:0.2.0 \
            docker://<REGISTRY>/capp-benchmark-runner:0.2.0
# test workload image — go-httpbin has a /delay endpoint to drive concurrency
skopeo copy docker://docker.io/mccutchen/go-httpbin:v2.15.0 \
            docker://<REGISTRY>/go-httpbin:v2.15.0
```
(Equivalent with `oc image mirror` or `docker save | docker load` if skopeo is unavailable.)

> The status-page image is **not** needed for this goal (deprioritized).

---

## 2. Enable User Workload Monitoring

```bash
oc -n openshift-monitoring patch configmap cluster-monitoring-config \
  --type merge -p '{"data":{"config.yaml":"enableUserWorkload: true\n"}}' \
  || oc -n openshift-monitoring create configmap cluster-monitoring-config \
       --from-literal=config.yaml=$'enableUserWorkload: true\n'
```
Verify the UWM stack comes up:
```bash
oc -n openshift-user-workload-monitoring get pods
```

---

## 3. Remote-write the Knative metrics to VictoriaMetrics

Configure UWM Prometheus to stream metrics to VM. The `writeRelabelConfigs` below keeps
VM lean by sending **only** the series this dashboard needs — drop it to send everything.

```yaml
# uwm-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: user-workload-monitoring-config
  namespace: openshift-user-workload-monitoring
data:
  config.yaml: |
    prometheus:
      remoteWrite:
        - url: "<VM_INSERT>/api/v1/write"
          writeRelabelConfigs:
            - sourceLabels: [__name__]
              regex: "autoscaler_.*|activator_.*"
              action: keep
```
```bash
oc apply -f uwm-config.yaml
```
> Single-node VM uses `http://<vmsingle>:8429/api/v1/write` instead.

---

## 4. Add the VictoriaMetrics datasource in Grafana

VM is Prometheus-API compatible — add it as a **Prometheus** datasource.

- **UI:** Connections → Data sources → Add → Prometheus → URL = `<VM_SELECT>`
  (the query endpoint — **not** the insert URL) → Save & test.
- Note the datasource **name/UID**; you'll pick it for the `${ds}` variable on import.

Provisioning equivalent (if managed by file):
```yaml
apiVersion: 1
datasources:
  - name: VictoriaMetrics
    type: prometheus
    access: proxy
    url: <VM_SELECT>
    isDefault: false
```

---

## 5. Install the suite (test Capp + scale benchmark)

Edit `examples/values-airgap.yaml` and fill in the two placeholders — `<REGISTRY>`
(your mirror) and `<VM_INSERT>` (the team's ingest URL). It disables the status-page
and observability sub-charts and configures only the benchmark suite. If the cluster
has `allow-zero-initial-scale=false`, also set `benchmarks.testCapp.initialScale: "1"`
(see Prerequisites). Then:

```bash
helm install capp-monitoring charts/capp-monitoring \
  --namespace <NS> --create-namespace \
  -f examples/values-airgap.yaml
```

This creates: the test Capp (`capp-slo-test`), the benchmark ServiceAccount/RBAC, the
hourly scale CronJob, and the script ConfigMap. `targetUrl`/`knativeService` are
**auto-derived** from the test Capp — nothing else to set.

Confirm the Capp scaled up its Knative service:
```bash
oc -n <NS> get capp,ksvc,pods
```

---

## 6. Run a benchmark now (don't wait for the schedule)

Either toggle the one-shot flag:
```bash
helm upgrade capp-monitoring charts/capp-monitoring --reuse-values \
  --namespace <NS> --set benchmarks.scale.oneshot=true
# (set it back to false afterwards)
```
…or spawn one straight from the CronJob (no redeploy):
```bash
oc -n <NS> create job capp-scale-now --from=cronjob/capp-monitoring-benchmark-scale
oc -n <NS> logs -f job/capp-scale-now
```
A run takes a few minutes (scale-to-zero wait + load + scale-out). Expect log lines for
cold-start, scale-out, and "Pushing metrics to VictoriaMetrics".

---

## 7. Verify metrics landed in VM (vmui)

In vmui (`<VM_BASE>/vmui`) or Grafana Explore on the VM datasource:
```promql
capp_cold_start_seconds
capp_time_to_scale_seconds
capp_scale_delay_seconds
capp_scale_test_success
autoscaler_actual_pods           # confirms remote-write is flowing
```
If the synthetic ones are missing → check the job logs (was `VM_IMPORT_URL` set / endpoint
reachable). If `autoscaler_*` are missing → check the UWM remote-write config (step 3).

---

## 8. Import the SRE dashboard

Grafana → Dashboards → Import → upload `grafana/sre-dashboard.json` → when prompted, set
the **`ds`** variable to the VictoriaMetrics datasource. Pick the `$capp` filter (defaults
to All).

> **Unit check:** the "activator p99" series may be in **milliseconds** (Knative default).
> If it reads ~1000× the synthetic TTFB, change its query to divide by 1000. The synthetic
> `capp_cold_start_seconds` is authoritative (seconds).

---

## 9. Troubleshooting

| Symptom | Likely cause |
|---|---|
| Synthetic metrics absent in VM | `victoriametrics.importUrl` empty, or VM import endpoint unreachable from the job |
| `autoscaler_*` absent in VM | UWM not enabled (step 2) or remote-write misconfigured (step 3) |
| Capp never scales past 1 | load too low vs `testCapp.target`; raise `scale.loadVUs` or lower `target` |
| `time_to_scale` never recorded (success=0) | `scale.targetPods` higher than load can reach within `scale.timeoutSeconds` |
| Job RBAC error listing pods | confirm the benchmark ClusterRole/Binding applied (`get,list pods`) |
| Dashboard panels empty | `${ds}` not pointed at the VM datasource, or no run yet |

---

## Notes

- **Synthetic metrics are VM-only** and will **not** appear in OpenShift Observe → Metrics
  (UWM has no push endpoint). The Knative `autoscaler_*`/`activator_*` remain visible in
  both planes.
- SLO **alerting** (vs. just the dashboard) is a follow-up: add `vmalert`/`VMRule` in the
  VM stack using the same thresholds (cold-start 3/8s, time-to-scale 30/60s, scale-delay
  10/30s, scale-lag 30/60 pods).
