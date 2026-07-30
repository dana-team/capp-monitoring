#!/usr/bin/env bash
# scale-test.sh – measures Knative autoscaling SLOs for a test Capp and pushes
# the results to VictoriaMetrics' Prometheus import endpoint.
#
# Metrics produced (event durations Prometheus can't expose directly):
#   capp_cold_start_seconds    – TTFB of the first request after a genuine scale-to-zero
#   capp_scale_delay_seconds   – load start -> scale-out past the first pod (>1 ready)
#   capp_time_to_scale_seconds – load start -> TARGET_PODS pods Ready
#   capp_scale_test_success    – 1 if TARGET_PODS reached within the timeout, else 0
#
# Tools used (all in capp-benchmark-runner): bash, kubectl, curl, k6, jq.
set -uo pipefail

KSVC_NAME="${KSVC_NAME:?KSVC_NAME must be set}"
KSVC_NAMESPACE="${KSVC_NAMESPACE:-default}"
TARGET_URL="${TARGET_URL:?TARGET_URL must be set}"
TARGET_PODS="${TARGET_PODS:-5}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-180}"
ZERO_WAIT_SECONDS="${ZERO_WAIT_SECONDS:-180}"
LOAD_VUS="${LOAD_VUS:-100}"
LOAD_DURATION="${LOAD_DURATION:-3m}"
LOAD_PATH="${LOAD_PATH:-/}"
VM_IMPORT_URLS="${VM_IMPORT_URLS:-}"

# ready_pods: count of Ready pods backing the Knative service.
ready_pods() {
  kubectl get pods -n "$KSVC_NAMESPACE" \
    -l serving.knative.dev/service="$KSVC_NAME" \
    -o jsonpath='{range .items[*]}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}' 2>/dev/null \
    | grep -c "True" || true
}

# Scale-to-zero is owned by the Capp (spec.scaleSpec.minReplicas=0 + initial-scale=0).
# Do NOT annotate the ksvc directly: the operator owns it and Knative rejects
# autoscaling annotations on the Service's top-level metadata. We just confirm the
# service is at zero; if it isn't, the cold-start metric is skipped (not faked).
echo "==> Confirming $KSVC_NAME is at zero pods (wait up to ${ZERO_WAIT_SECONDS}s)..."
zero_deadline=$(( $(date +%s) + ZERO_WAIT_SECONDS ))
while [ "$(ready_pods)" -ne 0 ] && [ "$(date +%s)" -lt "$zero_deadline" ]; do
  sleep 5
done
PODS_AT_START=$(ready_pods)
echo "    ready pods now: $PODS_AT_START"

COLD=""
if [ "$PODS_AT_START" -eq 0 ]; then
  echo "==> Measuring cold-start TTFB (scale from zero)...."
  COLD=$(curl -o /dev/null -s -w '%{time_starttransfer}' --max-time "$TIMEOUT_SECONDS" "$TARGET_URL" || echo "")
  echo "    cold_start_seconds: ${COLD:-<failed>}"
else
  echo "    WARN: not at zero pods – skipping cold-start (would measure a warm pod)"
fi

echo "==> Starting sustained load (vus=$LOAD_VUS, duration=$LOAD_DURATION, path=$LOAD_PATH)..."
LOAD_START=$(date +%s.%N)
k6 run --quiet \
  -e TARGET_URL="$TARGET_URL" \
  -e LOAD_PATH="$LOAD_PATH" \
  -e LOAD_VUS="$LOAD_VUS" \
  -e LOAD_DURATION="$LOAD_DURATION" \
  /scripts/load.js >/dev/null 2>&1 &
K6_PID=$!

echo "==> Watching autoscaler scale-out (target=$TARGET_PODS pods, timeout=${TIMEOUT_SECONDS}s)..."
SCALE_DELAY=""
TIME_TO_SCALE=""
SUCCESS=0
deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
while [ "$(date +%s)" -lt "$deadline" ]; do
  n=$(ready_pods)
  now=$(date +%s.%N)
  if [ -z "$SCALE_DELAY" ] && [ "$n" -gt 1 ]; then
    SCALE_DELAY=$(jq -n "$now - $LOAD_START")
    echo "    scale-out began: ${SCALE_DELAY}s ($n pods)"
  fi
  if [ "$n" -ge "$TARGET_PODS" ]; then
    TIME_TO_SCALE=$(jq -n "$now - $LOAD_START")
    SUCCESS=1
    echo "    reached $TARGET_PODS pods in ${TIME_TO_SCALE}s"
    break
  fi
  sleep 2
done

kill "$K6_PID" >/dev/null 2>&1 || true
wait "$K6_PID" 2>/dev/null || true
# Load stops here; the service scales back to zero on its own (no annotation needed).

echo "==> Results: cold_start=${COLD:-NaN} scale_delay=${SCALE_DELAY:-NaN} time_to_scale=${TIME_TO_SCALE:-NaN} success=$SUCCESS"
IFS=',' read -ra VM_URLS <<< "${VM_IMPORT_URLS:-}"
if [ ${#VM_URLS[@]} -eq 0 ]; then
  echo "VM_IMPORT_URLS not set – skipping metric push."
  exit 0
fi

METRICS=$(mktemp)
(
  [ -n "$COLD" ]            && echo "capp_cold_start_seconds $COLD"
  [ -n "$SCALE_DELAY" ]     && echo "capp_scale_delay_seconds $SCALE_DELAY"
  [ -n "$TIME_TO_SCALE" ]   && echo "capp_time_to_scale_seconds $TIME_TO_SCALE"
  echo "capp_scale_test_success $SUCCESS"
) > "$METRICS"

for url in "${VM_URLS[@]}"; do
  echo "==> Pushing metrics to VictoriaMetrics ($url)..."
  curl -s --data-binary @"$METRICS" \
    "${url}/api/v1/import/prometheus?extra_label=capp=${KSVC_NAME}&extra_label=namespace=${KSVC_NAMESPACE}"
  echo "    done."
done
rm -f "$METRICS"
