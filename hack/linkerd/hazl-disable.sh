#!/usr/bin/env bash

set -euo pipefail

linkerd upgrade | kubectl apply -f -
