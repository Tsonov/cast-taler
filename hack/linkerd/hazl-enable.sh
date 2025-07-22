#!/usr/bin/env bash

set -euo pipefail

linkerd upgrade --set "destinationController.additionalArgs[0]=-ext-endpoint-zone-weights" | kubectl apply -f -
