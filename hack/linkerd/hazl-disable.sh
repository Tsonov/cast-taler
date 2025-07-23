#!/usr/bin/env bash

set -euo pipefail
LINKERD_CMD=${LINKERD_CMD:-linkerd}

${LINKERD_CMD} upgrade | kubectl apply -f -
