#!/usr/bin/env bash

set -euo pipefail
LINKERD_CMD=${LINKERD_CMD:-linkerd}

# Exit early if HAZL is already enabled
if kubectl get ns linkerd &>/dev/null; then
  if kubectl get configmap linkerd-config -n linkerd \
     -o yaml | grep -q 'ext-endpoint-zone-weights'; then
    echo "✅ HAZL already enabled in control plane—skipping upgrade."
    exit 0
  fi
fi

echo "🔄 Enabling HAZL via linkerd upgrade…"
${LINKERD_CMD} upgrade --set "destinationController.additionalArgs[0]=-ext-endpoint-zone-weights" | kubectl apply -f -