#!/usr/bin/env bash
# cold-start.sh — scales a Knative service to zero, then fires a request
# and measures TTFB. Pushes result to Prometheus pushgateway if configured.
set -euo pipefail

KSVC_NAME="${KSVC_NAME:-}"
KSVC_NAMESPACE="${KSVC_NAMESPACE:-capp-system}"
TARGET_URL="${TARGET_URL:-}"
PUSHGATEWAY_URL="${PUSHGATEWAY_URL:-}"

if [[ -z "$KSVC_NAME" || -z "$TARGET_URL" ]]; then
  echo "ERROR: KSVC_NAME and TARGET_URL must be set" >&2
  exit 1
fi

echo "==> Scaling $KSVC_NAME to zero..."
kubectl annotate ksvc "$KSVC_NAME" \
  -n "$KSVC_NAMESPACE" \
  "autoscaling.knative.dev/initial-scale=0" \
  "autoscaling.knative.dev/min-scale=0" \
  --overwrite

echo "==> Waiting 30s for pods to terminate..."
sleep 30

echo "==> Firing cold-start request to $TARGET_URL..."
TTFB=$(curl -o /dev/null -s -w "%{time_starttransfer}" "$TARGET_URL")
echo "TTFB: ${TTFB}s"

if [[ -n "$PUSHGATEWAY_URL" ]]; then
  cat <<EOF | curl --data-binary @- "${PUSHGATEWAY_URL}/metrics/job/capp_cold_start"
# HELP capp_cold_start_ttfb_seconds Time to first byte for cold-start request
# TYPE capp_cold_start_ttfb_seconds gauge
capp_cold_start_ttfb_seconds $TTFB
EOF
  echo "==> Pushed TTFB metric to pushgateway"
fi

echo "==> Restoring min-scale to 1..."
kubectl annotate ksvc "$KSVC_NAME" \
  -n "$KSVC_NAMESPACE" \
  "autoscaling.knative.dev/min-scale=1" \
  --overwrite

echo "Done."
